package unitgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
	"github.com/lczyk/assert/require"
)

// Container-side paths the capture.sh script bakes into the JSON
// fixtures. capture.sh runs inside rust:1.84 with fd cloned at /tmp/fd
// and CARGO_HOME=/tmp/cargo-home, so every absolute path in ug.json and
// build-plan.json references one of these prefixes.
const (
	fdFixtureProjectRoot = "/tmp/fd"
	fdFixtureCargoHome   = "/tmp/cargo-home"
)

// TestFixture_Fd loads the captured fd unit-graph fixture,
// builds the corresponding plan, and sanity-checks the result.
//
// The fixture files (`testdata/fd/{unit-graph,build-plan}.json`) are
// produced by `testdata/fd/capture.sh` against rust:1.84 -- see the
// README in that dir for provenance. The test is skipped when the
// fixture isn't present so contributors can land code changes without
// running the (docker-heavy) capture pipeline.
func TestFixture_Fd(t *testing.T) {
	dir := filepath.Join("testdata", "fd")
	ugPath := filepath.Join(dir, "unit-graph.json")
	bpPath := filepath.Join(dir, "build-plan.json")

	if _, err := os.Stat(ugPath); os.IsNotExist(err) {
		t.Skipf("fd fixture not present (run testdata/fd/capture.sh to populate)")
	}

	ug, err := LoadUnitGraph(ugPath)
	require.NoError(t, err)

	got, err := Build(ug, BuildOptions{
		// Capture container is linux/<host-arch>; pass linux + a
		// representative arch. ProjectRoot + CargoHome derived from the
		// unit-graph automatically (path+ + registry+ source paths).
		Target:    Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		RustcPath: "rustc",
	})
	require.NoError(t, err)

	assert.Equal(t, len(got.Invocations), len(ug.Units), "one invocation per unit")

	// Every invocation must have a non-empty package_name (the manifest
	// fallback should at minimum derive name from pkg_id).
	for i, inv := range got.Invocations {
		assert.That(t, inv.PackageName != "",
			"invocation %d (%s) has empty package_name", i, ug.Units[i].PkgID)
	}

	// At least one proc-macro unit and one custom-build unit -- fd
	// pulls in clap_derive (proc-macro) and various deps with build
	// scripts. Confirms our type-aware build branched correctly.
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
		var sawEdition, sawCapLints bool
		for _, a := range fd.Args {
			if a == "--edition=2021" {
				sawEdition = true
			}
			if a == "--cap-lints" {
				sawCapLints = true
			}
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

	// Cross-check against the cargo-emitted build-plan ground truth, if
	// present. The capture.sh script always produces both files; older
	// fixtures might only carry ug.json.
	if _, err := os.Stat(bpPath); err == nil {
		bpData, err := os.ReadFile(bpPath)
		assert.NoError(t, err)
		var bp struct {
			Invocations []map[string]any `json:"invocations"`
		}
		assert.NoError(t, json.Unmarshal(bpData, &bp))
		assert.Equal(t, len(got.Invocations), len(bp.Invocations),
			"built invocation count should match cargo's --build-plan ground truth")
	}
}
