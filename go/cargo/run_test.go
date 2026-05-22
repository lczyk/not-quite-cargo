package cargo

import (
	"strings"
	"testing"

	"github.com/lczyk/assert"
)

// envMap collapses the kv-string slice that buildEnv returns into a map for
// easy assertions, asserting no duplicate keys exist (dedup invariant).
func envMap(t *testing.T, kvs []string) map[string]string {
	t.Helper()
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		assert.That(t, ok, "malformed env entry: %q", kv)
		_, dup := m[k]
		assert.That(t, !dup, "duplicate key %q in env (dedup broken)", k)
		m[k] = v
	}
	return m
}

func TestBuildEnv_PrecedenceAndDedupe(t *testing.T) {
	// Pollute the parent env with values for the keys the runtime injects --
	// these should be overridden by cfg, not deduplicated against it.
	t.Setenv("RUSTC", "/parent/rustc")
	t.Setenv("CARGO_HOME", "/parent/cargo")
	t.Setenv("PROJECT_ROOT", "/parent/proj")
	t.Setenv("CARGO_PKG_NAME", "parent-pkg")

	cfg := &Config{
		ProjectRoot: "/cfg/proj",
		CargoHome:   "/cfg/cargo",
		RustcPath:   "/cfg/rustc",
	}
	invEnv := map[string]string{
		"CARGO_PKG_NAME": "inv-pkg",    // overrides parent
		"RUSTC":          "/inv/rustc", // overrides cfg
		"INV_ONLY":       "yes",
	}
	got := envMap(t, buildEnv(cfg, invEnv))

	assert.Equal(t, got["RUSTC"], "/inv/rustc", "invocation env overrides cfg")
	assert.Equal(t, got["CARGO_HOME"], "/cfg/cargo", "cfg overrides parent")
	assert.Equal(t, got["PROJECT_ROOT"], "/cfg/proj", "cfg overrides parent")
	assert.Equal(t, got["CARGO_PKG_NAME"], "inv-pkg", "invocation overrides parent")
	assert.Equal(t, got["INV_ONLY"], "yes", "invocation-only key passes through")
}
