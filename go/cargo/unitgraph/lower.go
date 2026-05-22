package unitgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// LowerOptions controls the lowering. Most fields are optional and have
// sensible defaults derived from the unit-graph + environment.
type LowerOptions struct {
	// HostTriple identifies the planner's host platform, used for units
	// with `platform: null` (host builds, proc macros, build scripts).
	HostTriple string

	// CargoHome is the planner's CARGO_HOME, used to locate registry
	// sources on disk. Defaults to `$HOME/.cargo`.
	CargoHome string

	// ProjectRoot is the path under which `target/` will be written on
	// the runner; the lowerer emits paths relative to this so the
	// existing patch step can template them to `{{PROJECT_ROOT}}`.
	ProjectRoot string

	// RustcPath is the program string emitted for rustc invocations.
	// Defaults to "rustc" so the runner's PATH resolution + nqc's
	// {{RUSTC}} templating handle it.
	RustcPath string

	// RustcLinker, if non-empty, is forwarded to build scripts as
	// RUSTC_LINKER and to rustc via `-C linker=`.
	RustcLinker string

	// Cfg is the parsed `rustc --print cfg` output for the host
	// platform. Required so CARGO_CFG_* env vars can be synthesised.
	Cfg CfgMap

	// RegistryIndex names the index cache subdirectory under
	// $CARGO_HOME/registry/src/. If empty the lowerer scans and picks
	// the first one it finds (works for the common single-registry case).
	RegistryIndex string

	// SkipManifestErrors causes the lowerer to fall back to bare
	// CARGO_PKG_NAME/_VERSION (derived from pkg_id) when a manifest
	// can't be loaded, instead of returning the error. Useful for git
	// sources whose checkout dir isn't statically resolvable.
	SkipManifestErrors bool
}

// LowerOutput is the result of lowering: an `Invocation`-shaped JSON
// document equivalent to a cargo --build-plan output, plus warnings
// the caller may want to surface.
type LowerOutput struct {
	Invocations []Invocation
	Inputs      []string
	Warnings    []string
}

