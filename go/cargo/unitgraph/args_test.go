package unitgraph

import (
	"strings"
	"testing"

	"github.com/lczyk/assert"
)

func sampleUnit() *Unit {
	cu := 16
	return &Unit{
		PkgID: "path+file:///proj/foo#0.1.0",
		Target: UnitTarget{
			Kind:       []string{"lib"},
			CrateTypes: []string{"lib"},
			Name:       "foo",
			SrcPath:    "/proj/foo/src/lib.rs",
			Edition:    "2021",
		},
		Profile: UnitProfile{
			Name:            "dev",
			OptLevel:        "0",
			LTO:             "false",
			Debuginfo:       float64(2),
			DebugAssertions: true,
			OverflowChecks:  true,
			Incremental:     true,
			Panic:           "unwind",
			CodegenUnits:    &cu,
		},
		Mode:     "build",
		Features: []string{"default"},
	}
}

func argsContain(t *testing.T, args []string, substr string) {
	t.Helper()
	for _, a := range args {
		if strings.Contains(a, substr) {
			return
		}
	}
	t.Errorf("expected an arg containing %q, got: %v", substr, args)
}

func TestRustcArgs_Basic(t *testing.T) {
	args, err := RustcArgs(ArgsInputs{
		Unit:    sampleUnit(),
		Hash:    "0123456789abcdef",
		DepsDir: "/proj/target/debug/deps",
	})
	assert.NoError(t, err)

	// Crate name canonicalised (no hyphens in this case but verify).
	idx := indexOf(args, "--crate-name")
	assert.That(t, idx >= 0, "no --crate-name in %v", args)
	assert.Equal(t, args[idx+1], "foo")

	// Edition + source path.
	argsContain(t, args, "--edition=2021")
	argsContain(t, args, "/proj/foo/src/lib.rs")

	// Crate types.
	argsContain(t, args, "lib")

	// Metadata + extra-filename hashes.
	argsContain(t, args, "-C metadata=0123456789abcdef")
	argsContain(t, args, "-C extra-filename=-0123456789abcdef")

	// Output dir + dependency search dir.
	argsContain(t, args, "/proj/target/debug/deps")
}

func TestRustcArgs_HyphenCrateNameToUnderscore(t *testing.T) {
	u := sampleUnit()
	u.Target.Name = "fd-find"
	args, err := RustcArgs(ArgsInputs{Unit: u, Hash: "h", DepsDir: "/p"})
	assert.NoError(t, err)
	idx := indexOf(args, "--crate-name")
	assert.Equal(t, args[idx+1], "fd_find")
}

func TestRustcArgs_ExternsSortedDeterministic(t *testing.T) {
	args, err := RustcArgs(ArgsInputs{
		Unit:    sampleUnit(),
		Hash:    "h",
		DepsDir: "/p",
		Externs: []ExternRef{
			{Name: "zlib", Path: "/p/libzlib.rlib"},
			{Name: "alpha", Path: "/p/libalpha.rlib"},
			{Name: "middle", Path: "/p/libmiddle.rlib"},
		},
	})
	assert.NoError(t, err)
	// Find the order of --extern occurrences.
	order := []string{}
	for i, a := range args {
		if a == "--extern" && i+1 < len(args) {
			order = append(order, args[i+1])
		}
	}
	assert.EqualArrays(t, order, []string{"alpha=/p/libalpha.rlib", "middle=/p/libmiddle.rlib", "zlib=/p/libzlib.rlib"})
}

func TestRustcArgs_ProfileReleaseOptLevel(t *testing.T) {
	u := sampleUnit()
	u.Profile.OptLevel = "3"
	u.Profile.DebugAssertions = false
	u.Profile.OverflowChecks = false
	u.Profile.Incremental = false
	args, err := RustcArgs(ArgsInputs{Unit: u, Hash: "h", DepsDir: "/p"})
	assert.NoError(t, err)
	argsContain(t, args, "-C opt-level=3")
	argsContain(t, args, "-C debug-assertions=off")
	argsContain(t, args, "-C overflow-checks=off")
	// no incremental flag
	for _, a := range args {
		assert.That(t, a != "-C incremental", "release profile should not emit -C incremental")
	}
}

func TestRustcArgs_DebuginfoString(t *testing.T) {
	u := sampleUnit()
	u.Profile.Debuginfo = "line-tables-only"
	args, err := RustcArgs(ArgsInputs{Unit: u, Hash: "h", DepsDir: "/p"})
	assert.NoError(t, err)
	argsContain(t, args, "-C debuginfo=line-tables-only")
}

func TestRustcArgs_LTOEnabled(t *testing.T) {
	u := sampleUnit()
	u.Profile.LTO = "fat"
	args, err := RustcArgs(ArgsInputs{Unit: u, Hash: "h", DepsDir: "/p"})
	assert.NoError(t, err)
	argsContain(t, args, "-C lto=fat")
}

func TestRustcArgs_NilUnit(t *testing.T) {
	_, err := RustcArgs(ArgsInputs{})
	assert.Error(t, err, "nil unit")
}

// indexOf returns the position of v in args, or -1 if absent.
func indexOf(args []string, v string) int {
	for i, a := range args {
		if a == v {
			return i
		}
	}
	return -1
}
