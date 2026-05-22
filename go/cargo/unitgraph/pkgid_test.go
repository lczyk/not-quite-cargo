package unitgraph

import (
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

func TestParsePkgID_Path(t *testing.T) {
	id, err := ParsePkgID("path+file:///proj/foo#0.1.0")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDPath)
	assert.Equal(t, id.SourceURL, "file:///proj/foo")
	assert.Equal(t, id.Name, "foo")
	assert.Equal(t, id.Version, "0.1.0")
}

func TestParsePkgID_PathWithNameAt(t *testing.T) {
	// Newer cargo shape: explicit name@version after the fragment.
	id, err := ParsePkgID("path+file:///proj/foo#foo-find@0.1.0")
	assert.NoError(t, err)
	assert.Equal(t, id.Name, "foo-find")
	assert.Equal(t, id.Version, "0.1.0")
}

func TestParsePkgID_Registry(t *testing.T) {
	id, err := ParsePkgID("registry+https://github.com/rust-lang/crates.io-index#serde@1.0.215")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDRegistry)
	assert.Equal(t, id.SourceURL, "https://github.com/rust-lang/crates.io-index")
	assert.Equal(t, id.Name, "serde")
	assert.Equal(t, id.Version, "1.0.215")
}

func TestParsePkgID_Git(t *testing.T) {
	id, err := ParsePkgID("git+https://github.com/example/repo?rev=abc123#mycrate@0.2.0")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDGit)
	assert.Equal(t, id.Name, "mycrate")
	assert.Equal(t, id.Version, "0.2.0")
}

func TestParsePkgID_RejectsMissingPrefix(t *testing.T) {
	_, err := ParsePkgID("file:///proj/foo#0.1.0")
	assert.Error(t, err, "source-kind prefix")
}

func TestParsePkgID_RejectsUnknownKind(t *testing.T) {
	_, err := ParsePkgID("svn+http://x#name@1.0")
	assert.Error(t, err, "unknown source kind")
}

func TestParsePkgID_RejectsMissingFragment(t *testing.T) {
	_, err := ParsePkgID("path+file:///proj/foo")
	assert.Error(t, err, "missing #fragment")
}

func TestParsePkgID_LegacyRegistry(t *testing.T) {
	// cargo 1.84 emits the older `<name> <version> (<source>)` shape
	// from --unit-graph / --build-plan. Must parse equivalently.
	id, err := ParsePkgID("aho-corasick 1.1.3 (registry+https://github.com/rust-lang/crates.io-index)")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDRegistry)
	assert.Equal(t, id.Name, "aho-corasick")
	assert.Equal(t, id.Version, "1.1.3")
	assert.Equal(t, id.SourceURL, "https://github.com/rust-lang/crates.io-index")
}

func TestParsePkgID_LegacyPath(t *testing.T) {
	id, err := ParsePkgID("fd-find 10.2.0 (path+file:///tmp/fd)")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDPath)
	assert.Equal(t, id.Name, "fd-find")
	assert.Equal(t, id.Version, "10.2.0")
	assert.Equal(t, id.SourceURL, "file:///tmp/fd")
}

func TestParsePkgID_LegacyGitWithCommit(t *testing.T) {
	// Legacy git pkg_ids embed the resolved commit after a # inside
	// the parens; SourceURL strips it.
	id, err := ParsePkgID("foo 0.1.0 (git+https://example.com/repo#abc123)")
	assert.NoError(t, err)
	assert.Equal(t, id.Kind, PkgIDGit)
	assert.Equal(t, id.Name, "foo")
	assert.Equal(t, id.Version, "0.1.0")
	assert.Equal(t, id.SourceURL, "https://example.com/repo")
}

func TestPkgID_ManifestDir_Path(t *testing.T) {
	id, _ := ParsePkgID("path+file:///proj/foo#0.1.0")
	dir, err := id.ManifestDir("/cargo", "")
	assert.NoError(t, err)
	assert.Equal(t, dir, "/proj/foo")
}

func TestPkgID_ManifestDir_Registry(t *testing.T) {
	id, _ := ParsePkgID("registry+https://github.com/rust-lang/crates.io-index#serde@1.0.215")
	dir, err := id.ManifestDir("/cargo", "index.crates.io-1949cf8c6b5b557f")
	assert.NoError(t, err)
	want := filepath.Join("/cargo", "registry", "src", "index.crates.io-1949cf8c6b5b557f", "serde-1.0.215")
	assert.Equal(t, dir, want)
}

func TestPkgID_ManifestDir_RegistryNoIndex(t *testing.T) {
	// With empty indexDir we still produce a non-error path; just
	// without the index segment. Downstream consumers (run.go) only
	// stat outputs/links, not this dir, so missing index data is fine.
	id, _ := ParsePkgID("registry+https://github.com/rust-lang/crates.io-index#serde@1.0.215")
	dir, err := id.ManifestDir("/cargo", "")
	assert.NoError(t, err)
	assert.Equal(t, dir, filepath.Join("/cargo", "registry", "src", "serde-1.0.215"))
}

func TestPkgID_ManifestDir_GitErrors(t *testing.T) {
	id, _ := ParsePkgID("git+https://github.com/example/repo?rev=abc123#mycrate@0.2.0")
	_, err := id.ManifestDir("/cargo", "")
	assert.Error(t, err, "git sources need caller-resolved manifest dir")
}
