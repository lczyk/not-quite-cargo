// Package driver is a minimal cc-driver replacement that forwards
// to an ld-style linker after translating gcc-style link args into
// raw ld-style.
//
// rustc's default linker invocation for the linux-gnu targets uses
// cc as a driver: gcc-style flags like -Wl,foo,bar, -nodefaultlibs,
// -fpic and implicit crt/interpreter selection. Raw linkers (wild,
// mold, lld, plain GNU ld) don't understand any of that. This package
// owns the small translation layer so we don't ship a separate bash
// shim.
//
// Drive is invoked when not-quite-cargo is run with argv[0] basename
// "cc" (typically via a symlink at /usr/bin/cc) or via the explicit
// `not-quite-cargo drive ...` subcommand. The actual linker binary
// is whatever LinkerPath points at -- defaults to /usr/bin/ld.
package driver

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// Config tweaks the linker invocation. All fields have safe defaults
// suitable for a chiseled linux-gnu rock that ships an ld-style linker
// at /usr/bin/ld plus libgcc-*-dev + libc6-dev.
type Config struct {
	// LinkerPath is the ld-style linker binary to exec. Defaults to
	// /usr/bin/ld (symlink in the rock, typically pointing at wild).
	LinkerPath string
	// Triple is the multiarch triple (e.g. "aarch64-linux-gnu"); auto
	// detected from runtime.GOARCH when empty.
	Triple string
	// Interp is the dynamic-linker path baked into PT_INTERP. Auto
	// detected from Triple when empty.
	Interp string
	// LibDir / GccLibDir are the two -L paths added before the user
	// args. Auto derived from Triple when empty.
	LibDir    string
	GccLibDir string
}

// envOr returns the env value if non-empty, else fallback. Used so
// callers can leave fields blank and let env / built-in defaults win.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fillDefaults resolves blank fields in three tiers:
//
//  1. CLI flag (already set on Config by the caller; left untouched).
//  2. Env var (NQC_DRIVER_*). The argv[0]==cc shim mode owns no
//     command line of its own -- env vars are the only knob there.
//  3. Built-in default (PATH-ish system path).
func (c *Config) fillDefaults() error {
	if c.LinkerPath == "" {
		c.LinkerPath = envOr("NQC_DRIVER_LINKER", "/usr/bin/ld")
	}
	if c.Triple == "" {
		c.Triple = os.Getenv("NQC_DRIVER_TRIPLE")
	}
	if c.Triple == "" {
		switch runtime.GOARCH {
		case "amd64":
			c.Triple = "x86_64-linux-gnu"
		case "arm64":
			c.Triple = "aarch64-linux-gnu"
		default:
			return fmt.Errorf("driver: unsupported GOARCH %q", runtime.GOARCH)
		}
	}
	if c.Interp == "" {
		c.Interp = os.Getenv("NQC_DRIVER_INTERP")
	}
	if c.Interp == "" {
		switch c.Triple {
		case "x86_64-linux-gnu":
			c.Interp = "/usr/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2"
		case "aarch64-linux-gnu":
			c.Interp = "/usr/lib/aarch64-linux-gnu/ld-linux-aarch64.so.1"
		default:
			return fmt.Errorf("driver: unknown interpreter for triple %q", c.Triple)
		}
	}
	if c.LibDir == "" {
		c.LibDir = envOr("NQC_DRIVER_LIB_DIR", "/usr/lib/"+c.Triple)
	}
	if c.GccLibDir == "" {
		c.GccLibDir = envOr("NQC_DRIVER_GCC_LIB_DIR", "/usr/lib/gcc/"+c.Triple+"/14")
	}
	return nil
}

// gcc-driver-only flags that wild does not understand: they tell the
// driver how to find libs / startup files. wild gets the linker line
// directly, so the flags are meaningless and must be dropped.
//
// -plugin* / -fuse-ld=* / -flto* relate to LTO and linker-plugin
// loading; the rock builds wild without the `plugins` feature, so
// these arguments would be unrecognised. Stripped defensively.
var dropExact = map[string]bool{
	"-nodefaultlibs": true,
	"-nostartfiles":  true,
	"-nostdlib":      true,
	"-pthread":       true,
	"-fpic":          true,
	"-fPIC":          true,
	"-fPIE":          true,
	"-fpie":          true,
	"-plugin":        true,
}

