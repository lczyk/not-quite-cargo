package unitgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Cargo.toml")
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadManifest_Full(t *testing.T) {
	path := writeManifest(t, `
[package]
name = "fd-find"
version = "10.2.0"
authors = ["David Peter <mail@david-peter.de>"]
description = "A simple, fast and user-friendly alternative to find"
homepage = "https://github.com/sharkdp/fd"
license = "MIT OR Apache-2.0"
repository = "https://github.com/sharkdp/fd"
rust-version = "1.77.2"
readme = "README.md"
`)
	got, err := LoadManifest(path)
	assert.NoError(t, err)
	assert.Equal(t, got.Name, "fd-find")
	assert.Equal(t, got.Version, "10.2.0")
	assert.Equal(t, got.License, "MIT OR Apache-2.0")
	assert.Equal(t, got.RustVersion, "1.77.2")
	assert.Equal(t, got.Readme, "README.md")
	assert.EqualArrays(t, got.Authors, []string{"David Peter <mail@david-peter.de>"})
}

func TestLoadManifest_Minimal(t *testing.T) {
	path := writeManifest(t, `
[package]
name = "x"
version = "0.1.0"
`)
	got, err := LoadManifest(path)
	assert.NoError(t, err)
	assert.Equal(t, got.Name, "x")
	assert.Equal(t, got.Version, "0.1.0")
	assert.Equal(t, got.License, "")
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest(filepath.Join(t.TempDir(), "nope.toml"))
	assert.Error(t, err, assert.AnyError)
}

func TestLoadManifest_WorkspaceOnlyRejected(t *testing.T) {
	// Workspace root without [package] -- caller has to handle this.
	path := writeManifest(t, `
[workspace]
members = ["a", "b"]
`)
	_, err := LoadManifest(path)
	assert.Error(t, err, "no [package] table")
}

func TestLoadManifest_BadTOML(t *testing.T) {
	path := writeManifest(t, `not = "valid toml`)
	_, err := LoadManifest(path)
	assert.Error(t, err, "parse")
}

func TestLoadManifestForPkg_Path(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "Cargo.toml")
	assert.NoError(t, os.WriteFile(manifest, []byte(`
[package]
name = "p"
version = "0.1.0"
`), 0o644))
	id, _ := ParsePkgID("path+file://" + dir + "#0.1.0")
	got, err := LoadManifestForPkg(id, "/cargo", "")
	assert.NoError(t, err)
	assert.Equal(t, got.Name, "p")
}
