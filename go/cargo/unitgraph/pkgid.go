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
// cargo's pkg_id format evolved a few times. Two shapes are recognised:
//
//  1. Modern (cargo ~1.78+):
//     <source-kind>+<source-url>#<name>@<version>
//     `<name>@` is omitted when name == basename(source url).
//     Variant with `#<version>` (no name@) also exists -- treated like (1).
//
//  2. Legacy (cargo through ~1.77, still emitted by --build-plan and
//     --unit-graph in cargo 1.84.x):
//     <name> <version> (<source-kind>+<source-url>)
//
// The capture image (rust:1.84) emits the legacy form; ParsePkgID
// auto-detects by the leading char (scheme prefix vs identifier).
type PkgID struct {
	Raw       string
	Kind      PkgIDKind
	SourceURL string // "file:///abs/path", "https://github.com/rust-lang/crates.io-index", ...
	Name      string
	Version   string
}

// ParsePkgID decodes a cargo pkg_id string in either the modern or
// legacy shape (see PkgID doc).
func ParsePkgID(raw string) (PkgID, error) {
	if looksLegacy(raw) {
		return parseLegacyPkgID(raw)
	}
	return parseModernPkgID(raw)
}

// looksLegacy returns true for the `<name> <version> (<source>)` shape.
// Modern pkg_ids start with `path+`, `registry+` or `git+`; legacy ones
// start with the crate name.
func looksLegacy(raw string) bool {
	for _, prefix := range []string{"path+", "registry+", "git+"} {
		if strings.HasPrefix(raw, prefix) {
			return false
		}
	}
	return strings.Contains(raw, " (")
}

func parseModernPkgID(raw string) (PkgID, error) {
	prefix, rest, ok := strings.Cut(raw, "+")
	if !ok {
		return PkgID{}, fmt.Errorf("pkg_id %q: missing source-kind prefix", raw)
	}

	kind, err := kindFromPrefix(prefix, raw)
	if err != nil {
		return PkgID{}, err
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
		if u, err := url.Parse(source); err == nil && u.Path != "" {
			id.Name = filepath.Base(u.Path)
		}
	}
	return id, nil
}

// parseLegacyPkgID handles `<name> <version> (<source-kind>+<source-url>)`.
func parseLegacyPkgID(raw string) (PkgID, error) {
	// Walk to the opening paren of the source block; everything before
	// it is `<name> <version>` (space-separated).
	parenIdx := strings.Index(raw, " (")
	if parenIdx < 0 || !strings.HasSuffix(raw, ")") {
		return PkgID{}, fmt.Errorf("pkg_id %q: legacy form missing ' (...)' source", raw)
	}
	head := raw[:parenIdx]
	source := raw[parenIdx+2 : len(raw)-1] // strip " (" and trailing ")"

	parts := strings.SplitN(head, " ", 2)
	if len(parts) != 2 {
		return PkgID{}, fmt.Errorf("pkg_id %q: legacy head should be '<name> <version>'", raw)
	}

	prefix, sourceURL, ok := strings.Cut(source, "+")
	if !ok {
		return PkgID{}, fmt.Errorf("pkg_id %q: legacy source missing kind prefix", raw)
	}
	kind, err := kindFromPrefix(prefix, raw)
	if err != nil {
		return PkgID{}, err
	}

	// Legacy registry / git sources sometimes include a `#<commit-sha>`
	// trailer on the URL (git only); strip for the SourceURL field but
	// keep it in Raw.
	if i := strings.Index(sourceURL, "#"); i >= 0 {
		sourceURL = sourceURL[:i]
	}

	return PkgID{
		Raw:       raw,
		Kind:      kind,
		SourceURL: sourceURL,
		Name:      parts[0],
		Version:   parts[1],
	}, nil
}

func kindFromPrefix(prefix, raw string) (PkgIDKind, error) {
	switch prefix {
	case "path":
		return PkgIDPath, nil
	case "registry":
		return PkgIDRegistry, nil
	case "git":
		return PkgIDGit, nil
	default:
		return 0, fmt.Errorf("pkg_id %q: unknown source kind %q", raw, prefix)
	}
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
