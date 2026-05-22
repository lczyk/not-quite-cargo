package cargo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