// Invocation mirrors cargo's --build-plan invocation shape, which is
// also what nqc's existing cargo.Run consumes. Declared here (rather
// than importing from cargo/) to keep unitgraph free of import cycles;
// the orchestrator marshals to JSON and the cargo pkg unmarshals on the
// other end.
type Invocation struct {
	PackageName    string            `json:"package_name"`
	PackageVersion string            `json:"package_version"`
	TargetKind     []string          `json:"target_kind"`
	Kind           *string           `json:"kind"`
	CompileMode    string            `json:"compile_mode"`
	Deps           []int             `json:"deps"`
	Outputs        []string          `json:"outputs"`
	Links          map[string]string `json:"links"`
	Program        string            `json:"program"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	Cwd            string            `json:"cwd"`
}

// Lower converts a unit-graph into a build-plan-shaped document.
//
// The output's Invocations slice is in unit-graph index order (deps
// reference indices, so order matters). Run's topological sort handles
// execution order on the runner side.
func Lower(ug *UnitGraph, opt LowerOptions) (*LowerOutput, error) {
	if ug == nil {
		return nil, fmt.Errorf("lower: nil unit-graph")
	}
	if opt.HostTriple == "" {
		return nil, fmt.Errorf("lower: HostTriple is required")
	}
	if opt.Cfg == nil {
		return nil, fmt.Errorf("lower: Cfg is required")
	}
	if opt.CargoHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("lower: resolve CARGO_HOME: %w", err)
		}
		opt.CargoHome = filepath.Join(home, ".cargo")
	}
	if opt.RustcPath == "" {
		opt.RustcPath = "rustc"
	}
	if opt.RegistryIndex == "" {
		idx, err := guessRegistryIndex(opt.CargoHome)
		if err == nil {
			opt.RegistryIndex = idx
		}
	}

	out := &LowerOutput{
		Invocations: make([]Invocation, len(ug.Units)),
	}

	// Pre-pass: per unit, parse pkg id, load manifest (best-effort),
	// compute hash + output paths. Later units' externs reference these.
	derived := make([]unitDerived, len(ug.Units))
	for i := range ug.Units {
		d, warn, err := preDerive(&ug.Units[i], opt)
		if err != nil {
			return nil, fmt.Errorf("unit %d (%s): %w", i, ug.Units[i].PkgID, err)
		}
		if warn != "" {
			out.Warnings = append(out.Warnings, warn)
		}
		derived[i] = d
	}

	// Main pass: synthesise the Invocation for each unit.
	for i, u := range ug.Units {
		inv, err := buildInvocation(&u, i, derived, opt, ug)
		if err != nil {
			return nil, fmt.Errorf("unit %d (%s): %w", i, u.PkgID, err)
		}
		out.Invocations[i] = inv
	}

	return out, nil
}

// unitDerived caches the per-unit values the main pass needs about
// every other unit (for extern resolution).
type unitDerived struct {
	pkgID                PkgID
	pkg                  PkgMetadata
	hash                 string
	outputs              OutputPaths
	platform             string // resolved (HostTriple if Unit.IsHost)
	depsDir              string // target/<profile>/[<triple>/]deps
	isCustomBuildCompile bool   // build.rs compile step (vs run-custom-build / regular compile)
	isRunCustomBuild     bool   // running the previously-compiled build.rs
	outDir               string // for run-custom-build units, the OUT_DIR they write into
}

func preDerive(u *Unit, opt LowerOptions) (unitDerived, string, error) {
	id, err := ParsePkgID(u.PkgID)
	if err != nil {
		return unitDerived{}, "", err
	}

	var pkg PkgMetadata
	var warning string
	pkg, err = LoadManifestForPkg(id, opt.CargoHome, opt.RegistryIndex)
	if err != nil {
		if !opt.SkipManifestErrors {
			return unitDerived{}, "", fmt.Errorf("load manifest: %w", err)
		}
		// Fallback: best-effort from pkg_id alone.
		pkg = PkgMetadata{Name: id.Name, Version: id.Version}
		warning = fmt.Sprintf("manifest load failed for %s (falling back to pkg_id-only metadata): %v", id.Name, err)
	}

	platform := u.PlatformOrHost(opt.HostTriple)
	profileDir := profileDir(u.Profile.Name)

	hash := MetadataHash(HashInputs{
		PkgID:       u.PkgID,
		TargetName:  u.Target.Name,
		Mode:        u.Mode,
		ProfileName: u.Profile.Name,
		Features:    u.Features,
		Platform:    platform,
		Host:        opt.HostTriple,
	})

	out := OutputPathsFor(PathInputs{
		ProjectRoot: opt.ProjectRoot,
		ProfileDir:  profileDir,
		// Host units (proc macros, build scripts) land under the host
		// deps dir, not the target-triple dir, even when the wider
		// build is cross-compiling. Mirror cargo's behaviour.
		Platform: platformDir(u, opt.HostTriple),
		// ExtPlatform drives the file extension; for host units this is
		// the resolved host triple so proc macros get .dylib on darwin,
		// .dll on windows, .so elsewhere.
		ExtPlatform: platform,
		CrateName:   underscore(u.Target.Name),
		Hash:        hash,
		TargetKinds: u.Target.Kind,
	})

	depsDir := filepath.Dir(out.DepInfo)

	// For run-custom-build units, pre-compute OUT_DIR so subsequent
	// compile units of the same package can pick it up. Mirrors cargo's
	// `target/<profile>/build/<pkg>-<hash>/out` layout.
	var outDir string
	isRunCustom := u.Mode == "run-custom-build"
	if isRunCustom {
		buildBase := filepath.Join(opt.ProjectRoot, "target")
		if u.Platform != nil && *u.Platform != "" && *u.Platform != opt.HostTriple {
			buildBase = filepath.Join(buildBase, *u.Platform)
		}
		buildBase = filepath.Join(buildBase, profileDir)
		outDir = filepath.Join(buildBase, "build", pkg.Name+"-"+hash, "out")
	}

	return unitDerived{
		pkgID:                id,
		pkg:                  pkg,
		hash:                 hash,
		outputs:              out,
		platform:             platform,
		depsDir:              depsDir,
		isCustomBuildCompile: u.IsCustomBuild() && u.Mode == "build",
		isRunCustomBuild:     isRunCustom,
		outDir:               outDir,
	}, warning, nil
}

// platformDir returns the triple to embed in the target directory path,
// or empty for host-targeted units. Host units always land under
// target/<profile>/deps regardless of the wider --target setting.
func platformDir(u *Unit, host string) string {
	if u.IsHost() {
		return ""
	}
	if u.Platform != nil && *u.Platform != "" && *u.Platform != host {
		return *u.Platform
	}
	return ""
}

func buildInvocation(u *Unit, idx int, derived []unitDerived, opt LowerOptions, ug *UnitGraph) (Invocation, error) {
	d := derived[idx]

	// Deps as indices, preserved from the unit graph.
	deps := make([]int, len(u.Dependencies))
	for i, e := range u.Dependencies {
		deps[i] = e.Index
	}

	switch u.Mode {
	case "run-custom-build":
		return buildRunCustomBuild(u, idx, derived, deps, opt, ug)
	}

	// Default: rustc invocation (build, test, check, etc).
	externs := resolveExterns(u, derived)
	incDir := ""
	if u.Profile.Incremental {
		incDir = incrementalDirFor(opt.ProjectRoot, profileDir(u.Profile.Name), u, d.hash)
	}
	args, err := RustcArgs(ArgsInputs{
		Unit:           u,
		Hash:           d.hash,
		Out:            d.outputs,
		Externs:        externs,
		DepsDir:        d.depsDir,
		IncrementalDir: incDir,
		CapLints:       !isPrimaryPkg(u.PkgID),
	})
	if err != nil {
		return Invocation{}, err
	}

	env := MergeEnv(
		PkgEnv(d.pkg),
		FeatureEnv(u.Features),
		map[string]string{
			"CARGO_CRATE_NAME":      underscore(u.Target.Name),
			"CARGO_MANIFEST_DIR":    manifestDirOf(d, opt),
			"CARGO_MANIFEST_PATH":   filepath.Join(manifestDirOf(d, opt), "Cargo.toml"),
			"CARGO_PRIMARY_PACKAGE": primaryFlag(u),
		},
	)
	if isBinKind(u.Target.Kind) {
		env["CARGO_BIN_NAME"] = u.Target.Name
	}

	// OUT_DIR: if this unit depends on a run-custom-build for its own
	// package, cargo wires that build script's OUT_DIR into this
	// invocation's env so source code can `include!(env!("OUT_DIR"))`.
	for _, e := range u.Dependencies {
		if e.Index < 0 || e.Index >= len(derived) {
			continue
		}
		if derived[e.Index].isRunCustomBuild && derived[e.Index].outDir != "" {
			env["OUT_DIR"] = derived[e.Index].outDir
			break
		}
	}

	outputs := []string{d.outputs.DepInfo}
	if d.outputs.Primary != "" {
		outputs = append([]string{d.outputs.Primary}, outputs...)
	}
	outputs = append(outputs, d.outputs.Extras...)

	return Invocation{
		PackageName:    d.pkg.Name,
		PackageVersion: d.pkg.Version,
		TargetKind:     u.Target.Kind,
		CompileMode:    u.Mode,
		Deps:           deps,
		Outputs:        outputs,
		Links:          map[string]string{},
		Program:        opt.RustcPath,
		Args:           args,
		Env:            env,
		Cwd:            manifestDirOf(d, opt),
	}, nil
}

func buildRunCustomBuild(u *Unit, idx int, derived []unitDerived, deps []int, opt LowerOptions, ug *UnitGraph) (Invocation, error) {
	d := derived[idx]

	// The build script binary was produced by a sibling "build"-mode
	// custom-build unit -- find it via the deps edge.
	var program string
	for _, depIdx := range deps {
		if depIdx < 0 || depIdx >= len(derived) {
			continue
		}
		if depIsCompileBuildScript(&derived[depIdx]) {
			program = derived[depIdx].outputs.Primary
			break
		}
	}
	if program == "" {
		return Invocation{}, fmt.Errorf("run-custom-build: cannot resolve build script binary among deps")
	}
	outDir := d.outDir

	// The run-custom-build unit's own features list is usually empty;
	// cargo sets CARGO_FEATURE_<NAME>=1 vars from the package's actual
	// compile unit features. Find any non-custom-build unit of the same
	// package and copy its features. Stable order (first match wins).
	pkgFeatures := u.Features
	for i := range ug.Units {
		sibling := &ug.Units[i]
		if sibling.PkgID != u.PkgID || sibling.IsCustomBuild() {
			continue
		}
		pkgFeatures = sibling.Features
		break
	}

	env := MergeEnv(
		PkgEnv(d.pkg),
		FeatureEnv(pkgFeatures),
		CargoCfgEnv(opt.Cfg),
		buildScriptEnv(u, opt, outDir),
		map[string]string{
			"CARGO_MANIFEST_DIR":    manifestDirOf(d, opt),
			"CARGO_MANIFEST_PATH":   filepath.Join(manifestDirOf(d, opt), "Cargo.toml"),
			"CARGO_PRIMARY_PACKAGE": primaryFlag(u),
		},
	)

	return Invocation{
		PackageName:    d.pkg.Name,
		PackageVersion: d.pkg.Version,
		TargetKind:     u.Target.Kind,
		CompileMode:    u.Mode,
		Deps:           deps,
		Outputs:        []string{},
		Links:          map[string]string{},
		Program:        program,
		Args:           []string{},
		Env:            env,
		Cwd:            manifestDirOf(d, opt),
	}, nil
}

// buildScriptEnv returns the cargo-set env vars build scripts read at
// runtime (everything beyond CARGO_PKG_*, CARGO_FEATURE_*, CARGO_CFG_*).
func buildScriptEnv(u *Unit, opt LowerOptions, outDir string) map[string]string {
	env := map[string]string{
		"OUT_DIR":   outDir,
		"TARGET":    u.PlatformOrHost(opt.HostTriple),
		"HOST":      opt.HostTriple,
		"OPT_LEVEL": u.Profile.OptLevel,
		"PROFILE":   u.Profile.Name,
		"NUM_JOBS":  "1",
		"RUSTC":     opt.RustcPath,
	}
	if opt.RustcLinker != "" {
		env["RUSTC_LINKER"] = opt.RustcLinker
	}
	if u.Profile.DebugAssertions {
		env["DEBUG"] = "true"
	} else {
		env["DEBUG"] = "false"
	}
	return env
}

func depIsCompileBuildScript(d *unitDerived) bool {
	// Marker set during preDerive when the unit's target_kind contains
	// "custom-build" and mode == "build". More robust than matching the
	// output path basename, which changed when we canonicalised the
	// build-script crate name to underscores.
	return d.isCustomBuildCompile
}

func resolveExterns(u *Unit, derived []unitDerived) []ExternRef {
	refs := make([]ExternRef, 0, len(u.Dependencies))
	for _, e := range u.Dependencies {
		if e.Index < 0 || e.Index >= len(derived) {
			continue
		}
		dep := derived[e.Index]
		if dep.outputs.Primary == "" {
			// Build-script edges have no rlib output; cargo emits the
			// dep edge but no --extern. Skip.
			continue
		}
		refs = append(refs, ExternRef{Name: e.ExternCrateName, Path: dep.outputs.Primary})
	}
	return refs
}

func primaryFlag(u *Unit) string {
	// Cargo sets CARGO_PRIMARY_PACKAGE=1 for units belonging to a
	// workspace member, "" otherwise. Unit-graph doesn't expose the
	// workspace membership directly; for MVP, treat path+ units as
	// primary (the user's workspace) and others as non-primary.
	if isPrimaryPkg(u.PkgID) {
		return "1"
	}
	return ""
}

// isPrimaryPkg reports whether a pkg_id belongs to the local workspace
// (path+) or a fetched dep (registry+ / git+). Drives both
// CARGO_PRIMARY_PACKAGE and --cap-lints decisions.
func isPrimaryPkg(pkgID string) bool {
	return len(pkgID) >= 5 && pkgID[:5] == "path+"
}

func manifestDirOf(d unitDerived, opt LowerOptions) string {
	dir, err := d.pkgID.ManifestDir(opt.CargoHome, opt.RegistryIndex)
	if err != nil {
		return ""
	}
	return dir
}

func isBinKind(kinds []string) bool {
	return slices.Contains(kinds, "bin")
}

// profileDir maps cargo's profile names to their target dir name. Most
// names pass through; the special case is "dev" landing under "debug".
func profileDir(name string) string {
	if name == "dev" {
		return "debug"
	}
	return name
}

// incrementalDirFor mirrors cargo's per-unit incremental cache layout:
// `<root>/target/<profile>/incremental/<crate>-<hash>`. Run creates the
// directory automatically when ensuring output dirs exist (the dir
// holding the dep-info file is the parent of incremental/).
func incrementalDirFor(projectRoot, profileDir string, u *Unit, hash string) string {
	return filepath.Join(projectRoot, "target", profileDir, "incremental", underscore(u.Target.Name)+"-"+hash)
}

// guessRegistryIndex returns the first subdir of $CARGO_HOME/registry/src/
// it finds, on the assumption that most projects use a single registry.
// Returns an error if the dir doesn't exist or is empty.
func guessRegistryIndex(cargoHome string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(cargoHome, "registry", "src"))
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("no registry index dir under %s", cargoHome)
}

// LoadUnitGraph reads a unit-graph JSON file from disk.
func LoadUnitGraph(path string) (*UnitGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read unit graph: %w", err)
	}
	var ug UnitGraph
	if err := json.Unmarshal(data, &ug); err != nil {
		return nil, fmt.Errorf("parse unit graph: %w", err)
	}
	if ug.Version != 1 {
		return nil, fmt.Errorf("unsupported unit graph version %d (want 1)", ug.Version)
	}
	return &ug, nil
}
