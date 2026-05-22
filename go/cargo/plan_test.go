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
