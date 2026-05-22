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
	OS          string `long:"os" description:"Target OS (linux, macos, windows, freebsd, ...); defaults to host"`
	Arch        string `long:"arch" description:"Target arch (x86_64, aarch64, i686, ...); defaults to host"`
	Env         string `long:"env" description:"Target libc env (gnu, musl, msvc); empty picks a default from --os"`
	ProjectRoot string `long:"project-root" description:"Project root used for output paths (defaults to cwd)"`
	CargoHome   string `long:"cargo-home" description:"CARGO_HOME path spliced into manifest dirs (no file lookups)"`
	RustcPath   string `long:"rustc" description:"rustc program name to embed in the plan (defaults to 'rustc')"`

	Args struct {
		UnitGraph string `positional-arg-name:"unit-graph.json" description:"Input unit-graph JSON"`
	} `positional-args:"yes" required:"yes"`
}

func (c *LowerCommand) Execute(_ []string) error {
	ug, err := unitgraph.LoadUnitGraph(c.Args.UnitGraph)
	if err != nil {
		return err
	}

	tgt := unitgraph.Target{OS: c.OS, Arch: c.Arch, Env: c.Env}
	if tgt.OS == "" {
		tgt.OS = hostOS()
	}
	if tgt.Arch == "" {
		tgt.Arch = hostArch()
	}

	root := c.ProjectRoot
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}

	out, err := unitgraph.Lower(ug, unitgraph.LowerOptions{
		Target:      tgt,
		CargoHome:   c.CargoHome,
		ProjectRoot: root,
		RustcPath:   c.RustcPath,
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
	log.Printf("lowered %d units", len(out.Invocations))
	return nil
}

// hostOS returns the rust target OS string for the current process.
func hostOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
}

// hostArch returns the rust target arch string for the current process.
func hostArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	default:
		return runtime.GOARCH
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
