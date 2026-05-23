package unitgraph

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// PkgMetadata is the slice of a Cargo.toml that drives CARGO_PKG_* env
// vars. Build now populates only Name + Version (from pkg_id); the rest
// stay empty.
type PkgMetadata struct {
	Name        string
	Version     string // canonical semver, e.g. "1.2.3-rc1"
	Authors     []string
	Description string
	Homepage    string
	License     string
	LicenseFile string
	Repository  string
	RustVersion string
	Readme      string
}

// Target captures the cfg-driving subset of a rust target.
//
// SCOPE: v0 supports {linux, macos} x {aarch64, x86_64}. Both arches
// verified e2e via the sudo + fd example demos (arm64 locally, amd64
// on a linux runner).
//
// Libc carries the rust target env token (gnu / musl) for linux, or
// "none" for macos. cargo emits CARGO_CFG_TARGET_ENV as the empty
// string for "none".
type Target struct {
	OS   string // linux | macos
	Arch string // aarch64 | x86_64
	Libc string // gnu | musl (linux only) | none (macos)
	// VendorOverride lets callers replace the default vendor token in
	// the rust target triple. Defaults are "unknown" for linux (matching
	// rust-lang's official builds) and "apple" for macos; set this to
	// "alpine" (etc.) to target a distro that ships its own triple, e.g.
	// aarch64-alpine-linux-musl.
	VendorOverride string
}

// Validate enforces the supported scope.
func (t Target) Validate() error {
	switch t.OS {
	case "linux":
		switch t.Libc {
		case "gnu", "musl":
		default:
			return fmt.Errorf("target: linux libc must be gnu or musl, got %q", t.Libc)
		}
	case "macos":
		if t.Libc != "none" {
			return fmt.Errorf("target: macos libc must be none, got %q", t.Libc)
		}
	default:
		return fmt.Errorf("target: OS must be linux or macos, got %q", t.OS)
	}
	switch t.Arch {
	case "aarch64", "x86_64":
	default:
		return fmt.Errorf("target: arch must be aarch64 or x86_64, got %q", t.Arch)
	}
	return nil
}

// Triple synthesises the rust target triple.
//
//	linux + gnu  -> aarch64-unknown-linux-gnu
//	linux + musl -> aarch64-unknown-linux-musl
//	macos + none -> aarch64-apple-darwin
func (t Target) Triple() string {
	switch t.OS {
	case "macos":
		return t.Arch + "-apple-darwin"
	case "linux":
		return t.Arch + "-" + t.Vendor() + "-linux-" + t.Libc
	default:
		return ""
	}
}

// Family returns "unix" for the two OSes we support.
func (t Target) Family() string {
	return "unix"
}

// PointerWidth is "64" -- both supported arches (aarch64 + x86_64) are
// 64-bit.
func (t Target) PointerWidth() string {
	return "64"
}

// Endian is "little" -- both supported arches (aarch64 + x86_64) are
// little-endian.
func (t Target) Endian() string {
	return "little"
}

// Vendor returns the rust target-triple vendor token. Honors
// VendorOverride when set; otherwise defaults to "apple" for macos and
// "unknown" for everything else (matches rust-lang's official builds).
func (t Target) Vendor() string {
	if t.VendorOverride != "" {
		return t.VendorOverride
	}
	switch t.OS {
	case "macos":
		return "apple"
	default:
		return "unknown"
	}
}

