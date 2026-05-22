package cargo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// strippedEnvKeys are dropped from invocation env during patching because the
// runtime injects fresh values from Config.
var strippedEnvKeys = []string{"CARGO", "PROJECT_ROOT", "CARGO_HOME", "RUSTC"}

// Patch rewrites the build plan in-place, replacing concrete paths with
// `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders and the `rustc` program
// with `{{RUSTC}}`. The `{{RUSTC}}` placeholder is never written to disk for
// any other field; rustc is resolved at run time.
func Patch(path string, cfg *Config) error {
	if cfg.Logger != nil {
		cfg.Logger.Infof("patching build plan: %s", path)
	}

	plan, err := loadPlanJSON(path)
	if err != nil {
		return err
	}

	// Reverse replacements (path -> placeholder), with RUSTC stripped -- it's
	// a runtime concern.
	reverse := map[string]string{}
	for placeholder, value := range cfg.Replacements() {
		if placeholder == "{{RUSTC}}" {
			continue
		}
		reverse[value] = placeholder
	}

	invsRaw, _ := plan["invocations"].([]any)
	patched := make([]any, len(invsRaw))
	for i, invAny := range invsRaw {
		inv, ok := invAny.(map[string]any)
		if !ok {
			return fmt.Errorf("invocation %d has unexpected shape", i)
		}
		patchInvocation(inv)
		patched[i] = DeepReplace(inv, reverse)
	}
	plan["invocations"] = patched

	if inputs, ok := plan["inputs"].([]any); ok {
		out := make([]any, len(inputs))
		for i, input := range inputs {
			if s, ok := input.(string); ok {
				out[i] = replaceString(s, reverse)
			} else {
				out[i] = input
			}
		}
		plan["inputs"] = out
	}

	body, err := json.MarshalIndent(plan, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal patched plan: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write patched plan: %w", err)
	}
	if cfg.Logger != nil {
		cfg.Logger.Infof("patched build plan saved to %s", path)
	}
	return nil
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
		for _, a := range args {
			if s, ok := a.(string); ok && strings.HasPrefix(s, "--diagnostic-width") {
				continue
			}
			filtered = append(filtered, a)
		}
		inv["args"] = filtered
	}
}
