#!/usr/bin/env python3
# spell-checker: words RUSTC levelname
# mypy: disable-error-code="unused-ignore"

"""
Script to execute Cargo build plan invocations manually.

First, generate the build plan with:

```bash
cargo build -j1 -Z unstable-options --build-plan > build_plan.json
```

Then perform a minor brain surgery on the `build_plan.json` file to replace
paths with placeholders like `{{PROJECT_ROOT}}`, `{{CARGO_HOME}}`, and `{{RUSTC}}`
to make the build plan more portable.

```bash
python3 not-quite-cargo.py patch build_plan.json
```

Then run this script with:

```bash
python3 not-quite-cargo.py run build_plan.json
```
"""

import argparse
import json
import logging
import os
import pathlib
import subprocess as sub
import sys
from dataclasses import dataclass
from typing import TypeVar, no_type_check

__author__ = "Marcin Konowalczyk"
__version__ = "0.2.1"

__changelog__ = [
    ("0.2.1", "fix finding rustc in some environments", "@lczyk"),
    ("0.2.0", "add patch mode", "@lczyk"),
    ("0.1.0", "inital version", "@lczyk"),
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser("Cargo build plan runner")
    parser.add_argument(
        "--version",
        action="version",
        version="%(prog)s " + __version__,
        help="Show script version and exit.",
    )
    parser.add_argument("mode", type=str, choices=["patch", "run"], help="Mode to run the script in")
    parser.add_argument("build_plan", type=str, help="Path to the build plan file")
    return parser.parse_args()


def setup_logging(log_level: str) -> None:
    _logger = logging.getLogger()
    handler = logging.StreamHandler()
    fmt = "%(asctime)s %(levelname)s %(message)s"
    datefmt = "%Y-%m-%dT%H:%M:%S"
    formatter: type[logging.Formatter] = logging.Formatter
    try:
        # Try to use colorlog for colored output
        import colorlog  # type: ignore

        fmt = fmt.replace("%(levelname)s", "%(log_color)s%(levelname)s%(reset)s")
        formatter = colorlog.ColoredFormatter  # type: ignore
    except ImportError:
        pass

    handler.setFormatter(formatter(fmt, datefmt))  # type: ignore
    _logger.addHandler(handler)
    log_level = "critical" if log_level.lower() == "fatal" else log_level
    _logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))


_T = TypeVar("_T")


def deep_replace(data: _T, replacements: dict[str, str]) -> _T:
    """Recursively replace strings in a nested structure (dicts, lists, strings)."""
    _type = type(data)
    if _type is dict:
        return {  # type: ignore
            deep_replace(key, replacements): deep_replace(value, replacements)  # type: ignore
            for key, value in data.items()  # type: ignore
        }
    elif _type is list:
        return [deep_replace(item, replacements) for item in data]  # type: ignore
    elif _type is str:
        for old, new in replacements.items():
            data = data.replace(old, new)  # type: ignore
        return data  # type: ignore
    else:
        # Some other type we don't know how to handle, return as is
        return data


@dataclass
class Invocation:
    number: int
    package_name: str
    package_version: str
    target_kind: list[str]
    kind: "str | None"
    compile_mode: str
    deps: set[int]
    outputs: list[str]
    links: dict[str, str]
    program: str
    args: list[str]
    env: dict[str, str]
    cwd: str

    @no_type_check
    @classmethod
    def from_json(cls, number: int, data: dict) -> "Invocation":
        return cls(
            number=number,
            package_name=data["package_name"],
            package_version=data["package_version"],
            target_kind=data["target_kind"],
            kind=data.get("kind"),
            compile_mode=data["compile_mode"],
            deps=set(data["deps"]),
            outputs=data["outputs"],
            links=data["links"],
            program=data["program"],
            args=data["args"],
            env=data["env"],
            cwd=data["cwd"],
        )


