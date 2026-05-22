package cargo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run executes the build plan at path, after string-replacing placeholders
// according to cfg.
func Run(path string, cfg *Config) error {
	logger := cfg.Logger
	if logger == nil {
		logger = stdLogger{}
	}

	if out, err := exec.Command(cfg.RustcPath, "-vV").CombinedOutput(); err != nil {
		return fmt.Errorf("get rustc version from %s: %w\noutput:\n%s", cfg.RustcPath, err, out)
	} else {
		logger.Infof("rustc version: %s", strings.SplitN(string(out), "\n", 2)[0])
	}

	plan, err := loadPlanJSON(path)
	if err != nil {
		return err
	}
	invsRaw, _ := plan["invocations"].([]any)

	replacements := cfg.Replacements()
	replaced := make([]any, len(invsRaw))
	for i, inv := range invsRaw {
		replaced[i] = DeepReplace(inv, replacements)
	}

	invs, err := decodeInvocations(replaced)
	if err != nil {
		return err
	}
	ordered, err := ResolveInvocationOrder(invs)
	if err != nil {
		return err
	}

	// Pre-create output directories.
	for _, inv := range ordered {
		for _, out := range inv.Outputs {
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return fmt.Errorf("mkdir for output %s: %w", out, err)
			}
		}
	}

	directives := map[string]*CustomBuildDirectives{}

	for i, inv := range ordered {
		args := append([]string(nil), inv.Args...)
		program := inv.Program
		if program == "" {
			program = cfg.RustcPath
		}

		// Apply directives from any build script for the same package that
		// has already run.
		invEnv := map[string]string{}
		for k, v := range inv.Env {
			invEnv[k] = v
		}
		if d, ok := directives[inv.PackageName]; ok {
			args = append(args, d.RustcFlags...)
			for k, v := range d.EnvVars {
				invEnv[k] = v
			}
		}

		if outDir, ok := invEnv["OUT_DIR"]; ok {
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("mkdir OUT_DIR %s: %w", outDir, err)
			}
		}

		cmd := exec.Command(program, args...)
		cmd.Dir = inv.Cwd
		cmd.Env = buildEnv(cfg, invEnv)

		logger.Infof("(%d/%d) running '%s' for package '%s' v%s",
			i+1, len(ordered), inv.Program, inv.PackageName, inv.PackageVersion)
		logger.Infof("invoking: %s %s", program, truncate(strings.Join(args, " "), 100))

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			logger.Warnf("command failed: %s %s", program, strings.Join(args, " "))
			logger.Warnf("stdout:\n%s", stdout.String())
			logger.Warnf("stderr:\n%s", stderr.String())
			return fmt.Errorf("invocation %d (%s) failed: %w", inv.Number, inv.PackageName, err)
		}

		for link, target := range inv.Links {
			if _, err := os.Lstat(link); err == nil {
				if err := os.Remove(link); err != nil {
					return fmt.Errorf("remove stale symlink %s: %w", link, err)
				}
			}
			if err := os.Symlink(target, link); err != nil {
				return fmt.Errorf("create symlink %s -> %s: %w", link, target, err)
			}
			logger.Infof("symlink: %s -> %s", link, target)
		}

		if inv.CompileMode == "run-custom-build" {
			directives[inv.PackageName] = ParseBuildScriptOutput(stdout.String(), logger)
		}
	}

	logger.Infof("build plan execution complete")
	return nil
}

// buildEnv merges the parent env, cfg-derived vars and per-invocation vars,
// deduping by key so the rightmost wins. Order: parent < cfg < invocation,
// so an invocation env entry overrides anything else.
func buildEnv(cfg *Config, invEnv map[string]string) []string {
	merged := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			merged[k] = v
		}
	}
	merged["RUSTC"] = cfg.RustcPath
	merged["CARGO_HOME"] = cfg.CargoHome
	merged["PROJECT_ROOT"] = cfg.ProjectRoot
	for k, v := range invEnv {
		merged[k] = v
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
