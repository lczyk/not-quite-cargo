package cargo

import (
	"fmt"
	"maps"
	"strings"
)

// strippedEnvKeys are dropped from invocation env during patching because the
// runtime injects fresh values from Config.
var strippedEnvKeys = []string{"CARGO", "PROJECT_ROOT", "CARGO_HOME", "RUSTC"}

// PatchOptions bundles the optional knobs PatchPlan accepts. Empty / zero
// values are no-ops. Fields:
//
//   - Linker -- when non-empty, appends `-C linker=<Linker>` to every rustc
//     invocation. Same flag exists on `run` (which wins -- rustc honours the
//     last `-C linker=...` on the command line).
//   - CodegenBackend -- when non-empty, appends `-Z codegen-backend=<value>`.
//     The value can be a built-in backend name (e.g. "cranelift") or an
//     absolute path to a backend .so. Useful when the rock ships a rustc
//     with a non-default codegen backend and you want to make plans use it.
//   - Panic -- when non-empty, appends `-C panic=<value>`. Workaround for
//     cranelift-built rustc which only supports `panic=abort`; planner-side
//     plans default to `panic=unwind` on release.
//   - NoLTO -- when true, strips any LTO-family codegen flags and appends
//     `-C lto=off`. Workaround for backends that can't do LTO (cranelift):
//     left in place, rustc omits upstream rlibs from the final link
//     expecting LTO to inline them, and the link fails with undefined
//     symbols (std / generic instantiations). Same knob exists on `run`.
type PatchOptions struct {
	Linker         string
	CodegenBackend string
	Panic          string
	NoLTO          bool
}

// ltoCodegenValue reports whether v is the value half of an LTO-family
// `-C <v>` codegen flag: lto, linker-plugin-lto or embed-bitcode (bare or
// `=...`).
func ltoCodegenValue(v string) bool {
	for _, k := range []string{"lto", "linker-plugin-lto", "embed-bitcode"} {
		if v == k || strings.HasPrefix(v, k+"=") {
			return true
		}
	}
	return false
}

// stripLTO removes LTO-family codegen flags -- both the two-token `-C lto`
// / `--codegen lto` form and the single-token `-Clto=...` / `--codegen=lto`
// form -- and appends `-C lto=off`.
//
// cranelift can't LTO. Left in place, a requested `-C lto` makes rustc omit
// the upstream rlibs from the final link (it expects LTO to pull their code
// in as bitcode), so the link dies with undefined symbols. embed-bitcode is
// stripped too: `-C embed-bitcode=no` is rejected alongside a live `-C lto`,
// and `lto=off` leaves embed-bitcode off by default anyway.
func stripLTO(args []string) []string {
	out := make([]string, 0, len(args)+2)
	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		// two-token: "-C"/"--codegen" followed by an LTO-family value.
		if (a == "-C" || a == "--codegen") && i+1 < len(args) && ltoCodegenValue(args[i+1]) {
			skipNext = true
			continue
		}
		// single-token: "-Clto", "-Clto=fat", "--codegen=lto", ...
		if rest, ok := strings.CutPrefix(a, "-C"); ok && rest != "" && ltoCodegenValue(rest) {
			continue
		}
		if rest, ok := strings.CutPrefix(a, "--codegen="); ok && ltoCodegenValue(rest) {
			continue
		}
		out = append(out, a)
	}
	return append(out, "-C", "lto=off")
}

// PatchPlan returns a new build-plan map with concrete paths replaced by
// `{{PROJECT_ROOT}}` / `{{CARGO_HOME}}` placeholders and the `rustc` program
// replaced with `{{RUSTC}}`. Pure transform -- no file IO, no env reads. The
// `{{RUSTC}}` placeholder is never written to disk for any other field; rustc
// is resolved at run time.
//
// `opts` carries optional knobs (linker / codegen backend / panic strategy)
// that get injected into rustc invocations when set.
func PatchPlan(plan map[string]any, projectRoot, cargoHome string, opts PatchOptions) (map[string]any, error) {
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
		patchInvocation(clone, opts)
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
// drop --diagnostic-width which pins terminal width on the patching machine,
// and (per opts) append `-C linker=...`, `-Z codegen-backend=...`, and
// `-C panic=...` to the args of rustc invocations.
func patchInvocation(inv map[string]any, opts PatchOptions) {
	isRustc := false
	if prog, ok := inv["program"].(string); ok && prog == "rustc" {
		inv["program"] = "{{RUSTC}}"
		isRustc = true
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
		if isRustc {
			if opts.Linker != "" {
				filtered = append(filtered, "-C", "linker="+opts.Linker)
			}
			if opts.CodegenBackend != "" {
				filtered = append(filtered, "-Z", "codegen-backend="+opts.CodegenBackend)
			}
			if opts.Panic != "" {
				filtered = append(filtered, "-C", "panic="+opts.Panic)
			}
			if opts.NoLTO {
				strs := make([]string, 0, len(filtered))
				allStrings := true
				for _, a := range filtered {
					s, ok := a.(string)
					if !ok {
						allStrings = false
						break
					}
					strs = append(strs, s)
				}
				if allStrings {
					filtered = filtered[:0]
					for _, s := range stripLTO(strs) {
						filtered = append(filtered, s)
					}
				}
			}
		}
		inv["args"] = filtered
	}
}