def resolve_invocation_order(invocations: list[Invocation]) -> list[Invocation]:
    """Resolve invocation order based on dependencies using a simple topological sort."""
    ordered: list[Invocation] = []
    todo = invocations[:]
    satisfied: set[int] = set()
    while todo:
        for inv in todo:
            if inv.deps.issubset(satisfied):
                ordered.append(inv)
                satisfied.add(inv.number)
                todo.remove(inv)
                break
        else:
            print(
                "Could not resolve invocation order due to circular dependencies or missing deps.",
                file=sys.stderr,
            )
            sys.exit(1)
    return ordered


# https://doc.rust-lang.org/cargo/reference/build-scripts.html
class CustomBuildDirectives:
    rustc_flags: list[tuple[str, str]]
    env_vars: dict[str, str]

    def __init__(self, build_script_output: str) -> None:
        lines = build_script_output.splitlines()
        ignored = ["rerun-if-changed", "rerun-if-env-changed"]
        self.rustc_flags = []
        self.env_vars = {}
        for line in lines:
            line = line.strip()
            if not line.startswith("cargo:"):
                # we don't care about non-cargo lines
                continue
            line = line[len("cargo:") :]
            if "=" in line:
                key, value = line.split("=", 1)
            else:
                logging.warning(f"Malformed build script output line (no '='): {line}")
                continue
            if key in ignored:
                continue
            if key == "rustc-cfg":
                self.rustc_flags.append(("--cfg", value))
            elif key == "rustc-check-cfg":
                self.rustc_flags.append(("--check-cfg", value))
            elif key == "rustc-link-lib":
                self.rustc_flags.append(("-l", value))
            elif key == "rustc-link-arg":
                self.rustc_flags.append(("-C", f"link-arg={value}"))
            elif key == "rustc-link-search":
                if "=" in value:
                    kind, path = value.split("=", 1)
                    if kind == "native":
                        self.rustc_flags.append(("-L", path))
                    else:
                        logging.warning(
                            f"Unknown rustc-link-search kind: {kind} in line: {line}. Will try to add as is."
                        )
                        self.rustc_flags.append(("-L", path))
                else:
                    self.rustc_flags.append(("-L", value))
            elif key == "rustc-env":
                k, v = value.split("=", 1)
                self.env_vars[k] = v
            else:
                logging.warning(f"Unknown build script output line: {line}")
                continue

    def apply(self, cmd: list[str], env: dict[str, str]) -> None:
        for flag, value in self.rustc_flags:
            cmd.extend([flag, value])
        env.update(self.env_vars)


def extra_escape(cmd: list[str]) -> list[str]:
    """Extra escape cfg/check-cfg arguments for easy copy-paste from logs to shell."""
    cmd2 = cmd[:]
    for i, arg in enumerate(cmd2):
        if arg in ("--cfg", "--check-cfg") and i + 1 < len(cmd2):
            cmd2[i + 1] = f"'{cmd2[i + 1]}'"
    return cmd2


def find_rustc() -> str:
    _rustc = os.environ.get("RUSTC", None)

    if _rustc is None:
        # Try to get path to rustc from rustup
        result = sub.run(
            ["rustup", "which", "rustc"],
            capture_output=True,
            text=True,
            check=True,
            cwd="/",
        )
        if result.returncode == 0:
            _rustc = result.stdout.strip()
            logging.info(f"Found rustc at {_rustc} using rustup.")

    if _rustc is None:
        # try to find rustc using which
        result = sub.run(
            ["which", "rustc"],
            capture_output=True,
            text=True,
            check=True,
            cwd="/",
        )
        if result.returncode == 0:
            _rustc = result.stdout.strip()
            logging.info(f"Found rustc at {_rustc} using which.")

    if _rustc is None:
        # try to find rustc in PATH
        for path_dir in os.environ.get("PATH", "").split(os.pathsep):
            candidate = os.path.join(path_dir, "rustc")
            if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
                _rustc = candidate
                logging.info(f"Found rustc at {_rustc} by searching PATH.")
                break
    if _rustc is None:
        _rustc = "rustc"  # fallback to just "rustc", hope for the best

    return _rustc


