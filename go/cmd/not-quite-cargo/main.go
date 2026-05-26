package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	flags "github.com/jessevdk/go-flags"
	ver "github.com/lczyk/version/go"

	"github.com/lczyk/not-quite-cargo/go/cargo"
	"github.com/lczyk/not-quite-cargo/go/cargo/unitgraph"
	"github.com/lczyk/not-quite-cargo/go/driver"
	vinfo "github.com/lczyk/not-quite-cargo/go/internal/version"
)

type Options struct {
	Version func() `short:"v" long:"version" description:"Show version and exit"`

	Patch PatchCommand `command:"patch" description:"Rewrite paths in a Cargo build plan into placeholders"`
	Run   RunCommand   `command:"run"   description:"Execute a (patched) Cargo build plan"`
	Build BuildCommand `command:"build" description:"[EXPERIMENTAL] Build a runnable plan from a cargo --unit-graph"`
	Drive DriveCommand `command:"drive" description:"Act as a cc-driver shim around wild: translate gcc-style link args into raw ld-style and forward to wild"`
}

type planArg struct {
	BuildPlan string `positional-arg-name:"build-plan.json" description:"Path to the build plan JSON file"`
}

// PatchCommand rewrites paths in a build plan into placeholders. Pure
// transform on the plan JSON -- reads only the build-plan file (no env,
// no host filesystem inspection). PROJECT_ROOT / CARGO_HOME come in as
// required flags. Writes the patched plan to stdout by default, or back
// over the input file with --inplace.
type PatchCommand struct {
	ProjectRoot    string  `long:"project-root" required:"yes" description:"Concrete path to replace with {{PROJECT_ROOT}} in the plan"`
	CargoHome      string  `long:"cargo-home" required:"yes" description:"Concrete path to replace with {{CARGO_HOME}} in the plan"`
	InPlace        bool    `long:"inplace" description:"Write the patched plan back over the input file (atomic) instead of stdout"`
	Profile        string  `long:"profile" description:"Rewrite plan for target profile: 'release' or 'debug'"`
	Linker         string  `long:"linker" description:"Bake '-C linker=<path>' into every rustc invocation in the patched plan. Same flag exists on 'run' for ad-hoc overrides (last value wins)."`
	CodegenBackend string  `long:"codegen-backend" description:"Bake '-Z codegen-backend=<value>' into every rustc invocation. Value is a built-in backend name (e.g. 'cranelift') or an absolute path to a backend .so."`
	Panic          string  `long:"panic" description:"Bake '-C panic=<value>' into every rustc invocation. Use 'abort' to override the planner-side default of 'unwind' (cranelift-only rustc needs this)."`
	Args           planArg `positional-args:"yes" required:"yes"`
}

type RunCommand struct {
	Linker string  `long:"linker" description:"Path to a linker binary to inject as '-C linker=<path>' on every rustc invocation. Useful in environments where rustc's default linker driver (cc) is absent."`
	Args   planArg `positional-args:"yes" required:"yes"`
}

// DriveCommand acts as a cc-driver shim around an ld-style linker.
// rustc invokes it as the linker (typically via /usr/bin/cc symlinked
// to the not-quite-cargo binary, or via -C linker=not-quite-cargo
// with the `drive` subcommand prepended). The command translates the
// gcc-style link args it received into raw ld-style and exec's the
// configured linker (wild, mold, lld, plain ld -- anything ld-flavoured).
//
// Every knob has both a CLI flag (here) and a matching NQC_DRIVER_*
// env var (in driver.Config.fillDefaults). The argv[0]==cc shim mode
// is invoked by rustc and owns no command line of its own -- env
// vars are the only configuration channel there. Flag > env > default.
type DriveCommand struct {
	Linker    string `long:"linker"      description:"Path to the ld-style linker to forward translated args to (default /usr/bin/ld, env NQC_DRIVER_LINKER)"`
	Triple    string `long:"triple"      description:"Multiarch triple, e.g. aarch64-linux-gnu (default: auto-detect from runtime, env NQC_DRIVER_TRIPLE)"`
	Interp    string `long:"interp"      description:"Path baked into PT_INTERP, e.g. /usr/lib/aarch64-linux-gnu/ld-linux-aarch64.so.1 (default: auto-detect from triple, env NQC_DRIVER_INTERP)"`
	LibDir    string `long:"lib-dir"     description:"libc / crt directory used for -L and crt object paths (default /usr/lib/<triple>, env NQC_DRIVER_LIB_DIR)"`
	GccLibDir string `long:"gcc-lib-dir" description:"gcc runtime directory used for crtbegin/end + libgcc_s (default /usr/lib/gcc/<triple>/14, env NQC_DRIVER_GCC_LIB_DIR)"`
	Args      struct {
		LinkerArgs []string `positional-arg-name:"linker-args" description:"Linker arguments as rustc / cc-driver would invoke"`
	} `positional-args:"yes"`
}

func (c *DriveCommand) Execute(_ []string) error {
	return driver.Drive(c.Args.LinkerArgs, &driver.Config{
		LinkerPath: c.Linker,
		Triple:     c.Triple,
		Interp:     c.Interp,
		LibDir:     c.LibDir,
		GccLibDir:  c.GccLibDir,
	})
}

