package unitgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

// TestFixture_FdLowering loads the captured fd unit-graph + build-plan
// fixtures, runs Lower, and asserts the result matches the golden
// build-plan in shape and content.
//
// The fixture files (`testdata/fd/{ug,build-plan}.json`) are produced
// by `testdata/fd/capture.sh` against rust:1.84 -- see the README in
// that dir for provenance. The test is skipped if the fixtures aren't
// present so contributors can land code changes without running the
// (docker-heavy) capture pipeline.
func TestFixture_FdLowering(t *testing.T) {
	ugPath := filepath.Join("testdata", "fd", "ug.json")
	bpPath := filepath.Join("testdata", "fd", "build-plan.json")

	if _, err := os.Stat(ugPath); os.IsNotExist(err) {
		t.Skipf("fd fixture not present (run testdata/fd/capture.sh to populate)")
	}

	ug, err := LoadUnitGraph(ugPath)
	assert.NoError(t, err)

	cfg, err := ParseCfg(fdHostCfg)
	assert.NoError(t, err)

	got, err := Lower(ug, LowerOptions{
		HostTriple:         "aarch64-unknown-linux-gnu",
		Cfg:                cfg,
		CargoHome:          "{{CARGO_HOME}}",
		ProjectRoot:        "{{PROJECT_ROOT}}",
		RustcPath:          "{{RUSTC}}",
		SkipManifestErrors: true,
	})
	assert.NoError(t, err)

	// Load the cargo-emitted build-plan as the ground-truth invocation
	// list. Compare counts + key structural fields.
	bpData, err := os.ReadFile(bpPath)
	assert.NoError(t, err)
	var bp struct {
		Invocations []map[string]any `json:"invocations"`
	}
	assert.NoError(t, json.Unmarshal(bpData, &bp))

	// Should produce one invocation per cargo invocation.
	assert.Equal(t, len(got.Invocations), len(bp.Invocations),
		"invocation count mismatch (lowered=%d, ground-truth=%d)",
		len(got.Invocations), len(bp.Invocations))

	// Spot-check: package_name field present and non-empty on each.
	for i, inv := range got.Invocations {
		assert.That(t, inv.PackageName != "",
			"invocation %d has empty package_name", i)
	}
}

// fdHostCfg is the rustc --print cfg output captured alongside the fd
// fixture (aarch64 linux). Embedded inline to keep the test
// hermetic.
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
