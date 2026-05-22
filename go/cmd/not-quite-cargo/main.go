package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	flags "github.com/jessevdk/go-flags"
	ver "github.com/lczyk/version/go"

	"not-quite-cargo/cargo"
	"not-quite-cargo/cargo/unitgraph"
	vinfo "not-quite-cargo/internal/version"
)

type Options struct {
	Version func() `short:"v" long:"version" description:"Show version and exit"`

	Patch PatchCommand `command:"patch" description:"Rewrite paths in a Cargo build plan into placeholders"`
	Run   RunCommand   `command:"run"   description:"Execute a (patched) Cargo build plan"`
	Lower LowerCommand `command:"lower" description:"[EXPERIMENTAL] Lower a cargo --unit-graph into a build plan"`
}

type planArg struct {
	BuildPlan string `positional-arg-name:"build-plan.json" description:"Path to the build plan JSON file"`
}

type PatchCommand struct {
	Args planArg `positional-args:"yes" required:"yes"`
}

type RunCommand struct {
	Args planArg `positional-args:"yes" required:"yes"`
}

func (c *PatchCommand) Execute(_ []string) error {
	cfg, err := cargo.NewConfig(nil)
	if err != nil {
		return err
	}
	logConfig(cfg)
	return cargo.Patch(c.Args.BuildPlan, cfg)
}

func (c *RunCommand) Execute(_ []string) error {
	cfg, err := cargo.NewConfig(nil)
	if err != nil {
		return err
	}
	logConfig(cfg)
	return cargo.Run(c.Args.BuildPlan, cfg)
}

// LowerCommand turns a cargo `--unit-graph` JSON file into a build-plan-
// shaped JSON file that the existing patch + run pipeline can consume.
//
// EXPERIMENTAL. cargo removed --build-plan in 1.93.0; this command is the
// in-tree way to keep generating runnable plans without the now-gone
// upstream feature. correctness is best-effort and the on-disk format
// may change between nqc releases. see unit-graph-plan.md at the repo
// root for the design notes and known limitations.
type LowerCommand struct {
	Cfg          string `long:"cfg" description:"Path to a 'rustc --print cfg' dump for the host platform" required:"yes"`
	HostTriple   string `long:"host" description:"Host target triple (defaults to runtime detection)"`
	ProjectRoot  string `long:"project-root" description:"Project root used for output paths (defaults to cwd)"`
	CargoHome    string `long:"cargo-home" description:"CARGO_HOME on the planner (defaults to $HOME/.cargo)"`
	RustcPath    string `long:"rustc" description:"rustc program name to embed in the plan (defaults to 'rustc')"`
	SkipManifest bool   `long:"skip-manifest-errors" description:"Fall back to pkg_id-only metadata when a Cargo.toml cannot be loaded"`

	Args struct {
		UnitGraph string `positional-arg-name:"unit-graph.json" description:"Input unit-graph JSON"`
		Output    string `positional-arg-name:"build-plan.json" description:"Output build-plan JSON"`
	} `positional-args:"yes" required:"yes"`
}

func (c *LowerCommand) Execute(_ []string) error {
	ug, err := unitgraph.LoadUnitGraph(c.Args.UnitGraph)
	if err != nil {
		return err
	}

	cfgBytes, err := os.ReadFile(c.Cfg)
	if err != nil {
		return fmt.Errorf("read cfg: %w", err)
	}
	cfg, err := unitgraph.ParseCfg(string(cfgBytes))
	if err != nil {
		return fmt.Errorf("parse cfg: %w", err)
	}

	host := c.HostTriple
	if host == "" {
		host = detectHostTriple()
	}
	root := c.ProjectRoot
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}

	out, err := unitgraph.Lower(ug, unitgraph.LowerOptions{
		HostTriple:         host,
		Cfg:                cfg,
		CargoHome:          c.CargoHome,
		ProjectRoot:        root,
		RustcPath:          c.RustcPath,
		SkipManifestErrors: c.SkipManifest,
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
	if err := os.WriteFile(c.Args.Output, body, 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	log.Printf("lowered %d units to %s", len(out.Invocations), c.Args.Output)
	return nil
}

// detectHostTriple returns the rust-style target triple for the current
// process. Falls back to "<arch>-unknown-<os>" if the OS-specific
// vendor + libc bits aren't determinable.
func detectHostTriple() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}
	switch runtime.GOOS {
	case "darwin":
		return arch + "-apple-darwin"
	case "linux":
		return arch + "-unknown-linux-gnu"
	case "windows":
		return arch + "-pc-windows-msvc"
	default:
		return arch + "-unknown-" + runtime.GOOS
	}
}

func logConfig(cfg *cargo.Config) {
	cfg.Logger.Infof("PROJECT_ROOT: %s", cfg.ProjectRoot)
	cfg.Logger.Infof("CARGO_HOME:   %s", cfg.CargoHome)
	cfg.Logger.Infof("RUSTC:        %s", cfg.RustcPath)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	opts := Options{
		Version: func() {
			fmt.Printf("not-quite-cargo %s\n",
				ver.FormatVersion(vinfo.Version, vinfo.CommitSHA, vinfo.BuildDate, vinfo.BuildInfo))
			os.Exit(0)
		},
	}
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "not-quite-cargo"
	parser.Usage = "[OPTIONS] COMMAND [build-plan.json]"

	if _, err := parser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
}
