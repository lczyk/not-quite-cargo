package unitgraph

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// rawManifest captures the slice of Cargo.toml the lowerer reads.
//
// Each inheritable field accepts either a literal value (`version = "1.0"`)
// or the workspace-inheritance marker (`version.workspace = true`); we
// decode into interface{} and resolve in resolveInherited below.
type rawManifest struct {
	Package *struct {
		Name        any `toml:"name"`
		Version     any `toml:"version"`
		Authors     any `toml:"authors"`
		Description any `toml:"description"`
		Homepage    any `toml:"homepage"`
		License     any `toml:"license"`
		LicenseFile any `toml:"license-file"`
		Repository  any `toml:"repository"`
		RustVersion any `toml:"rust-version"`
		Readme      any `toml:"readme"`
	} `toml:"package"`
}

// rawWorkspace captures the `[workspace.package]` table at a workspace
// root, used to resolve inherited fields like `version.workspace = true`.
type rawWorkspace struct {
	Workspace *struct {
		Package *struct {
			Name        string   `toml:"name"`
			Version     string   `toml:"version"`
			Authors     []string `toml:"authors"`
			Description string   `toml:"description"`
			Homepage    string   `toml:"homepage"`
			License     string   `toml:"license"`
			LicenseFile string   `toml:"license-file"`
			Repository  string   `toml:"repository"`
			RustVersion string   `toml:"rust-version"`
			Readme      string   `toml:"readme"`
		} `toml:"package"`
	} `toml:"workspace"`
}

// LoadManifest reads a Cargo.toml from disk and extracts the fields that
// drive CARGO_PKG_* env vars. Workspace inheritance (`<field>.workspace =
// true`) is resolved by walking up the directory tree from the manifest
// to find a Cargo.toml with a `[workspace]` table.
func LoadManifest(manifestPath string) (PkgMetadata, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return PkgMetadata{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}
	var raw rawManifest
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return PkgMetadata{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	if raw.Package == nil {
		return PkgMetadata{}, fmt.Errorf("%s has no [package] table", manifestPath)
	}

	// Walk up to find a workspace root (best-effort; an absent workspace
	// just means none of the per-package fields can be inherited).
	wsRoot := findWorkspaceRoot(filepath.Dir(manifestPath))

	return resolveInherited(raw.Package, wsRoot)
}

// resolveInherited turns the loosely-typed Package struct into a
// PkgMetadata, looking up `.workspace = true` fields from the workspace
// root's `[workspace.package]` table.
func resolveInherited(pkg any, wsRoot string) (PkgMetadata, error) {
	p, ok := pkg.(*struct {
		Name        any `toml:"name"`
		Version     any `toml:"version"`
		Authors     any `toml:"authors"`
		Description any `toml:"description"`
		Homepage    any `toml:"homepage"`
		License     any `toml:"license"`
		LicenseFile any `toml:"license-file"`
		Repository  any `toml:"repository"`
		RustVersion any `toml:"rust-version"`
		Readme      any `toml:"readme"`
	})
	if !ok {
		return PkgMetadata{}, fmt.Errorf("resolveInherited: unexpected Package shape")
	}

	var ws *struct {
		Name        string
		Version     string
		Authors     []string
		Description string
		Homepage    string
		License     string
		LicenseFile string
		Repository  string
		RustVersion string
		Readme      string
	}
	if wsRoot != "" {
		ws = loadWorkspacePkg(wsRoot)
	}

	str := func(v any, fallback string) string {
		if isWorkspaceInherit(v) {
			return fallback
		}
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	strSlice := func(v any, fallback []string) []string {
		if isWorkspaceInherit(v) {
			return fallback
		}
		if arr, ok := v.([]any); ok {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
		return nil
	}

	var wsName, wsVersion, wsDesc, wsHome, wsLic, wsLicFile, wsRepo, wsRust, wsReadme string
	var wsAuthors []string
	if ws != nil {
		wsName, wsVersion, wsDesc, wsHome, wsLic, wsLicFile, wsRepo, wsRust, wsReadme =
			ws.Name, ws.Version, ws.Description, ws.Homepage, ws.License, ws.LicenseFile, ws.Repository, ws.RustVersion, ws.Readme
		wsAuthors = ws.Authors
	}

	return PkgMetadata{
		Name:        str(p.Name, wsName),
		Version:     str(p.Version, wsVersion),
		Authors:     strSlice(p.Authors, wsAuthors),
		Description: str(p.Description, wsDesc),
		Homepage:    str(p.Homepage, wsHome),
		License:     str(p.License, wsLic),
		LicenseFile: str(p.LicenseFile, wsLicFile),
		Repository:  str(p.Repository, wsRepo),
		RustVersion: str(p.RustVersion, wsRust),
		Readme:      str(p.Readme, wsReadme),
	}, nil
}

// isWorkspaceInherit reports whether a field value is the workspace
// inheritance marker -- a single-key map `{workspace = true}`.
func isWorkspaceInherit(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	w, ok := m["workspace"]
	if !ok {
		return false
	}
	b, ok := w.(bool)
	return ok && b
}

// findWorkspaceRoot walks up from dir looking for a Cargo.toml containing
// a `[workspace]` table. Returns the path to that Cargo.toml or "" when
// none is found before reaching the filesystem root.
func findWorkspaceRoot(dir string) string {
	for {
		candidate := filepath.Join(dir, "Cargo.toml")
		if data, err := os.ReadFile(candidate); err == nil {
			var probe rawWorkspace
			if _, err := toml.Decode(string(data), &probe); err == nil &&
				probe.Workspace != nil {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadWorkspacePkg reads the `[workspace.package]` table from the given
// workspace-root Cargo.toml. Returns nil if the file lacks the table.
func loadWorkspacePkg(wsRoot string) *struct {
	Name        string
	Version     string
	Authors     []string
	Description string
	Homepage    string
	License     string
	LicenseFile string
	Repository  string
	RustVersion string
	Readme      string
} {
	data, err := os.ReadFile(wsRoot)
	if err != nil {
		return nil
	}
	var raw rawWorkspace
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil
	}
	if raw.Workspace == nil || raw.Workspace.Package == nil {
		return nil
	}
	return &struct {
		Name        string
		Version     string
		Authors     []string
		Description string
		Homepage    string
		License     string
		LicenseFile string
		Repository  string
		RustVersion string
		Readme      string
	}{
		Name:        raw.Workspace.Package.Name,
		Version:     raw.Workspace.Package.Version,
		Authors:     raw.Workspace.Package.Authors,
		Description: raw.Workspace.Package.Description,
		Homepage:    raw.Workspace.Package.Homepage,
		License:     raw.Workspace.Package.License,
		LicenseFile: raw.Workspace.Package.LicenseFile,
		Repository:  raw.Workspace.Package.Repository,
		RustVersion: raw.Workspace.Package.RustVersion,
		Readme:      raw.Workspace.Package.Readme,
	}
}

// LoadManifestForPkg locates and loads the Cargo.toml for a given pkg_id.
// indexDir is the registry index cache subdirectory under
// $CARGO_HOME/registry/src/ (e.g. "index.crates.io-1949cf8c6b5b557f").
func LoadManifestForPkg(p PkgID, cargoHome, indexDir string) (PkgMetadata, error) {
	dir, err := p.ManifestDir(cargoHome, indexDir)
	if err != nil {
		return PkgMetadata{}, err
	}
	return LoadManifest(filepath.Join(dir, "Cargo.toml"))
}