// CargoCfgEnv emits the CARGO_CFG_* env-var subset that cargo would set
// when invoking rustc or a build script for this target.
//
// Covers TARGET_OS / _ARCH / _FAMILY / _ENV / _POINTER_WIDTH / _ENDIAN /
// _VENDOR plus the bare unix/windows cfg key. less-common cfgs (
// target_feature, target_has_atomic, debug_assertions) are deliberately
// omitted: they need rustc to enumerate them faithfully, and the
// experimental flow's planner contract forbids invoking it.
//
// `Libc == "none"` maps to CARGO_CFG_TARGET_ENV="" (matches cargo's
// emission for darwin etc.).
func (t Target) CargoCfgEnv() map[string]string {
	libc := t.Libc
	if libc == "none" {
		libc = ""
	}
	out := map[string]string{
		"CARGO_CFG_TARGET_OS":            t.OS,
		"CARGO_CFG_TARGET_ARCH":          t.Arch,
		"CARGO_CFG_TARGET_FAMILY":        t.Family(),
		"CARGO_CFG_TARGET_ENV":           libc,
		"CARGO_CFG_TARGET_POINTER_WIDTH": t.PointerWidth(),
		"CARGO_CFG_TARGET_ENDIAN":        t.Endian(),
		"CARGO_CFG_TARGET_VENDOR":        t.Vendor(),
	}
	switch t.Family() {
	case "unix":
		out["CARGO_CFG_UNIX"] = ""
	case "windows":
		out["CARGO_CFG_WINDOWS"] = ""
	}
	return out
}

// PkgEnv synthesises the CARGO_PKG_* env vars from a manifest extract.
//
// Version is parsed into MAJOR/MINOR/PATCH/PRE components; if the input
// isn't a well-formed semver the components are left empty (matching
// cargo's behaviour on broken manifests -- it doesn't refuse to build).
func PkgEnv(m PkgMetadata) map[string]string {
	major, minor, patch, pre := splitSemver(m.Version)
	return map[string]string{
		"CARGO_PKG_NAME":          m.Name,
		"CARGO_PKG_VERSION":       m.Version,
		"CARGO_PKG_VERSION_MAJOR": major,
		"CARGO_PKG_VERSION_MINOR": minor,
		"CARGO_PKG_VERSION_PATCH": patch,
		"CARGO_PKG_VERSION_PRE":   pre,
		"CARGO_PKG_AUTHORS":       strings.Join(m.Authors, ":"),
		"CARGO_PKG_DESCRIPTION":   m.Description,
		"CARGO_PKG_HOMEPAGE":      m.Homepage,
		"CARGO_PKG_LICENSE":       m.License,
		"CARGO_PKG_LICENSE_FILE":  m.LicenseFile,
		"CARGO_PKG_REPOSITORY":    m.Repository,
		"CARGO_PKG_RUST_VERSION":  m.RustVersion,
		"CARGO_PKG_README":        m.Readme,
	}
}

// FeatureEnv emits `CARGO_FEATURE_<NAME>=1` for each enabled feature.
//
// Cargo uppercases the feature name and replaces `-` with `_`; we do
// the same so build scripts that read `env!("CARGO_FEATURE_FOO_BAR")`
// see the value they expect for a feature called `foo-bar`.
func FeatureEnv(features []string) map[string]string {
	out := make(map[string]string, len(features))
	for _, f := range features {
		key := "CARGO_FEATURE_" + strings.ToUpper(strings.ReplaceAll(f, "-", "_"))
		out[key] = "1"
	}
	return out
}

// splitSemver extracts MAJOR.MINOR.PATCH-PRE from a SemVer 2.0.0 string.
// Returns empty strings for unparseable inputs rather than erroring --
// cargo doesn't refuse to build on a broken Cargo.toml here.
func splitSemver(v string) (major, minor, patch, pre string) {
	if v == "" {
		return "", "", "", ""
	}
	core, prePart, hasPre := strings.Cut(v, "-")
	if hasPre {
		pre = prePart
	}
	// strip build metadata (`+...`) from either side
	if i := strings.Index(core, "+"); i >= 0 {
		core = core[:i]
	}
	if i := strings.Index(pre, "+"); i >= 0 {
		pre = pre[:i]
	}
	parts := strings.SplitN(core, ".", 3)
	if len(parts) > 0 {
		major = parts[0]
	}
	if len(parts) > 1 {
		minor = parts[1]
	}
	if len(parts) > 2 {
		patch = parts[2]
	}
	return major, minor, patch, pre
}

// MergeEnv merges several env maps into one, with later maps overriding
// earlier ones. Useful for the orchestrator to combine layers
// (base + pkg + features + cfg + per-unit overrides).
func MergeEnv(layers ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, layer := range layers {
		maps.Copy(out, layer)
	}
	return out
}

// SortedKeys returns the keys of m in sorted order. Helps build
// deterministic command output even where Go map iteration would shuffle.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
