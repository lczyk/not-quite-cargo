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

// TestPatch_Golden patches a fixture plan in a temp dir and compares the
// result, byte for byte after re-encoding, to the expected fixture.
func TestPatch_Golden(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	assert.NoError(t, err)
	tmp := filepath.Join(t.TempDir(), "plan.json")
	assert.NoError(t, os.WriteFile(tmp, src, 0o644))

	cfg := &Config{
		ProjectRoot: "/proj/root",
		CargoHome:   "/cargo/home",
		RustcPath:   "/some/rustc",
	}
	assert.NoError(t, Patch(tmp, cfg))

	got, err := os.ReadFile(tmp)
	assert.NoError(t, err)
	want, err := os.ReadFile(filepath.Join("testdata", "plan.small.patched.json"))
	assert.NoError(t, err)
	assert.That(t, jsonEqual(t, got, want),
		"patched output does not match golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
}

func TestPatch_RUSTCNotWrittenToFile(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	assert.NoError(t, err)
	tmp := filepath.Join(t.TempDir(), "plan.json")
	assert.NoError(t, os.WriteFile(tmp, src, 0o644))
	cfg := &Config{
		ProjectRoot: "/proj/root",
		CargoHome:   "/cargo/home",
		// Pick a path that occurs in the file; if Patch ever reverse-mapped it
		// to {{RUSTC}}, this test would catch the regression.
		RustcPath: "/proj/root",
	}
	assert.NoError(t, Patch(tmp, cfg))
	got, err := os.ReadFile(tmp)
	assert.NoError(t, err)
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
	body, _ := json.Marshal(plan)
	tmp := filepath.Join(t.TempDir(), "plan.json")
	assert.NoError(t, os.WriteFile(tmp, body, 0o644))
	cfg := &Config{ProjectRoot: "/proj", CargoHome: "/cargo", RustcPath: "/r"}
	assert.NoError(t, Patch(tmp, cfg))
	got, _ := os.ReadFile(tmp)
	var out map[string]any
	assert.NoError(t, json.Unmarshal(got, &out))
	args := out["invocations"].([]any)[0].(map[string]any)["args"].([]any)
	want := []any{"--edition=2021", "--crate-name", "x", "--out-dir", "/tmp/out"}
	assert.EqualCmpAny(t, args, want, func(a, b any) bool { return reflect.DeepEqual(a, b) })
}

func TestPatch_AtomicWrite(t *testing.T) {
	// On success the target file ends up updated. Sanity check that no
	// stray temp file is left behind in the dir.
	src, _ := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	dir := t.TempDir()
	tmp := filepath.Join(dir, "plan.json")
	assert.NoError(t, os.WriteFile(tmp, src, 0o644))
	cfg := &Config{ProjectRoot: "/proj/root", CargoHome: "/cargo/home", RustcPath: "/r"}
	assert.NoError(t, Patch(tmp, cfg))
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	for _, e := range entries {
		assert.Equal(t, e.Name(), "plan.json", "stray file left behind")
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
