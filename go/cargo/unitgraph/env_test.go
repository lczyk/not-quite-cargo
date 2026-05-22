package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

func TestTarget_TripleLinuxGnu(t *testing.T) {
	tt := Target{OS: "linux", Arch: "aarch64", Libc: "gnu"}
	assert.Equal(t, tt.Triple(), "aarch64-unknown-linux-gnu")
}

func TestTarget_TripleLinuxMusl(t *testing.T) {
	tt := Target{OS: "linux", Arch: "aarch64", Libc: "musl"}
	assert.Equal(t, tt.Triple(), "aarch64-unknown-linux-musl")
}

func TestTarget_TripleMacos(t *testing.T) {
	tt := Target{OS: "macos", Arch: "aarch64", Libc: "none"}
	assert.Equal(t, tt.Triple(), "aarch64-apple-darwin")
}

func TestTarget_ValidateAccepts(t *testing.T) {
	for _, ok := range []Target{
		{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		{OS: "linux", Arch: "aarch64", Libc: "musl"},
		{OS: "macos", Arch: "aarch64", Libc: "none"},
	} {
		assert.NoError(t, ok.Validate(), "%+v", ok)
	}
}

func TestTarget_ValidateRejects(t *testing.T) {
	cases := []Target{
		{OS: "windows", Arch: "aarch64", Libc: "gnu"},
		{OS: "linux", Arch: "x86_64", Libc: "gnu"},
		{OS: "linux", Arch: "aarch64", Libc: "msvc"},
		{OS: "macos", Arch: "aarch64", Libc: "gnu"},
		{OS: "", Arch: "aarch64", Libc: "gnu"},
	}
	for _, bad := range cases {
		assert.Error(t, bad.Validate(), assert.AnyError, "should reject %+v", bad)
	}
}

func TestTarget_CargoCfgEnvLinuxGnu(t *testing.T) {
	tt := Target{OS: "linux", Arch: "aarch64", Libc: "gnu"}
	env := tt.CargoCfgEnv()
	assert.Equal(t, env["CARGO_CFG_TARGET_OS"], "linux")
	assert.Equal(t, env["CARGO_CFG_TARGET_ARCH"], "aarch64")
	assert.Equal(t, env["CARGO_CFG_TARGET_FAMILY"], "unix")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENV"], "gnu")
	assert.Equal(t, env["CARGO_CFG_TARGET_POINTER_WIDTH"], "64")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENDIAN"], "little")
	assert.Equal(t, env["CARGO_CFG_TARGET_VENDOR"], "unknown")
	_, hasUnix := env["CARGO_CFG_UNIX"]
	assert.That(t, hasUnix)
}

func TestTarget_CargoCfgEnvMacosNoneEmptiesEnv(t *testing.T) {
	tt := Target{OS: "macos", Arch: "aarch64", Libc: "none"}
	env := tt.CargoCfgEnv()
	assert.Equal(t, env["CARGO_CFG_TARGET_OS"], "macos")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENV"], "")
	assert.Equal(t, env["CARGO_CFG_TARGET_VENDOR"], "apple")
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
