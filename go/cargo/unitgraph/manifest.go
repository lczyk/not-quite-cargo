package unitgraph

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// rawManifest captures the slice of Cargo.toml the lowerer reads.
//
// Workspace manifests can omit the `[package]` table entirely (a pure
// workspace root). Inheritable workspace fields (`package.<field>.workspace
// = true`) are not yet supported here; the loader returns whatever the
// per-package manifest carries verbatim. Resolution of inherited values
// is deferred to the orchestrator once it has both the workspace root
// manifest and the per-package one.
type rawManifest struct {
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
}

// LoadManifest reads a Cargo.toml from disk and extracts the fields that
// drive CARGO_PKG_* env vars. Returns a zero PkgMetadata (with an error)
// if the file lacks a `[package]` table -- callers that hand it a
// workspace-only root must handle that case.
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
	return PkgMetadata{
		Name:        raw.Package.Name,
		Version:     raw.Package.Version,
		Authors:     raw.Package.Authors,
		Description: raw.Package.Description,
		Homepage:    raw.Package.Homepage,
		License:     raw.Package.License,
		LicenseFile: raw.Package.LicenseFile,
		Repository:  raw.Package.Repository,
		RustVersion: raw.Package.RustVersion,
		Readme:      raw.Package.Readme,
	}, nil
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
