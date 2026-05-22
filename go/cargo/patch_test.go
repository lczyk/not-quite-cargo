package cargo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lczyk/assert"
)

// patchFile loads a plan from disk, applies PatchPlan with the given
// roots, marshals back and returns the encoded JSON.
func patchFile(t *testing.T, path, projectRoot, cargoHome string) []byte {
	t.Helper()
	plan, err := loadPlanJSON(path)
	assert.NoError(t, err)
	out, err := PatchPlan(plan, projectRoot, cargoHome)
	assert.NoError(t, err)
	body, err := json.MarshalIndent(out, "", "    ")
	assert.NoError(t, err)
	return body
}

// TestPatch_Golden patches a fixture plan and compares the result, byte
// for byte after re-encoding, to the expected fixture.
func TestPatch_Golden(t *testing.T) {
	src := filepath.Join("testdata", "plan.small.json")
	got := patchFile(t, src, "/proj/root", "/cargo/home")
	want, err := os.ReadFile(filepath.Join("testdata", "plan.small.patched.json"))
	assert.NoError(t, err)
	assert.That(t, jsonEqual(t, got, want),
		"patched output does not match golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
}

func TestPatch_RUSTCNotWrittenToFile(t *testing.T) {
	src := filepath.Join("testdata", "plan.small.json")
	// Pick a path that occurs in the file; if PatchPlan ever reverse-mapped it
	// to {{RUSTC}}, this test would catch the regression. (We pass /proj/root
	// as the cargoHome too -- nonsense but enough to exercise the path.)
	got := patchFile(t, src, "/proj/root", "/proj/root")
	if bytes.Contains(got, []byte("{{RUSTC}}")) {
		// program=rustc -> {{RUSTC}} is allowed, but every other occurrence
		// must come from string replacement, which we explicitly skip.
		var plan map[string]any
		assert.NoError(t, json.Unmarshal(got, &plan))
		invs := plan["invocations"].([]any)
		for _, raw := range invs {
			inv := raw.(map[string]any)
			delete(inv, "program")
			stripped, _ := json.Marshal(inv)
			assert.That(t, !bytes.Contains(stripped, []byte("{{RUSTC}}")),
				"{{RUSTC}} leaked into non-program field: %s", stripped)
		}
	}
}

func TestPatch_DiagnosticWidthTwoArg(t *testing.T) {
	// Synthesise a plan with both forms of --diagnostic-width: the joined
	// form (--diagnostic-width=120) and the bare two-arg form
	// (--diagnostic-width 120). Both must be stripped, with the value arg
	// of the bare form also gone.
	plan := map[string]any{
		"invocations": []any{
			map[string]any{
				"package_name":    "x",
				"package_version": "0.1.0",
				"target_kind":     []any{"bin"},
				"compile_mode":    "build",
				"deps":            []any{},
				"outputs":         []any{},
				"links":           map[string]any{},
				"program":         "rustc",
				"args": []any{
					"--edition=2021",
					"--diagnostic-width",
					"120",
					"--crate-name", "x",
					"--diagnostic-width=80",
					"--out-dir", "/tmp/out",
				},
				"env": map[string]any{},
				"cwd": "/proj",
			},
		},
	}
	out, err := PatchPlan(plan, "/proj", "/cargo")
	assert.NoError(t, err)
	args := out["invocations"].([]any)[0].(map[string]any)["args"].([]any)
	want := []any{"--edition=2021", "--crate-name", "x", "--out-dir", "/tmp/out"}
	assert.EqualCmpAny(t, args, want, func(a, b any) bool { return reflect.DeepEqual(a, b) })
}

func TestPatch_EmptyArgsError(t *testing.T) {
	plan := map[string]any{"invocations": []any{}}
	_, err := PatchPlan(plan, "", "/cargo")
	assert.That(t, err != nil, "expected error when projectRoot is empty")
	_, err = PatchPlan(plan, "/proj", "")
	assert.That(t, err != nil, "expected error when cargoHome is empty")
}

func TestWriteAtomic_NoStrayTempFile(t *testing.T) {
	// WriteAtomic should leave only the target file behind on success.
	dir := t.TempDir()
	tmp := filepath.Join(dir, "out.json")
	assert.NoError(t, WriteAtomic(tmp, []byte("ok"), 0o644))
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	for _, e := range entries {
		assert.Equal(t, e.Name(), "out.json", "stray file left behind")
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	assert.NoError(t, json.Unmarshal(a, &av), "decode got")
	assert.NoError(t, json.Unmarshal(b, &bv), "decode want")
	ajson, _ := json.Marshal(av)
	bjson, _ := json.Marshal(bv)
	return bytes.Equal(ajson, bjson)
}
