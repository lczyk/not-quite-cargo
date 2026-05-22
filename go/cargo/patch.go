package cargo

import (
	"fmt"
	"maps"
	"strings"
)

// strippedEnvKeys are dropped from invocation env during patching because the
// runtime injects fresh values from Config.
var strippedEnvKeys = []string{"CARGO", "PROJECT_ROOT", "CARGO_HOME", "RUSTC"}

// PatchPlan returns a new build-plan map with concrete paths replaced by
// `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders and the `rustc` program
// replaced with `{{RUSTC}}`. Pure transform -- no file IO, no env reads. The
// `{{RUSTC}}` placeholder is never written to disk for any other field; rustc
// is resolved at run time.
func PatchPlan(plan map[string]any, projectRoot, cargoHome string) (map[string]any, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("PatchPlan: projectRoot is required")
	}
	if cargoHome == "" {
		return nil, fmt.Errorf("PatchPlan: cargoHome is required")
	}

	// Reverse replacements (path -> placeholder). RUSTC is intentionally not
	// substituted -- it's a runtime concern, only the literal `program: rustc`
	// gets templated.
	reverse := map[string]string{
		projectRoot: "{{PROJECT_ROOT}}",
		cargoHome:   "{{CARGO_HOME}}",
	}

	out := map[string]any{}
	maps.Copy(out, plan)

	invsRaw, _ := plan["invocations"].([]any)
	patched := make([]any, len(invsRaw))
	for i, invAny := range invsRaw {
		inv, ok := invAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invocation %d has unexpected shape", i)
		}
		// Copy + mutate to avoid touching the caller's map.
		clone := map[string]any{}
		maps.Copy(clone, inv)
		patchInvocation(clone)
		patched[i] = DeepReplace(clone, reverse)
	}
	out["invocations"] = patched

	if inputs, ok := plan["inputs"].([]any); ok {
		inputsOut := make([]any, len(inputs))
		for i, input := range inputs {
			if s, ok := input.(string); ok {
				inputsOut[i] = replaceString(s, reverse)
			} else {
				inputsOut[i] = input
			}
		}
		out["inputs"] = inputsOut
	}

	return out, nil
}

// patchInvocation applies the structural rewrites that aren't pure string
// replacement: swap program=rustc to a placeholder, strip injected env keys,
// drop --diagnostic-width which pins terminal width on the patching machine.
func patchInvocation(inv map[string]any) {
	if prog, ok := inv["program"].(string); ok && prog == "rustc" {
		inv["program"] = "{{RUSTC}}"
	}
	if env, ok := inv["env"].(map[string]any); ok {
		for _, k := range strippedEnvKeys {
			delete(env, k)
		}
	}
	if args, ok := inv["args"].([]any); ok {
		filtered := make([]any, 0, len(args))
		skipNext := false
		for _, a := range args {
			if skipNext {
				skipNext = false
				continue
			}
			if s, ok := a.(string); ok {
				if s == "--diagnostic-width" {
					// Two-arg form: drop value too.
					skipNext = true
					continue
				}
				if strings.HasPrefix(s, "--diagnostic-width=") {
					continue
				}
			}
			filtered = append(filtered, a)
		}
		inv["args"] = filtered
	}
}
