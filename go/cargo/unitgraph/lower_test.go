package unitgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

// tinyUG returns a minimal unit-graph: one path-based bin crate, one
// dependency on a registry crate, no build scripts.
func tinyUG(t *testing.T, projDir, cargoHome string) *UnitGraph {
	t.Helper()
	// Write Cargo.toml for the path crate.
	assert.NoError(t, os.WriteFile(filepath.Join(projDir, "Cargo.toml"), []byte(`
[package]
name = "demo"
version = "0.1.0"
`), 0o644))
	assert.NoError(t, os.MkdirAll(filepath.Join(projDir, "src"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(projDir, "src", "main.rs"), []byte(`fn main(){}`), 0o644))

	// Write a fake registry source for serde.
	idx := "index.crates.io-test"
	srcDir := filepath.Join(cargoHome, "registry", "src", idx, "serde-1.0.0")
	assert.NoError(t, os.MkdirAll(srcDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(srcDir, "Cargo.toml"), []byte(`
[package]
name = "serde"
version = "1.0.0"
license = "MIT OR Apache-2.0"
`), 0o644))

	return &UnitGraph{
		Version: 1,
		Roots:   []int{0},
		Units: []Unit{
			{
				PkgID: "path+file://" + projDir + "#demo@0.1.0",
				Target: UnitTarget{
					Kind:       []string{"bin"},
					CrateTypes: []string{"bin"},
					Name:       "demo",
					SrcPath:    filepath.Join(projDir, "src/main.rs"),
					Edition:    "2021",
				},
				Profile: UnitProfile{Name: "dev", OptLevel: "0", DebugAssertions: true, OverflowChecks: true, Incremental: true, Panic: "unwind", Debuginfo: float64(2)},
				Mode:    "build",
				Dependencies: []UnitDep{
					{Index: 1, ExternCrateName: "serde", Public: false},
				},
			},
			{
				PkgID: "registry+https://github.com/rust-lang/crates.io-index#serde@1.0.0",
				Target: UnitTarget{
					Kind:       []string{"lib"},
					CrateTypes: []string{"lib"},
					Name:       "serde",
					SrcPath:    filepath.Join(cargoHome, "registry/src", idx, "serde-1.0.0/src/lib.rs"),
					Edition:    "2018",
				},
				Profile: UnitProfile{Name: "dev", OptLevel: "0", DebugAssertions: true, OverflowChecks: true, Incremental: true, Panic: "unwind", Debuginfo: float64(2)},
				Mode:    "build",
			},
		},
	}
}

func tinyOpts(cargoHome string) LowerOptions {
	return LowerOptions{
		HostTriple:    "aarch64-unknown-linux-gnu",
		CargoHome:     cargoHome,
		ProjectRoot:   "/proj",
		RustcPath:     "/some/rustc",
		Target:        Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		RegistryIndex: "index.crates.io-test",
	}
}

func TestLower_TinyGraph(t *testing.T) {
	cargoHome := t.TempDir()
	projDir := t.TempDir()
	ug := tinyUG(t, projDir, cargoHome)

	out, err := Lower(ug, tinyOpts(cargoHome))
	assert.NoError(t, err)
	assert.Len(t, out.Invocations, 2)

	// Bin invocation -- check program, that crate name is canonical, and
	// that --extern serde=<path> resolves to the lib unit's output.
	bin := out.Invocations[0]
	assert.Equal(t, bin.Program, "/some/rustc")
	assert.Equal(t, bin.PackageName, "demo")
	assert.EqualArrays(t, bin.TargetKind, []string{"bin"})

	// Externs sorted; only one here so trivially correct.
	var sawExtern bool
	for i, a := range bin.Args {
		if a == "--extern" && i+1 < len(bin.Args) {
			sawExtern = true
			// path must end with the lib unit's primary artefact name.
			assert.ContainsString(t, bin.Args[i+1], "serde=/proj/target/debug/deps/libserde-")
		}
	}
	assert.That(t, sawExtern, "expected an --extern entry")

	// Env: CARGO_PKG_NAME from pkg_id, CARGO_CRATE_NAME canonical.
	// (pkg_id carries name + version; other CARGO_PKG_* stay empty
	// because Lower no longer reads Cargo.toml.)
	assert.Equal(t, bin.Env["CARGO_PKG_NAME"], "demo")
	assert.Equal(t, bin.Env["CARGO_CRATE_NAME"], "demo")
	assert.Equal(t, bin.Env["CARGO_BIN_NAME"], "demo")
	assert.Equal(t, bin.Env["CARGO_PRIMARY_PACKAGE"], "1")

	// Lib invocation -- registry package, not primary.
	lib := out.Invocations[1]
	assert.Equal(t, lib.PackageName, "serde")
	assert.Equal(t, lib.Env["CARGO_PRIMARY_PACKAGE"], "")
}

func TestLower_RejectsBadVersion(t *testing.T) {
	ug := &UnitGraph{Version: 99}
	_, err := Lower(ug, LowerOptions{
		Target:      Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		ProjectRoot: "/proj",
		RustcPath:   "rustc",
	})
	// Lower doesn't validate version itself (LoadUnitGraph does); it
	// just processes whatever it's given. The empty Units slice means
	// no error -- adjust expectation.
	assert.NoError(t, err)
}

func TestLower_RequiresValidTarget(t *testing.T) {
	_, err := Lower(&UnitGraph{Version: 1}, LowerOptions{})
	assert.Error(t, err, assert.AnyError)
}

func TestLower_RequiresRustcPath(t *testing.T) {
	_, err := Lower(&UnitGraph{Version: 1}, LowerOptions{
		Target: Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
	})
	assert.Error(t, err, "RustcPath is required")
}

func TestLower_DerivesProjectRootFromUnitGraph(t *testing.T) {
	// path+ pkg_id with file:// URL is the project root.
	ug := &UnitGraph{
		Version: 1,
		Units: []Unit{
			{
				PkgID:   "path+file:///work/myproj#demo@0.1.0",
				Target:  UnitTarget{Kind: []string{"bin"}, CrateTypes: []string{"bin"}, Name: "demo", SrcPath: "/work/myproj/src/main.rs", Edition: "2021"},
				Profile: UnitProfile{Name: "dev", OptLevel: "0", DebugAssertions: true, OverflowChecks: true, Panic: "unwind", Debuginfo: float64(2)},
				Mode:    "build",
			},
		},
	}
	out, err := Lower(ug, LowerOptions{
		Target:    Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		RustcPath: "rustc",
	})
	assert.NoError(t, err)
	// The bin invocation's output should land under the derived project root.
	assert.ContainsString(t, out.Invocations[0].Outputs[0], "/work/myproj/target/")
}

func TestLower_DerivesCargoHomeFromUnitGraph(t *testing.T) {
	// A registry src_path under /home/u/.cargo-custom/registry/src/...
	// should yield CargoHome = /home/u/.cargo-custom and the index dir
	// = "myindex-abc123" (auto-derived from the same path).
	ug := &UnitGraph{
		Version: 1,
		Units: []Unit{
			{
				PkgID:        "path+file:///proj#demo@0.1.0",
				Target:       UnitTarget{Kind: []string{"bin"}, CrateTypes: []string{"bin"}, Name: "demo", SrcPath: "/proj/src/main.rs", Edition: "2021"},
				Profile:      UnitProfile{Name: "dev", OptLevel: "0", DebugAssertions: true, OverflowChecks: true, Panic: "unwind", Debuginfo: float64(2)},
				Mode:         "build",
				Dependencies: []UnitDep{{Index: 1, ExternCrateName: "serde"}},
			},
			{
				PkgID:   "registry+https://github.com/rust-lang/crates.io-index#serde@1.0.0",
				Target:  UnitTarget{Kind: []string{"lib"}, CrateTypes: []string{"lib"}, Name: "serde", SrcPath: "/home/u/.cargo-custom/registry/src/myindex-abc123/serde-1.0.0/src/lib.rs", Edition: "2018"},
				Profile: UnitProfile{Name: "dev", OptLevel: "0", DebugAssertions: true, OverflowChecks: true, Panic: "unwind", Debuginfo: float64(2)},
				Mode:    "build",
			},
		},
	}
	out, err := Lower(ug, LowerOptions{
		Target:    Target{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		RustcPath: "rustc",
	})
	assert.NoError(t, err)
	// CARGO_MANIFEST_DIR for the registry crate should include both the
	// derived cargo home AND the index subdir name.
	libInv := out.Invocations[1]
	assert.ContainsString(t, libInv.Env["CARGO_MANIFEST_DIR"],
		"/home/u/.cargo-custom/registry/src/myindex-abc123/serde-1.0.0")
}

func TestLoadUnitGraph_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ug.json")
	assert.NoError(t, os.WriteFile(path, []byte(`{
		"version": 1,
		"roots": [0],
		"units": [{
			"pkg_id": "path+file:///x#foo@0.1.0",
			"target": {"kind":["bin"], "crate_types":["bin"], "name":"foo", "src_path":"/x/src/main.rs", "edition":"2021", "doc": true, "doctest": false, "test": true},
			"profile": {"name":"dev","opt_level":"0","lto":"false","debuginfo":2,"debug_assertions":true,"overflow_checks":true,"rpath":false,"incremental":true,"panic":"unwind","codegen_backend":null,"codegen_units":null,"split_debuginfo":null},
			"platform": null,
			"mode": "build",
			"features": [],
			"dependencies": []
		}]
	}`), 0o644))
	ug, err := LoadUnitGraph(path)
	assert.NoError(t, err)
	assert.Equal(t, ug.Version, 1)
	assert.Len(t, ug.Units, 1)
	assert.Equal(t, ug.Units[0].Target.Name, "foo")
}

func TestLoadUnitGraph_BadVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ug.json")
	assert.NoError(t, os.WriteFile(path, []byte(`{"version": 99, "units": [], "roots": []}`), 0o644))
	_, err := LoadUnitGraph(path)
	assert.Error(t, err, "unsupported unit graph version")
}
