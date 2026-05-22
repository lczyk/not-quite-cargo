package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

func TestCfgFromTriple_Linux(t *testing.T) {
	cfg := CfgFromTriple("x86_64-unknown-linux-gnu")
	assert.EqualArrays(t, cfg["target_arch"], []string{"x86_64"})
	assert.EqualArrays(t, cfg["target_os"], []string{"linux"})
	assert.EqualArrays(t, cfg["target_family"], []string{"unix"})
	assert.EqualArrays(t, cfg["target_env"], []string{"gnu"})
	assert.EqualArrays(t, cfg["target_pointer_width"], []string{"64"})
	assert.EqualArrays(t, cfg["target_endian"], []string{"little"})
	assert.EqualArrays(t, cfg["unix"], []string{""})
	// no windows cfg for linux
	_, hasWindows := cfg["windows"]
	assert.That(t, !hasWindows)
}

func TestCfgFromTriple_Darwin(t *testing.T) {
	cfg := CfgFromTriple("aarch64-apple-darwin")
	assert.EqualArrays(t, cfg["target_os"], []string{"macos"})
	assert.EqualArrays(t, cfg["target_family"], []string{"unix"})
	assert.EqualArrays(t, cfg["target_arch"], []string{"aarch64"})
	assert.EqualArrays(t, cfg["unix"], []string{""})
}

func TestCfgFromTriple_WindowsMSVC(t *testing.T) {
	cfg := CfgFromTriple("x86_64-pc-windows-msvc")
	assert.EqualArrays(t, cfg["target_os"], []string{"windows"})
	assert.EqualArrays(t, cfg["target_family"], []string{"windows"})
	assert.EqualArrays(t, cfg["target_env"], []string{"msvc"})
	assert.EqualArrays(t, cfg["windows"], []string{""})
	_, hasUnix := cfg["unix"]
	assert.That(t, !hasUnix)
}

func TestCfgFromTriple_Musl(t *testing.T) {
	cfg := CfgFromTriple("x86_64-unknown-linux-musl")
	assert.EqualArrays(t, cfg["target_env"], []string{"musl"})
	assert.EqualArrays(t, cfg["target_family"], []string{"unix"})
}

func TestCfgFromTriple_Unknown(t *testing.T) {
	// Best-effort: arch comes through, OS/family stay empty.
	cfg := CfgFromTriple("foo-bar-baz")
	assert.EqualArrays(t, cfg["target_arch"], []string{"foo"})
	_, hasOS := cfg["target_os"]
	assert.That(t, !hasOS)
}