func shouldDrop(arg string) bool {
	if dropExact[arg] {
		return true
	}
	switch {
	case strings.HasPrefix(arg, "-plugin-opt="):
		return true
	case strings.HasPrefix(arg, "-fuse-ld="):
		return true
	case strings.HasPrefix(arg, "-flto"):
		return true
	}
	return false
}

// Translate turns the gcc-style args coming from rustc into the
// wild-bound args. Pure: no IO, no env reads, suitable for testing.
//
// Args layout produced (in order):
//
//	-dynamic-linker <interp>   (only when not -shared)
//	-L <libdir>
//	-L <gcc-lib-dir>
//	<crt-pre>                  (startup objs, varies w/ -shared / -pie)
//	<translated user args>     (gcc-style flags stripped; -Wl,a,b,c
//	                            expanded to separate args)
//	<crt-post>                 (init/fini wrap-up objs)
func Translate(args []string, cfg *Config) ([]string, error) {
	if err := cfg.fillDefaults(); err != nil {
		return nil, err
	}

	isShared := false
	isPie := false
	for _, a := range args {
		switch a {
		case "-shared":
			isShared = true
		case "-pie":
			isPie = true
		}
	}

	var crtPre, crtPost []string
	switch {
	case isShared:
		// crtbeginS / crtendS provide PIC-safe ctor/dtor stubs that
		// .so objects need; the entry-point crt (Scrt1/crt1) is
		// skipped for shared objects.
		crtPre = []string{cfg.GccLibDir + "/crtbeginS.o"}
		crtPost = []string{cfg.GccLibDir + "/crtendS.o"}
	case isPie:
		crtPre = []string{
			cfg.LibDir + "/Scrt1.o",
			cfg.LibDir + "/crti.o",
			cfg.GccLibDir + "/crtbeginS.o",
		}
		crtPost = []string{
			cfg.GccLibDir + "/crtendS.o",
			cfg.LibDir + "/crtn.o",
		}
	default:
		crtPre = []string{
			cfg.LibDir + "/crt1.o",
			cfg.LibDir + "/crti.o",
			cfg.GccLibDir + "/crtbegin.o",
		}
		crtPost = []string{
			cfg.GccLibDir + "/crtend.o",
			cfg.LibDir + "/crtn.o",
		}
	}

	mid := make([]string, 0, len(args))
	for _, a := range args {
		if shouldDrop(a) {
			continue
		}
		if strings.HasPrefix(a, "-Wl,") {
			// Split on commas: -Wl,a,b,c -> a b c.
			for _, p := range strings.Split(strings.TrimPrefix(a, "-Wl,"), ",") {
				if p != "" {
					mid = append(mid, p)
				}
			}
			continue
		}
		mid = append(mid, a)
	}

	out := make([]string, 0, len(mid)+len(crtPre)+len(crtPost)+6)
	if !isShared {
		out = append(out, "-dynamic-linker", cfg.Interp)
	}
	out = append(out, "-L", cfg.LibDir, "-L", cfg.GccLibDir)
	out = append(out, crtPre...)
	out = append(out, mid...)
	out = append(out, crtPost...)
	return out, nil
}

// Drive translates `args` and exec's the configured linker with the
// result. On success this function does not return (uses execve).
// On any error before the exec, returns the error.
func Drive(args []string, cfg *Config) error {
	ldArgs, err := Translate(args, cfg)
	if err != nil {
		return err
	}

	// Resolve LinkerPath via PATH lookup so relative paths work;
	// then execve so rustc sees the linker's exit status directly.
	bin, err := exec.LookPath(cfg.LinkerPath)
	if err != nil {
		return fmt.Errorf("driver: locate linker %q: %w", cfg.LinkerPath, err)
	}
	argv := append([]string{bin}, ldArgs...)
	return syscall.Exec(bin, argv, os.Environ())
}