def main(args: argparse.Namespace) -> None:
    PROJECT_ROOT = os.environ.get("PROJECT_ROOT", os.getcwd())
    CARGO_HOME = os.environ.get("CARGO_HOME", os.path.expanduser("~/.cargo"))
    RUSTC = find_rustc()

    logging.info(f"PROJECT_ROOT: {PROJECT_ROOT}")
    logging.info(f"CARGO_HOME: {CARGO_HOME}")
    logging.info(f"RUSTC: {RUSTC}")

    replacements = {
        "{{PROJECT_ROOT}}": PROJECT_ROOT,
        "{{CARGO_HOME}}": CARGO_HOME,
        "{{RUSTC}}": RUSTC,
    }

    if args.mode == "patch":
        patch(args.build_plan, replacements)
    elif args.mode == "run":
        run(args.build_plan, replacements)
    else:
        logging.error(f"Unknown mode: {args.mode}")
        sys.exit(1)


def patch(build_plan_path: str, replacements: dict[str, str]) -> None:
    RUSTC = replacements.pop("{{RUSTC}}")  # don't replace RUSTC in the build plan
    rev_replacements = {v: k for k, v in replacements.items()}

    # TODO: This messes up encoding of some unicode... figure out why
    with open(build_plan_path) as f:
        build_plan = json.load(f)

    # Make sure we look like a build plan file before we patch it
    if "invocations" not in build_plan:
        logging.error(f"{build_plan_path} does not look like a Cargo build plan file.")
        sys.exit(1)

    # Replace all the instances of the paths with placeholders
    rev_replacements.pop("{{RUSTC}}", None)  # don't replace RUSTC in the build plan
    build_plan = deep_replace(build_plan, rev_replacements)

    # Patch each invocation
    invocations = build_plan.get("invocations", [])
    for i, inv in enumerate(invocations):
        env = inv.get("env", {})
        # Remove CARGO env var if present. where we are going there will be no CARGO
        env.pop("CARGO", None)
        # Also remove the vars which will be patched at runtime
        env.pop("PROJECT_ROOT", None)
        env.pop("CARGO_HOME", None)
        env.pop("RUSTC", None)
        inv["env"] = env

        args = inv.get("args", [])
        new_args: list[str] = []
        for arg in args:
            if arg.startswith("--diagnostic-width"):
                # drop diagnostic-width arg
                continue
            new_args.append(arg)
        inv["args"] = new_args

        if inv.get("program") == RUSTC:
            inv["program"] = "{{RUSTC}}"

        invocations[i] = inv
    build_plan["invocations"] = invocations

    with open(build_plan_path, "w") as f:
        json.dump(build_plan, f, indent=4)

    logging.info(f"Patched build plan saved to {build_plan_path}")


