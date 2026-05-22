package unitgraph

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// PkgIDKind enumerates the source kinds cargo writes into a pkg_id.
type PkgIDKind int

const (
	PkgIDPath     PkgIDKind = iota // path+file:///abs/path#name@version
	PkgIDRegistry                  // registry+https://.../crates.io-index#name@version
	PkgIDGit                       // git+https://github.com/...#name@version or with ?rev=...
)

// PkgID is the parsed form of a cargo pkg_id string.
//
// cargo's pkg_id format evolved a few times. The current "stable" shape
// (used by --unit-graph, cargo metadata --format-version=1, etc.) is:
//
//	<source-kind>+<source-url>#<name>@<version>
//
// where the `<name>@` prefix is omitted when name == basename(source url).
// Older shapes use `#<version>` (no name@). We accept both.
type PkgID struct {
	Raw       string
	Kind      PkgIDKind
	SourceURL string // "file:///abs/path", "https://github.com/rust-lang/crates.io-index", ...
	Name      string
	Version   string
}

// ParsePkgID decodes a cargo pkg_id string.
func ParsePkgID(raw string) (PkgID, error) {
	prefix, rest, ok := strings.Cut(raw, "+")
	if !ok {
		return PkgID{}, fmt.Errorf("pkg_id %q: missing source-kind prefix", raw)
	}

	var kind PkgIDKind
	switch prefix {
	case "path":
		kind = PkgIDPath
	case "registry":
		kind = PkgIDRegistry
	case "git":
		kind = PkgIDGit
	default:
		return PkgID{}, fmt.Errorf("pkg_id %q: unknown source kind %q", raw, prefix)
	}

	source, frag, ok := strings.Cut(rest, "#")
	if !ok {
		return PkgID{}, fmt.Errorf("pkg_id %q: missing #fragment", raw)
	}

	id := PkgID{Raw: raw, Kind: kind, SourceURL: source}

	// Fragment is `name@version` for the new shape, or just `version` for
	// the older shape (in which case name == basename(source)).
	if name, version, hasAt := strings.Cut(frag, "@"); hasAt {
		id.Name = name
		id.Version = version
	} else {
		id.Version = frag
		// Derive name from source basename.
		if u, err := url.Parse(source); err == nil && u.Path != "" {
			id.Name = filepath.Base(u.Path)
		}
	}
	return id, nil
}

// ManifestDir returns the directory containing the package's Cargo.toml
// on disk, given the resolved CARGO_HOME (used to locate registry / git
// sources).
//
// For path sources the source URL is the manifest dir directly. For
// registry sources we point at cargo's standard layout
// `$CARGO_HOME/registry/src/<index-hash-dir>/<name>-<version>/`. The
// index-hash-dir is cargo's content-addressed cache name; we can't
// derive it from a pkg_id alone, so callers pass `indexDir` -- the
// resolved directory under `$CARGO_HOME/registry/src/`.
//
// Git sources land under `$CARGO_HOME/git/checkouts/...`; we don't
// resolve those here since cargo writes a non-trivial path involving
// the git revision -- callers pass the resolved path themselves.
func (p PkgID) ManifestDir(cargoHome, indexDir string) (string, error) {
	switch p.Kind {
	case PkgIDPath:
		u, err := url.Parse(p.SourceURL)
		if err != nil {
			return "", fmt.Errorf("pkg_id %q: parse path source: %w", p.Raw, err)
		}
		if u.Scheme != "file" {
			return "", fmt.Errorf("pkg_id %q: path source must use file:// scheme, got %q", p.Raw, u.Scheme)
		}
		return u.Path, nil
	case PkgIDRegistry:
		if indexDir == "" {
			return "", fmt.Errorf("pkg_id %q: registry source needs an index dir", p.Raw)
		}
		return filepath.Join(cargoHome, "registry", "src", indexDir, p.Name+"-"+p.Version), nil
	case PkgIDGit:
		return "", fmt.Errorf("pkg_id %q: git sources need caller-resolved manifest dir", p.Raw)
	default:
		return "", fmt.Errorf("pkg_id %q: unknown kind", p.Raw)
	}
}
