package unitgraph

import (
	"fmt"
	"sort"
)

// ExternRef describes one resolved `--extern <name>=<path>` argument.
type ExternRef struct {
	Name string // extern_crate_name from the unit-graph edge
	Path string // path to the .rlib/.so/.dylib the dep produced
}

// ArgsInputs feeds the rustc-command builder.
type ArgsInputs struct {
	Unit    *Unit
	Hash    string
	Out     OutputPaths
	Externs []ExternRef

	// DepsDir is the `target/<profile>/deps` directory dependents need
	// for `-L dependency=`. Same dir as Out.Primary's parent.
	DepsDir string

	// CapLints, when true, adds `--cap-lints warn` to downgrade the
	// unit's lints to warnings -- cargo does this for non-primary
	// packages (registry / git deps) so a dep's `#![deny(...)]`
	// settings don't fail the local build.
	CapLints bool
}

// RustcArgs derives the literal rustc command line for a single unit.
//
// Goal is functional fidelity, not byte-for-byte match with cargo: the
// generated command must compile correctly and produce artefacts at the
// paths the orchestrator already pre-computed. Arg order follows cargo's
// usual layout (flags first, then sources, then `-L`/`--extern`) so the
// output is easy to eyeball-compare against a captured --build-plan.
func RustcArgs(in ArgsInputs) ([]string, error) {
	if in.Unit == nil {
		return nil, fmt.Errorf("RustcArgs: nil unit")
	}
	u := in.Unit
	args := []string{}

	// Crate identity + source.
	args = append(args, "--crate-name", crateNameFor(u))
	if u.Target.Edition != "" {
		args = append(args, fmt.Sprintf("--edition=%s", u.Target.Edition))
	}
	args = append(args, u.Target.SrcPath)

	// Crate type. cargo emits `--crate-type` once per type for cdylib /
	// staticlib combos; do the same.
	for _, ct := range u.Target.CrateTypes {
		args = append(args, "--crate-type", ct)
	}

	// Emit kinds: rustc default is `dep-info,link`; mirror cargo.
	args = append(args, "--emit=dep-info,link")

	// Profile-driven codegen flags.
	args = append(args, profileFlags(u.Profile)...)

	// Enabled features as --cfg flags. Cargo emits one --cfg per feature
	// in the form `feature="<name>"` so source-side `#[cfg(feature =
	// "x")]` evaluates correctly. Sorted for stable output.
	features := append([]string(nil), u.Features...)
	sort.Strings(features)
	for _, f := range features {
		args = append(args, "--cfg", fmt.Sprintf(`feature="%s"`, f))
	}

	// --cap-lints warn so dep crates' #![deny(...)] settings don't fail
	// the local build. Cargo applies this to non-primary packages
	// (registry / git deps); the caller decides via in.CapLints.
	if in.CapLints {
		args = append(args, "--cap-lints", "warn")
	}

	// Metadata + extra-filename get the same hash so dependents can find
	// the artefact.
	args = append(args, "-C", "metadata="+in.Hash)
	args = append(args, "-C", "extra-filename=-"+in.Hash)

	// Output dir.
	if in.DepsDir != "" {
		args = append(args, "--out-dir", in.DepsDir)
	}

	// -L dependency=<deps-dir> so dependent rmeta lookups succeed.
	if in.DepsDir != "" {
		args = append(args, "-L", "dependency="+in.DepsDir)
	}

	// Resolved externs. Deterministic order so two runs over the same
	// unit produce byte-identical commands.
	externs := append([]ExternRef(nil), in.Externs...)
	sort.Slice(externs, func(i, j int) bool { return externs[i].Name < externs[j].Name })
	for _, e := range externs {
		args = append(args, "--extern", e.Name+"="+e.Path)
	}

	// Custom-build units compile build.rs as a host binary; cargo also
	// emits --cfg 'feature="..."' here, but we feed features through
	// the features list further up. Nothing build-script specific to add.

	return args, nil
}

// crateNameFor returns the underscore-canonicalised crate name rustc
// expects. cargo replaces hyphens with underscores when forming
// --crate-name. The unit-graph `target.name` already carries this for
// most cases, but a few packages (build scripts named
// "build-script-build") need the canonical form.
func crateNameFor(u *Unit) string {
	return underscore(u.Target.Name)
}

func underscore(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			b[i] = '_'
			continue
		}
		b[i] = c
	}
	return string(b)
}

// profileFlags translates a unit's profile into the corresponding rustc
// codegen flags. Each option is emitted as a separate (-C, key=value)
// pair so rustc parses them as independent args -- single-string
// "-C key=value" entries would be passed through as one argv element
// and rustc rejects the embedded space.
func profileFlags(p UnitProfile) []string {
	args := []string{}
	if p.OptLevel != "" && p.OptLevel != "0" {
		args = append(args, "-C", "opt-level="+p.OptLevel)
	}
	if p.DebugAssertions {
		args = append(args, "-C", "debug-assertions=on")
	} else {
		args = append(args, "-C", "debug-assertions=off")
	}
	if p.OverflowChecks {
		args = append(args, "-C", "overflow-checks=on")
	} else {
		args = append(args, "-C", "overflow-checks=off")
	}
	// Incremental is intentionally omitted for now: rustc requires a
	// directory path (`-C incremental=<dir>`), which cargo computes
	// per-unit at `<project>/target/<profile>/incremental/<name>-<hash>`.
	// We currently emit a non-incremental build instead -- correct,
	// slower. TODO(lczyk): synthesise the dir + pass it through.
	_ = p.Incremental
	if p.Rpath {
		args = append(args, "-C", "rpath")
	}
	if p.Panic != "" {
		args = append(args, "-C", "panic="+p.Panic)
	}
	// Debug info: number or "line-tables-only"
	switch v := p.Debuginfo.(type) {
	case float64: // JSON numbers decode as float64 into `any`
		if int(v) > 0 {
			args = append(args, "-C", fmt.Sprintf("debuginfo=%d", int(v)))
		}
	case string:
		if v != "" {
			args = append(args, "-C", "debuginfo="+v)
		}
	}
	if p.CodegenUnits != nil {
		args = append(args, "-C", fmt.Sprintf("codegen-units=%d", *p.CodegenUnits))
	}
	if p.LTO != "" && p.LTO != "false" {
		args = append(args, "-C", "lto="+p.LTO)
	}
	if p.SplitDebuginfo != nil && *p.SplitDebuginfo != "" {
		args = append(args, "-C", "split-debuginfo="+*p.SplitDebuginfo)
	}
	if p.CodegenBackend != nil && *p.CodegenBackend != "" {
		args = append(args, "-Z", "codegen-backend="+*p.CodegenBackend)
	}
	return args
}
