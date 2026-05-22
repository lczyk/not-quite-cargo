package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

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
	Build BuildCommand `command:"build" description:"[EXPERIMENTAL] Build a runnable plan from a cargo --unit-graph"`
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
	Arch      string `long:"arch" required:"yes" description:"Target arch: aarch64 (verified) or x86_64 (untested)"`
	Libc      string `long:"libc" default:"gnu" description:"Target libc: gnu / musl (linux), or 'none' (macos)"`
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
	// the cargo home. Lower handles the derivation when these are empty
	// in opts.
	out, err := unitgraph.Lower(ug, unitgraph.LowerOptions{
		Target:    unitgraph.Target{OS: c.OS, Arch: normaliseArch(c.Arch), Libc: c.Libc},
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

	if _, err := parser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}
}
