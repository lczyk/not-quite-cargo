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
