package cargo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/lczyk/assert"
)

func planWithPaths(t *testing.T, profile string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{
		"invocations": [
			{
				"args": ["-C", "opt-level=3", "-C", "debuginfo=0", "--out-dir", "/work/target/%[1]s/deps"],
				"outputs": ["/work/target/%[1]s/libfoo.rlib"],
				"links": { "/work/target/%[1]s/foo": "/work/target/%[1]s/deps/foo-abc" },
				"cwd": "/work",
				"env": { "PROFILE": "%[1]s", "OPT_LEVEL": "3", "DEBUG": "false" }
			}
		],
		"inputs": ["/work/Cargo.toml"]
	}`, profile)
	var plan map[string]any
	assert.NoError(t, json.Unmarshal([]byte(body), &plan))
	return plan
}

func firstInvField(t *testing.T, plan map[string]any, key string) any {
	t.Helper()
	invs, ok := plan["invocations"].([]any)
	assert.That(t, ok, "invocations missing")
	inv, ok := invs[0].(map[string]any)
	assert.That(t, ok, "invocation[0] shape")
	return inv[key]
}

func argStrs(t *testing.T, plan map[string]any) []string {
	t.Helper()
	args, ok := firstInvField(t, plan, "args").([]any)
	assert.That(t, ok, "args shape")
	out := make([]string, 0, len(args))
	for _, a := range args {
		if s, ok := a.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func anyContains(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestRewriteProfile_ReleaseToDebug(t *testing.T) {
	plan := planWithPaths(t, "release")
	RewriteProfile(plan, Debug)

	strs := argStrs(t, plan)
	assert.That(t, slices.Contains(strs, "opt-level=0"), "expected opt-level=0 in %v", strs)
	assert.That(t, slices.Contains(strs, "debuginfo=2"), "expected debuginfo=2 in %v", strs)
	assert.That(t, anyContains(strs, "/debug/deps"), "expected /debug/deps in %v", strs)
	assert.That(t, !anyContains(strs, "/release/"), "unexpected /release/ in %v", strs)

	outs := firstInvField(t, plan, "outputs").([]any)
	out0 := outs[0].(string)
	assert.That(t, strings.Contains(out0, "/debug/"), "outputs[0]=%s", out0)
	assert.That(t, !strings.Contains(out0, "/release/"), "outputs[0]=%s", out0)

	env := firstInvField(t, plan, "env").(map[string]any)
	assert.Equal(t, env["PROFILE"].(string), "debug")
	assert.Equal(t, env["OPT_LEVEL"].(string), "0")
	assert.Equal(t, env["DEBUG"].(string), "true")
}

func TestRewriteProfile_DebugToRelease(t *testing.T) {
	plan := planWithPaths(t, "debug")
	RewriteProfile(plan, Release)

	strs := argStrs(t, plan)
	assert.That(t, slices.Contains(strs, "opt-level=3"), "expected opt-level=3 in %v", strs)
	assert.That(t, slices.Contains(strs, "debuginfo=0"), "expected debuginfo=0 in %v", strs)
	assert.That(t, anyContains(strs, "/release/deps"), "expected /release/deps in %v", strs)
	assert.That(t, !anyContains(strs, "/debug/"), "unexpected /debug/ in %v", strs)
}

func TestRewriteProfile_Idempotent(t *testing.T) {
	plan := planWithPaths(t, "release")
	before, err := json.Marshal(plan)
	assert.NoError(t, err)
	RewriteProfile(plan, Release)
	after, err := json.Marshal(plan)
	assert.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

func TestParseProfile(t *testing.T) {
	_, ok := ParseProfile("release")
	assert.That(t, ok, "release should parse")
	_, ok = ParseProfile("debug")
	assert.That(t, ok, "debug should parse")
	_, ok = ParseProfile("dev")
	assert.That(t, !ok, "dev should not parse")
	_, ok = ParseProfile("")
	assert.That(t, !ok, "empty should not parse")
}
