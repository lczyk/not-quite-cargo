package cargo

import (
	"reflect"
	"testing"
)

func TestDeepReplace(t *testing.T) {
	repl := map[string]string{
		"{{A}}": "alpha",
		"{{B}}": "beta",
	}
	in := map[string]any{
		"{{A}}-key": "{{A}} value",
		"nested": map[string]any{
			"list": []any{"{{B}}/x", 42, true, nil},
		},
		"plain": "no-placeholder",
	}
	got := DeepReplace(in, repl)
	want := map[string]any{
		"alpha-key": "alpha value",
		"nested": map[string]any{
			"list": []any{"beta/x", 42, true, nil},
		},
		"plain": "no-placeholder",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeepReplace mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDeepReplace_NonStringScalar(t *testing.T) {
	// Numbers, bools, nil pass through untouched even if replacements exist.
	got := DeepReplace(42, map[string]string{"42": "no"})
	if got != 42 {
		t.Fatalf("expected 42, got %v", got)
	}
}

func TestDeepReplace_OverlappingKeys(t *testing.T) {
	// Regression for the patch-overlap bug: when one replacement key is a
	// prefix of another (PROJECT_ROOT being a parent dir of CARGO_HOME), the
	// longer key must win regardless of map iteration order.
	repl := map[string]string{
		"/home/u/proj":        "{{PROJECT_ROOT}}",
		"/home/u/proj/.cargo": "{{CARGO_HOME}}",
	}
	// Run many times -- with map iteration this is non-deterministic; with
	// the length-desc fix the answer is the same every time.
	want := "{{CARGO_HOME}}/registry"
	for i := 0; i < 100; i++ {
		got := DeepReplace("/home/u/proj/.cargo/registry", repl)
		if got != want {
			t.Fatalf("iter %d: got %q, want %q", i, got, want)
		}
	}
}
