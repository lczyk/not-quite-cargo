package cargo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestPatch_Golden patches a fixture plan in a temp dir and compares the
// result, byte for byte after re-encoding, to the expected fixture.
func TestPatch_Golden(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(tmp, src, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		ProjectRoot: "/proj/root",
		CargoHome:   "/cargo/home",
		RustcPath:   "/some/rustc",
	}
	if err := Patch(tmp, cfg); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "plan.small.patched.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Normalise both via Go's json round-trip so whitespace / key ordering
	// quirks don't drive a false negative -- semantic equality is what matters.
	if !jsonEqual(t, got, want) {
		t.Errorf("patched output does not match golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPatch_RUSTCNotWrittenToFile(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(tmp, src, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		ProjectRoot: "/proj/root",
		CargoHome:   "/cargo/home",
		// Pick a path that occurs in the file; if Patch ever reverse-mapped it
		// to {{RUSTC}}, this test would catch the regression.
		RustcPath: "/proj/root",
	}
	if err := Patch(tmp, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("{{RUSTC}}")) {
		// program=rustc -> {{RUSTC}} is allowed, but every other occurrence
		// must come from string replacement, which we explicitly skip.
		// Decode and check program field only.
		var plan map[string]any
		if err := json.Unmarshal(got, &plan); err != nil {
			t.Fatal(err)
		}
		invs := plan["invocations"].([]any)
		for _, raw := range invs {
			inv := raw.(map[string]any)
			body, _ := json.Marshal(inv)
			// strip out the program field, then check no {{RUSTC}} remains.
			delete(inv, "program")
			stripped, _ := json.Marshal(inv)
			if bytes.Contains(stripped, []byte("{{RUSTC}}")) {
				t.Errorf("{{RUSTC}} leaked into non-program field: %s", body)
			}
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
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{ProjectRoot: "/proj", CargoHome: "/cargo", RustcPath: "/r"}
	if err := Patch(tmp, cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(tmp)
	var out map[string]any
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatal(err)
	}
	args := out["invocations"].([]any)[0].(map[string]any)["args"].([]any)
	want := []any{"--edition=2021", "--crate-name", "x", "--out-dir", "/tmp/out"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args mismatch\n got: %v\nwant: %v", args, want)
	}
}

func TestPatch_AtomicWrite(t *testing.T) {
	// On success the target file ends up updated. Sanity check that no
	// stray temp file is left behind in the dir.
	src, _ := os.ReadFile(filepath.Join("testdata", "plan.small.json"))
	dir := t.TempDir()
	tmp := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(tmp, src, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{ProjectRoot: "/proj/root", CargoHome: "/cargo/home", RustcPath: "/r"}
	if err := Patch(tmp, cfg); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "plan.json" {
			t.Errorf("stray file left behind: %s", e.Name())
		}
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	ajson, _ := json.Marshal(av)
	bjson, _ := json.Marshal(bv)
	return bytes.Equal(ajson, bjson)
}
