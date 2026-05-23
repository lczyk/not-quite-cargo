use std::path::PathBuf;

use anyhow::Context;
use lexopt::prelude::*;

use not_quite_cargo as nqc;
use version::version;

fn main() -> anyhow::Result<()> {
    let mut parser = lexopt::Parser::from_env();
    let mut subcommand: Option<String> = None;

    // Peek at first positional for subcommand.
    while let Some(arg) = parser.next()? {
        match arg {
            Short('v') | Long("version") => {
                println!("not-quite-cargo {}", version!());
                return Ok(());
            }
            Long("help") => {
                print_help();
                return Ok(());
            }
            Value(val) => {
                subcommand = Some(val.to_string_lossy().into_owned());
                break;
            }
            _ => {
                // Unknown flag before subcommand -- let it slide and handle below.
            }
        }
    }

    match subcommand.as_deref() {
        Some("patch") => cmd_patch(parser),
        Some("run") => cmd_run(parser),
        Some(other) => {
            anyhow::bail!("unknown subcommand: {other}");
        }
        None => {
            print_help();
            Ok(())
        }
    }
}

fn cmd_patch(mut parser: lexopt::Parser) -> anyhow::Result<()> {
    let mut project_root: Option<String> = None;
    let mut cargo_home: Option<String> = None;
    let mut inplace = false;
    let mut build_plan: Option<PathBuf> = None;

    while let Some(arg) = parser.next()? {
        match arg {
            Long("project-root") => {
                project_root = Some(parser.value()?.to_string_lossy().into_owned());
            }
            Long("cargo-home") => {
                cargo_home = Some(parser.value()?.to_string_lossy().into_owned());
            }
            Long("inplace") => {
                inplace = true;
            }
            Long("help") => {
                print_patch_help();
                return Ok(());
            }
            Value(val) => {
                build_plan = Some(PathBuf::from(val.to_string_lossy().into_owned()));
            }
            _ => {
                anyhow::bail!("unexpected argument: {arg:?}");
            }
        }
    }

    let project_root = project_root.context("--project-root is required")?;
    let cargo_home = cargo_home.context("--cargo-home is required")?;
    let build_plan = build_plan.context("build-plan.json is required")?;

    let plan = nqc::load_plan_json(&build_plan)?;
    let patched = nqc::patch_plan(&plan, &project_root, &cargo_home)?;
    let body = nqc::pretty_format(&patched);
    let output = format!("{body}\n");

    if inplace {
        use std::fs::Permissions;
        use std::os::unix::fs::PermissionsExt;
        nqc::write_atomic(
            &build_plan,
            output.as_bytes(),
            Permissions::from_mode(0o644),
        )?;
    } else {
        use std::io::Write;
        std::io::stdout()
            .write_all(output.as_bytes())
            .context("write stdout")?;
    }

    Ok(())
}

fn cmd_run(mut parser: lexopt::Parser) -> anyhow::Result<()> {
    let mut build_plan: Option<PathBuf> = None;

    while let Some(arg) = parser.next()? {
        match arg {
            Long("help") => {
                print_run_help();
                return Ok(());
            }
            Value(val) => {
                build_plan = Some(PathBuf::from(val.to_string_lossy().into_owned()));
            }
            _ => {
                anyhow::bail!("unexpected argument: {arg:?}");
            }
        }
    }

    let build_plan = build_plan.context("build-plan.json is required")?;

    let logger = nqc::Logger::new();
    let cfg = nqc::new_config(logger)?;
    log_config(&cfg);
    nqc::run(&build_plan, &cfg)
}

fn log_config(cfg: &nqc::Config) {
    cfg.logger
        .info(&format!("PROJECT_ROOT: {}", cfg.project_root.display()));
    cfg.logger
        .info(&format!("CARGO_HOME:   {}", cfg.cargo_home.display()));
    cfg.logger
        .info(&format!("RUSTC:        {}", cfg.rustc_path.display()));
}

fn print_help() {
    eprintln!(
        "\
Usage: not-quite-cargo <command> [options]

Commands:
  patch     Rewrite paths in a Cargo build plan into placeholders
  run       Execute a (patched) Cargo build plan

Options:
  --version, -v   Show version and exit
  --help          Show this help

Run 'not-quite-cargo <command> --help' for command-specific options."
    );
}

fn print_patch_help() {
    eprintln!(
        "\
Usage: not-quite-cargo patch --project-root <dir> --cargo-home <dir> [--inplace] <build-plan.json>

Options:
  --project-root <dir>   Concrete path to replace with {{{{PROJECT_ROOT}}}} in the plan
  --cargo-home <dir>     Concrete path to replace with {{{{CARGO_HOME}}}} in the plan
  --inplace              Write the patched plan back over the input file (atomic)
  --help                 Show this help"
    );
}

fn print_run_help() {
    eprintln!(
        "\
Usage: not-quite-cargo run <build-plan.json>

Options:
  --help   Show this help"
    );
}
