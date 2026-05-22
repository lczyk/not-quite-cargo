package cargo

import (
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

// All cases set RUSTC to a fake path so findRustc returns immediately
// instead of trying to locate a real rustc on the test machine.
func TestNewConfig_DefaultsToCwdAndHomeCargo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "")
	t.Setenv("CARGO_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := NewConfig(silentLogger{})
	assert.NoError(t, err)
	// macOS may resolve /var to /private/var etc., so compare via EvalSymlinks.
	wantRoot, _ := filepath.EvalSymlinks(dir)
	gotRoot, _ := filepath.EvalSymlinks(cfg.ProjectRoot)
	assert.Equal(t, gotRoot, wantRoot, "ProjectRoot")
	assert.Equal(t, cfg.CargoHome, filepath.Join(home, ".cargo"), "CargoHome")
	assert.Equal(t, cfg.RustcPath, "/fake/rustc", "RustcPath")
}

func TestNewConfig_EnvOverrides(t *testing.T) {
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "/proj/explicit")
	t.Setenv("CARGO_HOME", "/cargo/explicit")

	cfg, err := NewConfig(silentLogger{})
	assert.NoError(t, err)
	assert.Equal(t, cfg.ProjectRoot, "/proj/explicit")
	assert.Equal(t, cfg.CargoHome, "/cargo/explicit")
}

func TestNewConfig_EmptyProjectRootFallsBackToCwd(t *testing.T) {
	// Explicit empty string should NOT override (matches the LookupEnv check).
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("RUSTC", "/fake/rustc")
	t.Setenv("PROJECT_ROOT", "")

	cfg, err := NewConfig(silentLogger{})
	assert.NoError(t, err)
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(cfg.ProjectRoot)
	assert.Equal(t, got, want)
}

func TestNewConfig_RustcMissingReturnsError(t *testing.T) {
	t.Setenv("RUSTC", "")
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	_, err := NewConfig(silentLogger{})
	assert.Error(t, err, assert.AnyError)
}

type silentLogger struct{}

func (silentLogger) Infof(string, ...any) {}
func (silentLogger) Warnf(string, ...any) {}