func (c *PatchCommand) Execute(_ []string) error {
	plan, err := cargo.LoadPlanJSON(c.Args.BuildPlan)
	if err != nil {
		return err
	}
	patched, err := cargo.PatchPlan(plan, c.ProjectRoot, c.CargoHome, cargo.PatchOptions{
		Linker:         c.Linker,
		CodegenBackend: c.CodegenBackend,
		Panic:          c.Panic,
	})
	if err != nil {
		return err
	}
	if c.Profile != "" {
		spec, ok := cargo.ParseProfile(c.Profile)
		if !ok {
			return fmt.Errorf("--profile must be 'release' or 'debug', got %q", c.Profile)
		}
		cargo.RewriteProfile(patched, spec)
	}
	body, err := json.MarshalIndent(patched, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if c.InPlace {
		if err := cargo.WriteAtomic(c.Args.BuildPlan, append(body, '\n'), 0o644); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		return nil
	}
	if _, err := os.Stdout.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (c *RunCommand) Execute(_ []string) error {
	cfg, err := cargo.NewConfig(nil)
	if err != nil {
		return err
	}
	cfg.Linker = c.Linker
	logConfig(cfg)
	return cargo.Run(c.Args.BuildPlan, cfg)
}

// BuildCommand turns a cargo `--unit-graph` JSON file into a build-plan-
// shaped JSON file that the existing patch + run pipeline can consume.
//
// EXPERIMENTAL. cargo removed --build-plan in 1.93.0; this command is the
// in-tree way to keep generating runnable plans without the now-gone
// upstream feature. correctness is best-effort and the on-disk format
// may change between nqc releases. see unit-graph-plan.md at the repo
// root for the design notes and known limitations.
type BuildCommand struct {
	OS        string `long:"os" required:"yes" description:"Target OS: linux or macos"`
	Arch      string `long:"arch" required:"yes" description:"Target arch: aarch64 or x86_64"`
	Libc      string `long:"libc" default:"gnu" description:"Target libc: gnu / musl (linux), or 'none' (macos)"`
	Vendor    string `long:"vendor" description:"Override the target-triple vendor token (e.g. alpine for aarch64-alpine-linux-musl). Default: unknown (linux) / apple (macos)"`
	RustcPath string `long:"rustc" default:"rustc" description:"rustc program name to embed in the plan"`

	Args struct {
		UnitGraph string `positional-arg-name:"unit-graph.json" description:"Input unit-graph JSON"`
	} `positional-args:"yes" required:"yes"`
}

func (c *BuildCommand) Execute(_ []string) error {
	ug, err := unitgraph.LoadUnitGraph(c.Args.UnitGraph)
	if err != nil {
		return err
	}

	// Project root and cargo home are auto-derived from the unit-graph:
	// path+ pkg_ids for the workspace and registry+ source paths for
	// the cargo home. Build handles the derivation when these are empty
	// in opts.
	out, err := unitgraph.Build(ug, unitgraph.BuildOptions{
		Target:    unitgraph.Target{OS: c.OS, Arch: normaliseArch(c.Arch), Libc: c.Libc, VendorOverride: c.Vendor},
		RustcPath: c.RustcPath,
	})
	if err != nil {
		return err
	}

	for _, w := range out.Warnings {
		log.Printf("warning: %s", w)
	}

	plan := map[string]any{
		"invocations": out.Invocations,
		"inputs":      out.Inputs,
	}
	body, err := json.MarshalIndent(plan, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if _, err := os.Stdout.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	log.Printf("built %d invocations from %d units", len(out.Invocations), len(ug.Units))
	return nil
}

// normaliseArch accepts a few common spelling aliases (arm64 / amd64
// from Go / Docker world) and maps them to the rust-canonical names.
// Other inputs pass through and let Target.Validate reject them.
func normaliseArch(a string) string {
	switch a {
	case "arm64":
		return "aarch64"
	case "amd64":
		return "x86_64"
	default:
		return a
	}
}

func logConfig(cfg *cargo.Config) {
	cfg.Logger.Infof("PROJECT_ROOT: %s", cfg.ProjectRoot)
	cfg.Logger.Infof("CARGO_HOME:   %s", cfg.CargoHome)
	cfg.Logger.Infof("RUSTC:        %s", cfg.RustcPath)
	if cfg.Linker != "" {
		cfg.Logger.Infof("LINKER:       %s (injected as -C linker=...)", cfg.Linker)
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	// When invoked as `cc` (typically via a /usr/bin/cc symlink to
	// this binary), shortcut straight to the driver so rustc's
	// default cc-driver linker invocation works without any extra
	// shell shim. All remaining argv goes through driver.Drive.
	if filepath.Base(os.Args[0]) == "cc" {
		if err := driver.Drive(os.Args[1:], &driver.Config{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// driver.Drive exec's wild on success, so we should never reach
		// here -- if we do, treat it as a hard error.
		os.Exit(1)
	}

	opts := Options{
		Version: func() {
			fmt.Printf("not-quite-cargo %s\n",
				ver.FormatVersion(vinfo.Version, vinfo.CommitSHA, vinfo.BuildDate, vinfo.BuildInfo))
			os.Exit(0)
		},
	}
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "not-quite-cargo"

	if _, err := parser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
}
