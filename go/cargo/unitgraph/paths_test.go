package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

func TestOutputPathsFor_Lib(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		CrateName:   "foo",
		Hash:        "abcdef0123456789",
		TargetKinds: []string{"lib"},
	})
	assert.Equal(t, got.Primary, "/proj/target/debug/deps/libfoo-abcdef0123456789.rlib")
	assert.Equal(t, got.DepInfo, "/proj/target/debug/deps/foo-abcdef0123456789.d")
	assert.Len(t, got.Extras, 0)
}

func TestOutputPathsFor_Bin(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "release",
		CrateName:   "mybin",
		Hash:        "deadbeefcafe0001",
		TargetKinds: []string{"bin"},
	})
	assert.Equal(t, got.Primary, "/proj/target/release/deps/mybin-deadbeefcafe0001")
}

func TestOutputPathsFor_BinWindows(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		Platform:    "x86_64-pc-windows-msvc",
		CrateName:   "mybin",
		Hash:        "0123456789abcdef",
		TargetKinds: []string{"bin"},
	})
	assert.ContainsString(t, got.Primary, "/proj/target/x86_64-pc-windows-msvc/debug/deps/mybin-0123456789abcdef.exe")
}

func TestOutputPathsFor_ProcMacroLinux(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		CrateName:   "serde_derive",
		Hash:        "1111222233334444",
		TargetKinds: []string{"proc-macro"},
	})
	assert.Equal(t, got.Primary, "/proj/target/debug/deps/libserde_derive-1111222233334444.so")
}

func TestOutputPathsFor_ProcMacroDarwin(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		Platform:    "aarch64-apple-darwin",
		CrateName:   "serde_derive",
		Hash:        "1111222233334444",
		TargetKinds: []string{"proc-macro"},
	})
	assert.Equal(t, got.Primary, "/proj/target/aarch64-apple-darwin/debug/deps/libserde_derive-1111222233334444.dylib")
}

func TestOutputPathsFor_Cdylib(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		CrateName:   "ffi",
		Hash:        "aaaaBBBBccccDDDD",
		TargetKinds: []string{"lib", "cdylib"},
	})
	// Primary chosen from the first known kind (lib).
	assert.Equal(t, got.Primary, "/proj/target/debug/deps/libffi-aaaaBBBBccccDDDD.rlib")
	assert.Len(t, got.Extras, 1)
	assert.Equal(t, got.Extras[0], "/proj/target/debug/deps/libffi-aaaaBBBBccccDDDD.so")
}

func TestOutputPathsFor_CustomBuild(t *testing.T) {
	// The caller passes the canonicalised crate name (underscores, no
	// hyphens) because that's what rustc embeds in the output filename.
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		CrateName:   "build_script_build",
		Hash:        "feedfacecafebabe",
		TargetKinds: []string{"custom-build"},
	})
	assert.Equal(t, got.Primary, "/proj/target/debug/deps/build_script_build-feedfacecafebabe")
}

func TestOutputPathsFor_StaticlibAddsExtra(t *testing.T) {
	got := OutputPathsFor(PathInputs{
		ProjectRoot: "/proj",
		ProfileDir:  "debug",
		CrateName:   "ffi",
		Hash:        "0000111122223333",
		TargetKinds: []string{"lib", "staticlib"},
	})
	assert.Len(t, got.Extras, 1)
	assert.Equal(t, got.Extras[0], "/proj/target/debug/deps/libffi-0000111122223333.a")
}
