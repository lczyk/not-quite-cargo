package unitgraph

import (
	"sort"
	"strings"
)

// PkgMetadata is the slice of a Cargo.toml that drives CARGO_PKG_* env
// vars. Lower now populates only Name + Version (from pkg_id); the rest
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

// Target captures the cfg-driving subset of a rust target. OS + Arch
// are user inputs; everything else (family, env, pointer-width, endian,
// vendor) derives from those two unless explicitly overridden.
//
// Replaces the prior CfgMap abstraction. The two fields are what build
// scripts read 99% of the time; the rest is a deterministic function of
// them, so we compute on demand rather than ask the user.
type Target struct {
	// OS is the lower-case rust target OS (linux, macos, windows,
	// freebsd, openbsd, netbsd, android, ios, ...).
	OS string
	// Arch is the lower-case rust target architecture (x86_64,
	// aarch64, x86, arm, ...).
	Arch string
	// Env optionally overrides the libc env (gnu / musl / msvc); empty
	// asks Target to pick a sensible default from OS.
	Env string
}

// Triple synthesises a rust target triple like "aarch64-apple-darwin"
// from OS + Arch. Used to populate TARGET / HOST env vars and as the
// HostTriple input to Lower.
func (t Target) Triple() string {
	switch t.OS {
	case "macos", "ios":
		return t.Arch + "-apple-" + t.OS
	case "windows":
		env := t.envOr("msvc")
		return t.Arch + "-pc-windows-" + env
	case "linux":
		env := t.envOr("gnu")
		return t.Arch + "-unknown-linux-" + env
	default:
		if t.OS == "" {
			return t.Arch
		}
		return t.Arch + "-unknown-" + t.OS
	}
}

// Family returns "unix" / "windows" / "" depending on OS.
func (t Target) Family() string {
	switch t.OS {
	case "linux", "macos", "ios", "android", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "illumos":
		return "unix"
	case "windows":
		return "windows"
	default:
		return ""
	}
}

// PointerWidth returns "64" / "32" / "" depending on Arch.
func (t Target) PointerWidth() string {
	switch t.Arch {
	case "x86_64", "aarch64", "powerpc64", "powerpc64le", "riscv64", "s390x", "mips64", "mips64el", "loongarch64", "sparc64":
		return "64"
	case "i686", "i586", "i386", "x86", "arm", "armv7", "thumbv7em", "riscv32", "mips", "mipsel", "powerpc", "wasm32":
		return "32"
	default:
		return ""
	}
}

// Endian returns "big" / "little".
func (t Target) Endian() string {
	switch t.Arch {
	case "powerpc64", "powerpc", "mips", "mips64", "s390x", "sparc64":
		return "big"
	default:
		return "little"
	}
}

// Vendor returns the rust target-triple vendor token. Cargo's
// CARGO_CFG_TARGET_VENDOR uses this.
func (t Target) Vendor() string {
	switch t.OS {
	case "macos", "ios":
		return "apple"
	case "windows":
		return "pc"
	default:
		return "unknown"
	}
}

func (t Target) envOr(def string) string {
	if t.Env != "" {
		return t.Env
	}
	return def
}

// CargoCfgEnv emits the CARGO_CFG_* env-var subset that cargo would set
// when invoking rustc or a build script for this target.
//
// Covers TARGET_OS / _ARCH / _FAMILY / _ENV / _POINTER_WIDTH / _ENDIAN /
// _VENDOR plus the bare unix/windows cfg key. less-common cfgs (
// target_feature, target_has_atomic, debug_assertions) are deliberately
// omitted: they need rustc to enumerate them faithfully, and the
// experimental flow's planner contract forbids invoking it.
func (t Target) CargoCfgEnv() map[string]string {
	out := map[string]string{
		"CARGO_CFG_TARGET_OS":            t.OS,
		"CARGO_CFG_TARGET_ARCH":          t.Arch,
		"CARGO_CFG_TARGET_FAMILY":        t.Family(),
		"CARGO_CFG_TARGET_ENV":           t.envOr(defaultEnvFor(t.OS)),
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

func defaultEnvFor(os string) string {
	switch os {
	case "linux", "android":
		return "gnu"
	case "windows":
		return "msvc"
	default:
		return ""
	}
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
		for k, v := range layer {
			out[k] = v
		}
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
