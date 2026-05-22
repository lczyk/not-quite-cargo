package unitgraph

import (
	"fmt"
	"sort"
	"strings"
)

// PkgMetadata is the slice of a Cargo.toml that drives CARGO_PKG_* env
// vars. Loaded from manifest.go later; defined here for use in PkgEnv.
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

// CfgMap is the parsed form of `rustc --print cfg`.
//
// A single-element value with an empty string represents a bare boolean
// cfg key like `unix` or `debug_assertions`. Keys that may carry multiple
// values (`target_feature`, `target_has_atomic`, ...) accumulate them in
// order of appearance.
type CfgMap map[string][]string

// ParseCfg converts the raw stdout of `rustc --print cfg` into a CfgMap.
//
// Lines have two shapes:
//   - bare keys -- e.g. `unix`, `debug_assertions` -- map to a single
//     empty-string value, indicating presence.
//   - `key="value"` -- the quotes are required by rustc; we strip them.
//
// Blank lines and lines starting with `//` (rustc never emits them but
// callers sometimes hand us files with comments) are ignored.
func ParseCfg(text string) (CfgMap, error) {
	out := CfgMap{}
	for lineNo, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		key, value, hasValue := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("cfg line %d: empty key in %q", lineNo+1, raw)
		}
		if !hasValue {
			out[key] = append(out[key], "")
			continue
		}
		value = strings.TrimSpace(value)
		// rustc emits values wrapped in double quotes. Strip exactly one
		// pair if present; reject malformed shapes.
		switch {
		case len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"':
			value = value[1 : len(value)-1]
		case value == "":
			// `key=` (empty value) -- accept, treat like bare key.
		default:
			return nil, fmt.Errorf("cfg line %d: value not quoted in %q", lineNo+1, raw)
		}
		out[key] = append(out[key], value)
	}
	return out, nil
}

// CargoCfgEnv turns a CfgMap into the `CARGO_CFG_*` env-var subset that
// cargo would set when invoking rustc or a build script.
//
// Naming rules (matching cargo):
//   - key uppercased, hyphens to underscores -> `CARGO_CFG_<KEY>`
//   - multi-value keys are comma-joined in the order they appeared
//   - bare keys (no `=value`) get an empty value; presence of the env
//     var alone signals truth (build scripts check `env::var_os` for
//     them, not the value)
func CargoCfgEnv(cfg CfgMap) map[string]string {
	out := make(map[string]string, len(cfg))
	for key, values := range cfg {
		envKey := "CARGO_CFG_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		// Collapse "presence only" cfgs (single empty value) to empty
		// string; otherwise comma-join.
		if len(values) == 1 && values[0] == "" {
			out[envKey] = ""
			continue
		}
		out[envKey] = strings.Join(values, ",")
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
