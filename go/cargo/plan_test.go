package cargo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lczyk/assert"
)

func TestLoadPlanJSON_MissingFile(t *testing.T) {
	_, err := loadPlanJSON(filepath.Join(t.TempDir(), "does-not-exist.json"))
	assert.Error(t, err, assert.AnyError)
}

func TestLoadPlanJSON_BadJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "plan.json")
	assert.NoError(t, os.WriteFile(tmp, []byte("not json {"), 0o644))
	_, err := loadPlanJSON(tmp)
	assert.Error(t, err, assert.AnyError)
}

func TestLoadPlanJSON_NoInvocationsField(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "plan.json")
	assert.NoError(t, os.WriteFile(tmp, []byte(`{"other": []}`), 0o644))
	_, err := loadPlanJSON(tmp)
	assert.Error(t, err, "Cargo build plan")
}

func TestDecodeInvocations_RequiresPackageName(t *testing.T) {
	raw := []any{
		map[string]any{
			"package_version": "0.1.0",
			"program":         "rustc",
		},
	}
	_, err := decodeInvocations(raw)
	assert.Error(t, err, "package_name")
}

func TestDecodeInvocations_RequiresPackageVersion(t *testing.T) {
	raw := []any{
		map[string]any{
			"package_name": "foo",
			"program":      "rustc",
		},
	}
	_, err := decodeInvocations(raw)
	assert.Error(t, err, "package_version")
}

func TestDecodeInvocations_DefaultsCompileMode(t *testing.T) {
	raw := []any{
		map[string]any{
			"package_name":    "foo",
			"package_version": "0.1.0",
			"program":         "rustc",
		},
	}
	invs, err := decodeInvocations(raw)
	assert.NoError(t, err)
	assert.Equal(t, invs[0].CompileMode, "build")
}

func TestDecodeInvocations_RejectsNonObject(t *testing.T) {
	raw := []any{"not an object"}
	_, err := decodeInvocations(raw)
	assert.Error(t, err, "unexpected shape")
}
