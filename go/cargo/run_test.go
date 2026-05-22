package cargo

import (
	"strings"
	"testing"
)

// envMap collapses the kv-string slice that buildEnv returns into a map for
// easy assertions.
func envMap(t *testing.T, kvs []string) map[string]string {
	t.Helper()
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			t.Fatalf("malformed env entry: %q", kv)
		}
		if _, dup := m[k]; dup {
			t.Fatalf("duplicate key %q in env (dedup broken)", k)
		}
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

	// Invocation env wins over cfg.
	if got["RUSTC"] != "/inv/rustc" {
		t.Errorf("RUSTC: invocation env should override cfg, got %q", got["RUSTC"])
	}
	// Cfg wins over parent.
	if got["CARGO_HOME"] != "/cfg/cargo" {
		t.Errorf("CARGO_HOME: cfg should override parent, got %q", got["CARGO_HOME"])
	}
	if got["PROJECT_ROOT"] != "/cfg/proj" {
		t.Errorf("PROJECT_ROOT: cfg should override parent, got %q", got["PROJECT_ROOT"])
	}
	// Invocation wins over parent (cfg doesn't set this key).
	if got["CARGO_PKG_NAME"] != "inv-pkg" {
		t.Errorf("CARGO_PKG_NAME: invocation should override parent, got %q", got["CARGO_PKG_NAME"])
	}
	// Invocation-only key passes through.
	if got["INV_ONLY"] != "yes" {
		t.Errorf("INV_ONLY: missing or wrong, got %q", got["INV_ONLY"])
	}
}
