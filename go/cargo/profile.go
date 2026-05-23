package cargo

import (
	"maps"
	"strings"
)

// ProfileSpec describes the codegen + env values for a cargo build profile.
type ProfileSpec struct {
	Name            string
	OptLevel        string
	Debuginfo       string
	DebugAssertions string
	OverflowChecks  string
	DebugEnv        string
}

var Release = ProfileSpec{
	Name:            "release",
	OptLevel:        "3",
	Debuginfo:       "0",
	DebugAssertions: "false",
	OverflowChecks:  "false",
	DebugEnv:        "false",
}

var Debug = ProfileSpec{
	Name:            "debug",
	OptLevel:        "0",
	Debuginfo:       "2",
	DebugAssertions: "true",
	OverflowChecks:  "true",
	DebugEnv:        "true",
}

// ParseProfile maps the user-facing flag value to a known spec.
func ParseProfile(s string) (ProfileSpec, bool) {
	switch s {
	case "release":
		return Release, true
	case "debug":
		return Debug, true
	default:
		return ProfileSpec{}, false
	}
}

// RewriteProfile mutates a build-plan map to retarget any `/release/` or
// `/debug/` path segments and codegen-flag / env values to the requested
// profile. Auto-detects the source profile from invocation outputs; falls
// back to the target name (i.e. noop on paths) when undetectable.
func RewriteProfile(plan map[string]any, target ProfileSpec) {
	source := detectSource(plan)
	if source == "" {
		source = target.Name
	}
	if invs, ok := plan["invocations"].([]any); ok {
		for _, invAny := range invs {
			if inv, ok := invAny.(map[string]any); ok {
				rewriteInvocation(inv, source, target)
			}
		}
	}
	if inputs, ok := plan["inputs"].([]any); ok {
		for i, in := range inputs {
			if s, ok := in.(string); ok {
				inputs[i] = swapSegment(s, source, target.Name)
			}
		}
	}
}

func detectSource(plan map[string]any) string {
	invs, ok := plan["invocations"].([]any)
	if !ok {
		return ""
	}
	for _, invAny := range invs {
		inv, ok := invAny.(map[string]any)
		if !ok {
			continue
		}
		outs, ok := inv["outputs"].([]any)
		if !ok {
			continue
		}
		for _, o := range outs {
			s, ok := o.(string)
			if !ok {
				continue
			}
			if strings.Contains(s, "/release/") {
				return "release"
			}
			if strings.Contains(s, "/debug/") {
				return "debug"
			}
		}
	}
	return ""
}

func swapSegment(s, source, target string) string {
	if source == target {
		return s
	}
	return strings.ReplaceAll(s, "/"+source+"/", "/"+target+"/")
}

func rewriteInvocation(inv map[string]any, source string, target ProfileSpec) {
	if prog, ok := inv["program"].(string); ok {
		inv["program"] = swapSegment(prog, source, target.Name)
	}
	if args, ok := inv["args"].([]any); ok {
		rewriteArgs(args, source, target)
	}
	if outs, ok := inv["outputs"].([]any); ok {
		for i, o := range outs {
			if s, ok := o.(string); ok {
				outs[i] = swapSegment(s, source, target.Name)
			}
		}
	}
	if links, ok := inv["links"].(map[string]any); ok {
		// Rebuild because keys may collide on rename.
		newLinks := make(map[string]any, len(links))
		for k, v := range links {
			nk := swapSegment(k, source, target.Name)
			if s, ok := v.(string); ok {
				newLinks[nk] = swapSegment(s, source, target.Name)
			} else {
				newLinks[nk] = v
			}
		}
		for k := range links {
			delete(links, k)
		}
		maps.Copy(links, newLinks)
	}
	if cwd, ok := inv["cwd"].(string); ok {
		inv["cwd"] = swapSegment(cwd, source, target.Name)
	}
	if env, ok := inv["env"].(map[string]any); ok {
		for k, v := range env {
			s, ok := v.(string)
			if !ok {
				continue
			}
			switch k {
			case "PROFILE":
				env[k] = target.Name
			case "OPT_LEVEL":
				env[k] = target.OptLevel
			case "DEBUG":
				env[k] = target.DebugEnv
			case "DEBUG_ASSERTIONS":
				env[k] = target.DebugAssertions
			case "OVERFLOW_CHECKS":
				env[k] = target.OverflowChecks
			default:
				env[k] = swapSegment(s, source, target.Name)
			}
		}
	}
}

func rewriteArgs(args []any, source string, target ProfileSpec) {
	for i := 0; i < len(args); i++ {
		s, ok := args[i].(string)
		if !ok {
			continue
		}
		// Two-arg form: ["-C", "key=val"]
		if s == "-C" && i+1 < len(args) {
			if next, ok := args[i+1].(string); ok {
				if rewritten, ok := rewriteCodegenValue(next, target); ok {
					args[i+1] = rewritten
				} else {
					args[i+1] = swapSegment(next, source, target.Name)
				}
				i++
				continue
			}
		}
		// One-arg form: "-C key=val"
		if rewritten, ok := rewriteCodegenSingle(s, target); ok {
			args[i] = rewritten
			continue
		}
		args[i] = swapSegment(s, source, target.Name)
	}
}

func rewriteCodegenValue(val string, target ProfileSpec) (string, bool) {
	eq := strings.IndexByte(val, '=')
	if eq < 0 {
		return "", false
	}
	key := val[:eq]
	newVal, ok := profileCodegen(key, target)
	if !ok {
		return "", false
	}
	return key + "=" + newVal, true
}

func rewriteCodegenSingle(s string, target ProfileSpec) (string, bool) {
	body, ok := strings.CutPrefix(s, "-C ")
	if !ok {
		return "", false
	}
	val, ok := rewriteCodegenValue(body, target)
	if !ok {
		return "", false
	}
	return "-C " + val, true
}

func profileCodegen(key string, target ProfileSpec) (string, bool) {
	switch key {
	case "opt-level":
		return target.OptLevel, true
	case "debuginfo":
		return target.Debuginfo, true
	case "debug-assertions":
		return target.DebugAssertions, true
	case "overflow-checks":
		return target.OverflowChecks, true
	default:
		return "", false
	}
}
