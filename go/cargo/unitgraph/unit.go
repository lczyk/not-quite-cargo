package unitgraph

import "slices"

// Unit mirrors a single entry in cargo's `--unit-graph` JSON output.
//
// Fields are decoded directly from the JSON shape cargo emits; only the
// fields the lowerer reads are modelled (others are tolerated as unknown
// keys by encoding/json's default behaviour).
type Unit struct {
	PkgID        string      `json:"pkg_id"`
	Target       UnitTarget  `json:"target"`
	Profile      UnitProfile `json:"profile"`
	Platform     *string     `json:"platform"` // null = host
	Mode         string      `json:"mode"`     // "build", "test", "check", "doc", "run-custom-build", ...
	Features     []string    `json:"features"`
	Dependencies []UnitDep   `json:"dependencies"`
}

// UnitTarget models the `target` sub-object inside a unit.
type UnitTarget struct {
	Kind       []string `json:"kind"`        // ["lib"], ["bin"], ["proc-macro"], ["custom-build"], ...
	CrateTypes []string `json:"crate_types"` // ["lib","rlib"], ["bin"], ["proc-macro"], ...
	Name       string   `json:"name"`
	SrcPath    string   `json:"src_path"`
	Edition    string   `json:"edition"`
	Doc        bool     `json:"doc"`
	Doctest    bool     `json:"doctest"`
	Test       bool     `json:"test"`
}

// UnitProfile models the `profile` sub-object inside a unit.
type UnitProfile struct {
	Name            string  `json:"name"`
	OptLevel        string  `json:"opt_level"`
	LTO             string  `json:"lto"`
	CodegenBackend  *string `json:"codegen_backend"`
	CodegenUnits    *int    `json:"codegen_units"`
	Debuginfo       any     `json:"debuginfo"` // can be int (0/1/2) or string ("line-tables-only")
	SplitDebuginfo  *string `json:"split_debuginfo"`
	DebugAssertions bool    `json:"debug_assertions"`
	OverflowChecks  bool    `json:"overflow_checks"`
	Rpath           bool    `json:"rpath"`
	Incremental     bool    `json:"incremental"`
	Panic           string  `json:"panic"`
}

// UnitDep models an edge in the unit graph.
type UnitDep struct {
	Index           int    `json:"index"`
	ExternCrateName string `json:"extern_crate_name"`
	Public          bool   `json:"public"`
	Noprelude       bool   `json:"noprelude"`
}

// UnitGraph is the top-level shape of cargo's `--unit-graph` JSON.
type UnitGraph struct {
	Version int    `json:"version"`
	Units   []Unit `json:"units"`
	Roots   []int  `json:"roots"`
}

// PlatformOrHost returns the target triple if the unit specifies one,
// or the supplied host triple if the unit targets the host (i.e.
// `platform: null` in unit-graph).
func (u *Unit) PlatformOrHost(host string) string {
	if u.Platform == nil || *u.Platform == "" {
		return host
	}
	return *u.Platform
}

// IsHost reports whether the unit compiles to / runs on the host. Proc
// macros and build scripts are host units even when the wider build is
// targeting a different platform.
func (u *Unit) IsHost() bool {
	return u.Platform == nil
}

// IsProcMacro reports whether the unit produces a procedural macro.
func (u *Unit) IsProcMacro() bool {
	return slices.Contains(u.Target.Kind, "proc-macro")
}

// IsCustomBuild reports whether the unit relates to a build script
// (either compiling build.rs or executing the resulting binary).
func (u *Unit) IsCustomBuild() bool {
	return slices.Contains(u.Target.Kind, "custom-build")
}
