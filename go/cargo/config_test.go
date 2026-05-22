package cargo

import (
	"path/filepath"
	"testing"
)

// All cases set RUSTC to a fake path so findRustc returns immediately
// instead of trying to locate a real rustc on the test machine.
func TestNewConfig_DefaultsToCwdAndHomeCargo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "")
	t.Setenv("CARGO_HOME", "")
	// HOME is what UserHomeDir falls back to.
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := NewConfig(silentLogger{})
	if err != nil {
		t.Fatal(err)
	}
	// macOS may resolve /var to /private/var etc., so compare via EvalSymlinks.
	wantRoot, _ := filepath.EvalSymlinks(dir)
	gotRoot, _ := filepath.EvalSymlinks(cfg.ProjectRoot)
	if gotRoot != wantRoot {
		t.Errorf("ProjectRoot: got %q, want %q", gotRoot, wantRoot)
	}
	wantCargo := filepath.Join(home, ".cargo")
	if cfg.CargoHome != wantCargo {
		t.Errorf("CargoHome: got %q, want %q", cfg.CargoHome, wantCargo)
	}
	if cfg.RustcPath != "/fake/rustc" {
		t.Errorf("RustcPath: got %q, want /fake/rustc", cfg.RustcPath)
	}
}

func TestNewConfig_EnvOverrides(t *testing.T) {
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "/proj/explicit")
	t.Setenv("CARGO_HOME", "/cargo/explicit")

	cfg, err := NewConfig(silentLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectRoot != "/proj/explicit" {
		t.Errorf("PROJECT_ROOT env override ignored, got %q", cfg.ProjectRoot)
	}
	if cfg.CargoHome != "/cargo/explicit" {
		t.Errorf("CARGO_HOME env override ignored, got %q", cfg.CargoHome)
	}
}

func TestNewConfig_EmptyProjectRootFallsBackToCwd(t *testing.T) {
	// Explicit empty string should NOT override (matches the LookupEnv check).
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "")

	cfg, err := NewConfig(silentLogger{})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(cfg.ProjectRoot)
	if got != want {
		t.Errorf("expected cwd %q, got %q", want, got)
	}
}

func TestNewConfig_RustcMissingReturnsError(t *testing.T) {
	t.Setenv("RUSTC", "")
	// Empty PATH so exec.LookPath fails for both rustup and rustc.
	t.Setenv("PATH", "")
	// findRustc reads HOME indirectly via nothing here, but be safe.
	t.Setenv("HOME", t.TempDir())

	_, err := NewConfig(silentLogger{})
	if err == nil {
		t.Fatal("expected error when rustc cannot be located, got nil")
	}
}

type silentLogger struct{}

func (silentLogger) Infof(string, ...any) {}
func (silentLogger) Warnf(string, ...any) {}
