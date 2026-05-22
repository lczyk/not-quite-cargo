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
	// Platform is the target triple, or empty for host builds.
	Platform  string
	CrateName string
	Hash      string
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
				op.Primary = filepath.Join(base, stem+binExt(in.Platform))
			}
		case "proc-macro":
			if op.Primary == "" {
				op.Primary = filepath.Join(base, "lib"+stem+dylibExt(in.Platform))
			}
		case "cdylib":
			op.Extras = append(op.Extras, filepath.Join(base, "lib"+stem+dylibExt(in.Platform)))
		case "staticlib":
			op.Extras = append(op.Extras, filepath.Join(base, "lib"+stem+".a"))
		case "custom-build":
			if op.Primary == "" {
				op.Primary = filepath.Join(base, "build-script-build-"+in.Hash+binExt(in.Platform))
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
