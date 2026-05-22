package cargo

import "strings"

// CustomBuildDirectives captures the rustc-affecting output of a build script.
type CustomBuildDirectives struct {
	RustcFlags []string
	EnvVars    map[string]string
}

var ignoredDirectiveKeys = map[string]struct{}{
	"rerun-if-changed":     {},
	"rerun-if-env-changed": {},
}

// ParseBuildScriptOutput extracts rustc flags and env vars from the stdout of
// a build script (lines starting with "cargo:").
func ParseBuildScriptOutput(output string, logger Logger) *CustomBuildDirectives {
	if logger == nil {
		logger = stdLogger{}
	}
	d := &CustomBuildDirectives{
		RustcFlags: []string{},
		EnvVars:    map[string]string{},
	}

	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "cargo:") {
			continue
		}
		line = strings.TrimPrefix(line, "cargo:")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			logger.Warnf("malformed build script line (no '='): %s", raw)
			continue
		}
		if _, skip := ignoredDirectiveKeys[key]; skip {
			continue
		}
		switch key {
		case "rustc-cfg":
			d.RustcFlags = append(d.RustcFlags, "--cfg", value)
		case "rustc-check-cfg":
			d.RustcFlags = append(d.RustcFlags, "--check-cfg", value)
		case "rustc-link-lib":
			d.RustcFlags = append(d.RustcFlags, "-l", value)
		case "rustc-link-arg":
			d.RustcFlags = append(d.RustcFlags, "-C", "link-arg="+value)
		case "rustc-link-search":
			// value may be `kind=path` or just `path`
			if _, path, ok := strings.Cut(value, "="); ok {
				d.RustcFlags = append(d.RustcFlags, "-L", path)
			} else {
				d.RustcFlags = append(d.RustcFlags, "-L", value)
			}
		case "rustc-env":
			if k, v, ok := strings.Cut(value, "="); ok {
				d.EnvVars[k] = v
			}
		default:
			logger.Warnf("unknown build script directive: %s", raw)
		}
	}
	return d
}
