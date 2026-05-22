package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

func TestTarget_TripleLinux(t *testing.T) {
	tt := Target{OS: "linux", Arch: "x86_64"}
	assert.Equal(t, tt.Triple(), "x86_64-unknown-linux-gnu")
}

func TestTarget_TripleLinuxMusl(t *testing.T) {
	tt := Target{OS: "linux", Arch: "aarch64", Env: "musl"}
	assert.Equal(t, tt.Triple(), "aarch64-unknown-linux-musl")
}

func TestTarget_TripleDarwin(t *testing.T) {
	tt := Target{OS: "macos", Arch: "aarch64"}
	assert.Equal(t, tt.Triple(), "aarch64-apple-macos")
}

func TestTarget_TripleWindows(t *testing.T) {
	tt := Target{OS: "windows", Arch: "x86_64"}
	assert.Equal(t, tt.Triple(), "x86_64-pc-windows-msvc")
}

func TestTarget_Family(t *testing.T) {
	cases := []struct {
		os   string
		want string
	}{
		{"linux", "unix"},
		{"macos", "unix"},
		{"freebsd", "unix"},
		{"windows", "windows"},
		{"unknown-os", ""},
	}
	for _, c := range cases {
		assert.Equal(t, (Target{OS: c.os}).Family(), c.want, "os=%s", c.os)
	}
}

func TestTarget_PointerWidth(t *testing.T) {
	cases := []struct {
		arch string
		want string
	}{
		{"x86_64", "64"},
		{"aarch64", "64"},
		{"i686", "32"},
		{"wasm32", "32"},
		{"weirdarch", ""},
	}
	for _, c := range cases {
		assert.Equal(t, (Target{Arch: c.arch}).PointerWidth(), c.want, "arch=%s", c.arch)
	}
}

func TestTarget_Endian(t *testing.T) {
	assert.Equal(t, (Target{Arch: "x86_64"}).Endian(), "little")
	assert.Equal(t, (Target{Arch: "powerpc"}).Endian(), "big")
}

func TestTarget_Vendor(t *testing.T) {
	assert.Equal(t, (Target{OS: "macos"}).Vendor(), "apple")
	assert.Equal(t, (Target{OS: "windows"}).Vendor(), "pc")
	assert.Equal(t, (Target{OS: "linux"}).Vendor(), "unknown")
}

func TestTarget_CargoCfgEnvLinux(t *testing.T) {
	tt := Target{OS: "linux", Arch: "x86_64"}
	env := tt.CargoCfgEnv()
	assert.Equal(t, env["CARGO_CFG_TARGET_OS"], "linux")
	assert.Equal(t, env["CARGO_CFG_TARGET_ARCH"], "x86_64")
	assert.Equal(t, env["CARGO_CFG_TARGET_FAMILY"], "unix")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENV"], "gnu")
	assert.Equal(t, env["CARGO_CFG_TARGET_POINTER_WIDTH"], "64")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENDIAN"], "little")
	assert.Equal(t, env["CARGO_CFG_TARGET_VENDOR"], "unknown")
	_, hasUnix := env["CARGO_CFG_UNIX"]
	assert.That(t, hasUnix)
	_, hasWindows := env["CARGO_CFG_WINDOWS"]
	assert.That(t, !hasWindows)
}

func TestTarget_CargoCfgEnvWindows(t *testing.T) {
	tt := Target{OS: "windows", Arch: "x86_64"}
	env := tt.CargoCfgEnv()
	assert.Equal(t, env["CARGO_CFG_TARGET_FAMILY"], "windows")
	assert.Equal(t, env["CARGO_CFG_TARGET_ENV"], "msvc")
	_, hasWindows := env["CARGO_CFG_WINDOWS"]
	assert.That(t, hasWindows)
	_, hasUnix := env["CARGO_CFG_UNIX"]
	assert.That(t, !hasUnix)
}

func TestTarget_CargoCfgEnvEnvOverride(t *testing.T) {
	tt := Target{OS: "linux", Arch: "x86_64", Env: "musl"}
	env := tt.CargoCfgEnv()
	assert.Equal(t, env["CARGO_CFG_TARGET_ENV"], "musl")
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
