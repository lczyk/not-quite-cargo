package cargo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const Version = "0.2.1"

// Config holds the resolved environment paths used to patch and run a build plan.
type Config struct {
	ProjectRoot string
	CargoHome   string
	RustcPath   string

	Logger Logger
}

// Replacements returns the placeholder -> path substitutions derived from the config.
func (c *Config) Replacements() map[string]string {
	return map[string]string{
		"{{PROJECT_ROOT}}": c.ProjectRoot,
		"{{CARGO_HOME}}":   c.CargoHome,
		"{{RUSTC}}":        c.RustcPath,
	}
}

// NewConfig resolves PROJECT_ROOT, CARGO_HOME and RUSTC from env / defaults.
func NewConfig(logger Logger) (*Config, error) {
	if logger == nil {
		logger = stdLogger{}
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}
	if pr, ok := os.LookupEnv("PROJECT_ROOT"); ok && pr != "" {
		projectRoot = pr
	}

	cargoHome := os.Getenv("CARGO_HOME")
	if cargoHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		cargoHome = filepath.Join(home, ".cargo")
	}

	rustc, err := findRustc(logger)
	if err != nil {
		return nil, err
	}

	return &Config{
		ProjectRoot: projectRoot,
		CargoHome:   cargoHome,
		RustcPath:   rustc,
		Logger:      logger,
	}, nil
}

// findRustc tries RUSTC env, then rustup, then PATH.
func findRustc(logger Logger) (string, error) {
	if path := os.Getenv("RUSTC"); path != "" {
		logger.Infof("found rustc at %s via RUSTC env", path)
		return path, nil
	}

	if rustup, err := exec.LookPath("rustup"); err == nil {
		cmd := exec.Command(rustup, "which", "rustc")
		cmd.Dir = "/"
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err == nil {
			path := strings.TrimSpace(string(out))
			if path != "" {
				logger.Infof("found rustc at %s via rustup", path)
				return path, nil
			}
		}
	}

	if path, err := exec.LookPath("rustc"); err == nil {
		logger.Infof("found rustc at %s via PATH", path)
		return path, nil
	}

	return "", fmt.Errorf("could not locate rustc (set RUSTC, install rustup, or put rustc on PATH)")
}
