package cargo

import (
	"reflect"
	"testing"

	"github.com/lczyk/assert"
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
	assert.EqualCmpAny(t, got, want, func(a, b any) bool { return reflect.DeepEqual(a, b) })
}

func TestDeepReplace_NonStringScalar(t *testing.T) {
	got := DeepReplace(42, map[string]string{"42": "no"})
	assert.Equal(t, got, any(42))
}

func TestDeepReplace_OverlappingKeys(t *testing.T) {
	// Regression for the patch-overlap bug: when one replacement key is a
	// prefix of another (PROJECT_ROOT being a parent dir of CARGO_HOME), the
	// longer key must win regardless of map iteration order.
	repl := map[string]string{
		"/home/u/proj":        "{{PROJECT_ROOT}}",
		"/home/u/proj/.cargo": "{{CARGO_HOME}}",
	}
	for i := 0; i < 100; i++ {
		got := DeepReplace("/home/u/proj/.cargo/registry", repl)
		assert.Equal(t, got, any("{{CARGO_HOME}}/registry"), "iter %d", i)
	}
}
