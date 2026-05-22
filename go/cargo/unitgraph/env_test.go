package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

// sample mirrors a real rustc --print cfg snippet for aarch64-apple-darwin
// (trimmed to keep the test focused on the parser's edge cases).
const sampleCfg = `debug_assertions
panic="unwind"
target_arch="aarch64"
target_endian="little"
target_env=""
target_family="unix"
target_feature="aes"
target_feature="crc"
target_feature="neon"
target_has_atomic
target_has_atomic="16"
target_has_atomic="32"
target_has_atomic="64"
target_os="macos"
target_pointer_width="64"
target_vendor="apple"
unix`

func TestParseCfg_AllShapes(t *testing.T) {
	got, err := ParseCfg(sampleCfg)
	assert.NoError(t, err)
	// bare key
	assert.EqualArrays(t, got["unix"], []string{""})
	assert.EqualArrays(t, got["debug_assertions"], []string{""})
	// quoted single value
	assert.EqualArrays(t, got["target_os"], []string{"macos"})
	// quoted empty value
	assert.EqualArrays(t, got["target_env"], []string{""})
	// repeated key accumulates in order
	assert.EqualArrays(t, got["target_feature"], []string{"aes", "crc", "neon"})
	// bare key plus repeated quoted values
	assert.EqualArrays(t, got["target_has_atomic"], []string{"", "16", "32", "64"})
}

func TestParseCfg_EmptyAndComments(t *testing.T) {
	got, err := ParseCfg("\n// a comment\nunix\n\n\n")
	assert.NoError(t, err)
	assert.EqualArrays(t, got["unix"], []string{""})
	assert.Equal(t, len(got), 1)
}

func TestParseCfg_MalformedValueRejected(t *testing.T) {
	_, err := ParseCfg(`target_os=macos`) // missing quotes
	assert.Error(t, err, "not quoted")
}

func TestParseCfg_EmptyKeyRejected(t *testing.T) {
	_, err := ParseCfg(`=hi`)
	assert.Error(t, err, "empty key")
}

func TestCargoCfgEnv_NamingAndJoining(t *testing.T) {
	cfg, err := ParseCfg(sampleCfg)
	assert.NoError(t, err)
	env := CargoCfgEnv(cfg)

	// boolean cfg present, value empty
	v, ok := env["CARGO_CFG_UNIX"]
	assert.That(t, ok)
	assert.Equal(t, v, "")

	// scalar quoted
	assert.Equal(t, env["CARGO_CFG_TARGET_OS"], "macos")

	// multi-value comma-joined in order
	assert.Equal(t, env["CARGO_CFG_TARGET_FEATURE"], "aes,crc,neon")

	// mixed bare + quoted comma-joined incl. empty
	assert.Equal(t, env["CARGO_CFG_TARGET_HAS_ATOMIC"], ",16,32,64")
}

func TestCargoCfgEnv_HyphenToUnderscore(t *testing.T) {
	cfg, err := ParseCfg(`has-foo`)
	assert.NoError(t, err)
	env := CargoCfgEnv(cfg)
	_, ok := env["CARGO_CFG_HAS_FOO"]
	assert.That(t, ok)
}

func TestPkgEnv_BasicAndSemverSplit(t *testing.T) {
	env := PkgEnv(PkgMetadata{
		Name:        "fd-find",
		Version:     "10.2.0",
		Authors:     []string{"David Peter <mail@david-peter.de>"},
		Description: "A simple, fast and user-friendly alternative to find",
		Homepage:    "https://github.com/sharkdp/fd",
		License:     "MIT OR Apache-2.0",
		Repository:  "https://github.com/sharkdp/fd",
	})
	assert.Equal(t, env["CARGO_PKG_NAME"], "fd-find")
	assert.Equal(t, env["CARGO_PKG_VERSION"], "10.2.0")
	assert.Equal(t, env["CARGO_PKG_VERSION_MAJOR"], "10")
	assert.Equal(t, env["CARGO_PKG_VERSION_MINOR"], "2")
	assert.Equal(t, env["CARGO_PKG_VERSION_PATCH"], "0")
	assert.Equal(t, env["CARGO_PKG_VERSION_PRE"], "")
	assert.Equal(t, env["CARGO_PKG_AUTHORS"], "David Peter <mail@david-peter.de>")
}

func TestPkgEnv_VersionPrerelease(t *testing.T) {
	env := PkgEnv(PkgMetadata{Version: "1.0.0-rc.2"})
	assert.Equal(t, env["CARGO_PKG_VERSION_MAJOR"], "1")
	assert.Equal(t, env["CARGO_PKG_VERSION_MINOR"], "0")
	assert.Equal(t, env["CARGO_PKG_VERSION_PATCH"], "0")
	assert.Equal(t, env["CARGO_PKG_VERSION_PRE"], "rc.2")
}

func TestPkgEnv_VersionBuildMetadataStripped(t *testing.T) {
	env := PkgEnv(PkgMetadata{Version: "1.0.0+meta.20240101"})
	assert.Equal(t, env["CARGO_PKG_VERSION_MAJOR"], "1")
	assert.Equal(t, env["CARGO_PKG_VERSION_PATCH"], "0")
	assert.Equal(t, env["CARGO_PKG_VERSION_PRE"], "")
}

func TestPkgEnv_MultipleAuthors(t *testing.T) {
	// Cargo joins multiple authors with `:`.
	env := PkgEnv(PkgMetadata{
		Authors: []string{"Alice", "Bob"},
	})
	assert.Equal(t, env["CARGO_PKG_AUTHORS"], "Alice:Bob")
}

func TestFeatureEnv_NameMangling(t *testing.T) {
	env := FeatureEnv([]string{"default", "json-output", "ssl"})
	assert.Equal(t, env["CARGO_FEATURE_DEFAULT"], "1")
	assert.Equal(t, env["CARGO_FEATURE_JSON_OUTPUT"], "1")
	assert.Equal(t, env["CARGO_FEATURE_SSL"], "1")
	assert.Equal(t, len(env), 3)
}

func TestFeatureEnv_Empty(t *testing.T) {
	env := FeatureEnv(nil)
	assert.Equal(t, len(env), 0)
}

func TestMergeEnv_LaterWins(t *testing.T) {
	got := MergeEnv(
		map[string]string{"A": "1", "B": "2"},
		map[string]string{"B": "3", "C": "4"},
	)
	assert.Equal(t, got["A"], "1")
	assert.Equal(t, got["B"], "3")
	assert.Equal(t, got["C"], "4")
}

func TestSortedKeys(t *testing.T) {
	got := SortedKeys(map[string]string{"b": "", "a": "", "c": ""})
	assert.EqualArrays(t, got, []string{"a", "b", "c"})
}
