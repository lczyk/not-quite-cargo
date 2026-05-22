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
