package unitgraph

import (
	"path/filepath"
	"strings"
)

// OutputPaths is everything a single compilation unit drops on disk.
type OutputPaths struct {
	// Primary is the main artefact (rlib for libraries, executable for
	// bins, dylib/so for proc-macros). Always set.
	Primary string
	// DepInfo is the rustc-generated `.d` dep-info file. Always set.
	DepInfo string
	// Extras lists secondary artefacts -- a `cdylib` target produces both
	// an rlib (Primary) and a `.so`/`.dylib`/`.dll` (Extras), for
	// instance. May be empty.
	Extras []string
}

// PathInputs is what OutputPathsFor needs to compute paths.
type PathInputs struct {
	ProjectRoot string
	// ProfileDir is the per-profile target subdirectory, e.g. "debug",
	// "release". Cargo derives this from `profile.name` with renames
	// (the "dev" profile lands under target/debug).
	ProfileDir string
	// Platform drives the target/<triple>/ subdirectory: non-empty for
	// cross-compile target units, empty for host units (proc macros,
	// build scripts, plus everything in a non-cross build).
	Platform string
	// ExtPlatform drives only the file-extension decision (`.dylib` on
	// darwin, `.dll` on windows, `.so` elsewhere) -- proc-macros are
	// host units with empty Platform but need the correct host
	// extension, so the orchestrator passes the resolved host triple
	// here when Platform is empty.
	ExtPlatform string
	CrateName   string
	Hash        string
	// TargetKinds is the unit's `target.kind` field; first known kind
	// determines the layout. Recognised: "lib", "rlib", "bin",
	// "proc-macro", "cdylib", "staticlib", "custom-build".
	TargetKinds []string
}

// OutputPathsFor mirrors cargo's `target/<profile>/[<triple>/]deps/<crate>-<hash>.<ext>`
// layout, substituting our hash for cargo's.
func OutputPathsFor(in PathInputs) OutputPaths {
	base := filepath.Join(in.ProjectRoot, "target")
	if in.Platform != "" {
		base = filepath.Join(base, in.Platform)
	}
	base = filepath.Join(base, in.ProfileDir, "deps")

	stem := in.CrateName + "-" + in.Hash
	op := OutputPaths{
		DepInfo: filepath.Join(base, stem+".d"),
	}

	for _, kind := range in.TargetKinds {
		switch kind {
		case "lib", "rlib":
			if op.Primary == "" {
				op.Primary = filepath.Join(base, "lib"+stem+".rlib")
			}
		case "bin":
			if op.Primary == "" {
				op.Primary = filepath.Join(base, stem+binExt(in.ExtPlatform))
			}
		case "proc-macro":
			if op.Primary == "" {
				op.Primary = filepath.Join(base, "lib"+stem+dylibExt(in.ExtPlatform))
			}
		case "cdylib":
			op.Extras = append(op.Extras, filepath.Join(base, "lib"+stem+dylibExt(in.ExtPlatform)))
		case "staticlib":
			op.Extras = append(op.Extras, filepath.Join(base, "lib"+stem+".a"))
		case "custom-build":
			// rustc names the output by `--crate-name` + `--extra-filename`,
			// so the build-script binary lands at
			// `<deps>/<crate_name>-<hash>` (no `lib` prefix, no extension
			// on unix). The crate name itself is the unit's target.name
			// canonicalised (typically "build_script_build").
			if op.Primary == "" {
				op.Primary = filepath.Join(base, stem+binExt(in.ExtPlatform))
			}
		}
	}
	return op
}

func binExt(platform string) string {
	if strings.Contains(platform, "windows") {
		return ".exe"
	}
	return ""
}

func dylibExt(platform string) string {
	switch {
	case strings.Contains(platform, "darwin"), strings.Contains(platform, "apple"):
		return ".dylib"
	case strings.Contains(platform, "windows"):
		return ".dll"
	default:
		return ".so"
	}
}
