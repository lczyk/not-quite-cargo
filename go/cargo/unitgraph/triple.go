package unitgraph

import "strings"

// CfgFromTriple builds a minimal CfgMap from a rust target triple.
//
// Covers the CARGO_CFG_* env vars build scripts most commonly read:
// target_os / _arch / _family / _env / _pointer_width / _endian, plus
// the bare unix/windows cfg key. Less common cfgs (target_feature,
// target_has_atomic, debug_assertions) are omitted -- callers that need
// faithful coverage should still feed a captured `rustc --print cfg`
// dump via ParseCfg instead.
//
// Unknown triples are best-effort: missing fields stay empty so build
// scripts that branch on them see a "not present" answer.
func CfgFromTriple(triple string) CfgMap {
	cfg := CfgMap{}
	parts := strings.Split(triple, "-")
	if len(parts) < 3 {
		return cfg
	}
	arch := parts[0]

	// Normalise rust's i686/i586 to match cargo conventions.
	cfg["target_arch"] = []string{arch}

	// OS + family + env + endian + pointer-width are derived from the
	// rest of the triple. Tables are partial; extend as new targets
	// become interesting.
	os, family, env := osFamilyEnvFromTriple(parts)
	if os != "" {
		cfg["target_os"] = []string{os}
	}
	if family != "" {
		cfg["target_family"] = []string{family}
	}
	cfg["target_env"] = []string{env}

	if family == "unix" {
		cfg["unix"] = []string{""}
	}
	if family == "windows" {
		cfg["windows"] = []string{""}
	}

	width, endian := pointerWidthEndian(arch)
	if width != "" {
		cfg["target_pointer_width"] = []string{width}
	}
	if endian != "" {
		cfg["target_endian"] = []string{endian}
	}
	cfg["target_vendor"] = []string{vendorOf(parts)}

	return cfg
}

func osFamilyEnvFromTriple(parts []string) (os, family, env string) {
	joined := strings.Join(parts[1:], "-")
	switch {
	case strings.Contains(joined, "linux"):
		os = "linux"
		family = "unix"
	case strings.Contains(joined, "darwin") || strings.Contains(joined, "apple"):
		os = "macos"
		family = "unix"
	case strings.Contains(joined, "windows"):
		os = "windows"
		family = "windows"
	case strings.Contains(joined, "freebsd"):
		os = "freebsd"
		family = "unix"
	case strings.Contains(joined, "openbsd"):
		os = "openbsd"
		family = "unix"
	case strings.Contains(joined, "netbsd"):
		os = "netbsd"
		family = "unix"
	case strings.Contains(joined, "android"):
		os = "android"
		family = "unix"
	case strings.Contains(joined, "ios"):
		os = "ios"
		family = "unix"
	}

	switch {
	case strings.Contains(joined, "musl"):
		env = "musl"
	case strings.Contains(joined, "gnu"):
		env = "gnu"
	case strings.Contains(joined, "msvc"):
		env = "msvc"
	}
	return
}

func pointerWidthEndian(arch string) (width, endian string) {
	switch arch {
	case "x86_64", "aarch64", "powerpc64", "powerpc64le", "riscv64", "s390x", "mips64", "mips64el", "loongarch64", "sparc64":
		width = "64"
	case "i686", "i586", "i386", "x86", "arm", "armv7", "thumbv7em", "riscv32", "mips", "mipsel", "powerpc", "wasm32":
		width = "32"
	}
	switch arch {
	case "powerpc64le", "mips64el", "mipsel":
		endian = "little"
	case "powerpc64", "powerpc", "mips", "mips64", "s390x", "sparc64":
		endian = "big"
	default:
		endian = "little"
	}
	return
}

func vendorOf(parts []string) string {
	// Rust triples are usually <arch>-<vendor>-<os>[-<env>].
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}