def run(build_plan_path: str, replacements: dict[str, str]) -> None:
    # Call -vV on the rustc to ensure it's installed and works
    result = sub.run(
        [replacements["{{RUSTC}}"], "-vV"],
        capture_output=True,
        text=True,
        check=True,
    )
    if result.returncode != 0:
        logging.error(f"Failed getting rustc version from {replacements['{{RUSTC}}']}")
        logging.error(f"stdout:\n{result.stdout}")
        logging.error(f"stderr:\n{result.stderr}")
        sys.exit(result.returncode)

    logging.info(f"{{{{RUSTC}}}} version: {result.stdout.strip().splitlines()[0]}")

    rev_replacements = {v: k for k, v in replacements.items()}

    RUSTC, PROJECT_ROOT, CARGO_HOME = (
        replacements["{{RUSTC}}"],
        replacements["{{PROJECT_ROOT}}"],
        replacements["{{CARGO_HOME}}"],
    )

    with open(build_plan_path) as f:
        build_plan = json.load(f)

    invocations_json = build_plan.get("invocations", [])
    invocations_json = deep_replace(invocations_json, replacements)

    # Sanity check
    invocations_str = json.dumps(invocations_json)
    for old, new in replacements.items():
        assert old not in invocations_str, f"Replacement failed for {old} -> {new}"

    invocations: list[Invocation] = [
        Invocation.from_json(i, item)  # type: ignore
        for i, item in enumerate(invocations_json)
    ]

    # sort invocations according to deps
    invocations = resolve_invocation_order(invocations)

    # create target directories
    target_dirs: set[str] = set()
    for inv in invocations:
        for output in inv.outputs:
            target_dir = os.path.dirname(output)
            target_dirs.add(target_dir)
    for dir in target_dirs:
        pathlib.Path(dir).mkdir(parents=True, exist_ok=True)

    # Keep track of any custom build directives from build scripts
    custom_build_directives: dict[str, CustomBuildDirectives] = {}

    for inv in invocations:
        # command to run
        cmd = [inv.program, *inv.args]

        # patch environment
        env = os.environ.copy()
        env.update(inv.env)
        env["RUSTC"] = RUSTC
        env["PROJECT_ROOT"] = PROJECT_ROOT
        env["CARGO_HOME"] = CARGO_HOME

        # Apply any custom build directives from build scripts of dependencies
        if inv.package_name in custom_build_directives:
            custom_build_directives[inv.package_name].apply(cmd, env)

        # Ensure OUT_DIR exists
        if "OUT_DIR" in env:
            pathlib.Path(env["OUT_DIR"]).mkdir(parents=True, exist_ok=True)

        # A pile of logging to know what's going on
        logging.info(
            "(%d/%d) Running '%s' for package '%s' v%s",
            inv.number,
            len(invocations),
            deep_replace(inv.program, rev_replacements),
            inv.package_name,
            inv.package_version,
        )

        if "custom-build" in inv.target_kind:
            if inv.compile_mode == "build":
                logging.info("This invocation is compiling a custom build script.")
            elif inv.compile_mode == "run-custom-build":
                logging.info("This invocation is running a custom build script.")
            else:
                logging.warning(f"Unknown compile_mode for custom-build: {inv.compile_mode}")

        logging.debug("Invoking: %s", " ".join(extra_escape(cmd)))
        logging.debug("In cwd: %s", inv.cwd)
        logging.debug("With env: %s", inv.env)

        # Actually run the command
        result = sub.run(cmd, check=False, env=env, cwd=inv.cwd, capture_output=True, text=True)

        if result.returncode != 0:
            logging.error(f"stdout:\n{result.stdout}")
            logging.error(f"stderr:\n{result.stderr}")
            logging.error("Command failed with exit code %d", result.returncode)
            sys.exit(result.returncode)

        # Create symlinks for outputs
        for link, target in inv.links.items():
            try:
                if os.path.exists(link):
                    logging.warning(f"Link {link} already exists. Overwriting.")
                    os.remove(link)
                os.symlink(target, link)
                link_str = deep_replace(link, rev_replacements)
                target_str = deep_replace(target, rev_replacements)
                logging.info(f"Created symlink: {link_str} -> {target_str}")
            except Exception as e:
                logging.error(f"Failed to create symlink {link} -> {target}: {e}")
                sys.exit(1)

        # Capture build script outputs
        if inv.compile_mode == "run-custom-build":
            custom_build_directives[inv.package_name] = CustomBuildDirectives(result.stdout)


if __name__ == "__main__":
    args = parse_args()
    setup_logging("INFO")
    try:
        main(args)
    except Exception as e:
        e_str = str(e)
        if not e_str:
            e_str = "An unknown error occurred."
        logging.critical(e_str, exc_info=True)
        sys.exit(1)
