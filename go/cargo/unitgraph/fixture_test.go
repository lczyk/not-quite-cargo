package unitgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

// TestFixture_FdLowering loads the captured fd unit-graph fixture,
// lowers it, and sanity-checks the result.
//
// The fixture (`testdata/fd/ug.json`) is the output of
// `cargo build -Z unstable-options --unit-graph` against fd v10.2.0,
// captured locally then path-anonymised via the same `{{PROJECT_ROOT}}`
// / `{{CARGO_HOME}}` / `{{RUSTC}}` placeholders that nqc patch emits.
// See testdata/fd/README.md for provenance and refresh instructions.
//
// We don't have a ground-truth build-plan to diff against (cargo 1.93+
// removed --build-plan, and the host doesn't have an older cargo); the
// test focuses on structural properties any correct lowering must hold.
// A full cargo --build-plan ground truth can be captured by running
// testdata/fd/capture.sh in rust:1.84, which will land the second
// fixture file used by future, stricter, comparison tests.
func TestFixture_FdLowering(t *testing.T) {
	ugPath := filepath.Join("testdata", "fd", "ug.json")
	if _, err := os.Stat(ugPath); os.IsNotExist(err) {
		t.Skipf("fd fixture not present at %s", ugPath)
	}

	ug, err := LoadUnitGraph(ugPath)
	assert.NoError(t, err)

	cfg, err := ParseCfg(fdHostCfg)
	assert.NoError(t, err)

	got, err := Lower(ug, LowerOptions{
		HostTriple:         "aarch64-apple-darwin",
		Cfg:                cfg,
		CargoHome:          "{{CARGO_HOME}}",
		ProjectRoot:        "{{PROJECT_ROOT}}",
		RustcPath:          "{{RUSTC}}",
		SkipManifestErrors: true, // anonymised paths -> no manifests on disk
	})
	assert.NoError(t, err)

	// fd v10.2.0 at the captured commit had 77 units (see CHANGELOG for
	// the fixture). If the dep tree changes the fixture should be
	// refreshed via testdata/fd/capture.sh.
	assert.Equal(t, len(got.Invocations), len(ug.Units), "one invocation per unit")
	assert.Equal(t, len(got.Invocations), 77, "expected 77 units for the pinned fd v10.2.0 fixture")

	// Every invocation must have a non-empty package_name (the manifest
	// fallback should at minimum derive name from pkg_id).
	for i, inv := range got.Invocations {
		assert.That(t, inv.PackageName != "",
			"invocation %d (%s) has empty package_name", i, ug.Units[i].PkgID)
	}

	// At least one proc-macro unit and one custom-build unit -- fd
	// pulls in clap_derive (proc-macro) and various deps with build
	// scripts. Confirms our type-aware lowering branched correctly.
	var sawProcMacro, sawCustomBuild, sawRunCustomBuild bool
	for i := range ug.Units {
		if ug.Units[i].IsProcMacro() {
			sawProcMacro = true
		}
		if ug.Units[i].IsCustomBuild() && ug.Units[i].Mode == "build" {
			sawCustomBuild = true
		}
		if ug.Units[i].Mode == "run-custom-build" {
			sawRunCustomBuild = true
		}
	}
	assert.That(t, sawProcMacro, "fd fixture should contain at least one proc-macro unit")
	assert.That(t, sawCustomBuild, "fd fixture should contain at least one custom-build compile unit")
	assert.That(t, sawRunCustomBuild, "fd fixture should contain at least one run-custom-build unit")

	// Spot-check the root fd-find unit: must be a bin compile, edition
	// 2021, with --extern proc_macro absent (it's not a proc-macro
	// crate), --cap-lints absent (fd-find itself is the primary
	// workspace member), and CARGO_BIN_NAME set.
	fdIdx := -1
	for i, inv := range got.Invocations {
		if inv.PackageName == "fd-find" && inv.CompileMode == "build" {
			// Pick the unit whose target is "fd" (the bin), not
			// "build_script_build".
			if len(inv.TargetKind) > 0 && inv.TargetKind[0] == "bin" {
				fdIdx = i
				break
			}
		}
	}
	assert.That(t, fdIdx >= 0, "fd-find bin unit not found")
	if fdIdx >= 0 {
		fd := got.Invocations[fdIdx]
		assert.Equal(t, fd.Env["CARGO_BIN_NAME"], "fd")
		assert.Equal(t, fd.Env["CARGO_CRATE_NAME"], "fd")
		assert.Equal(t, fd.Env["CARGO_PRIMARY_PACKAGE"], "1")
		// Args must contain --crate-name fd, --edition=2021, no --cap-lints.
		var sawEdition, sawCapLints bool
		for i, a := range fd.Args {
			if a == "--edition=2021" {
				sawEdition = true
			}
			if a == "--cap-lints" {
				sawCapLints = true
			}
			_ = i
		}
		assert.That(t, sawEdition, "fd unit must have --edition=2021")
		assert.That(t, !sawCapLints, "fd-find is primary; should not get --cap-lints")
	}

	// Spot-check a proc-macro unit: must include --extern proc_macro
	// and --cap-lints warn (clap_derive is a registry dep).
	for _, inv := range got.Invocations {
		if inv.PackageName != "clap_derive" || inv.CompileMode != "build" {
			continue
		}
		var sawExternPM, sawCapLints bool
		for i, a := range inv.Args {
			if a == "--extern" && i+1 < len(inv.Args) && inv.Args[i+1] == "proc_macro" {
				sawExternPM = true
			}
			if a == "--cap-lints" {
				sawCapLints = true
			}
		}
		assert.That(t, sawExternPM, "clap_derive must get --extern proc_macro")
		assert.That(t, sawCapLints, "clap_derive is non-primary; should get --cap-lints")
		break
	}
}

// fdHostCfg is a representative rustc --print cfg output for aarch64
// linux (the demo's target). embedded inline so the test stays hermetic.
const fdHostCfg = `debug_assertions
panic="unwind"
target_arch="aarch64"
target_endian="little"
target_env="gnu"
target_family="unix"
target_feature="aes"
target_feature="crc"
target_feature="neon"
target_has_atomic
target_has_atomic="16"
target_has_atomic="32"
target_has_atomic="64"
target_has_atomic="8"
target_has_atomic="ptr"
target_os="linux"
target_pointer_width="64"
target_vendor="unknown"
unix
`
