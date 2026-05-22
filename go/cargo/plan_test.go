package cargo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPlanJSON_MissingFile(t *testing.T) {
	_, err := loadPlanJSON(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadPlanJSON_BadJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(tmp, []byte("not json {"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadPlanJSON(tmp)
	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
}

func TestLoadPlanJSON_NoInvocationsField(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(tmp, []byte(`{"other": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadPlanJSON(tmp)
	if err == nil {
		t.Fatal("expected error for missing invocations, got nil")
	}
	if !strings.Contains(err.Error(), "Cargo build plan") {
		t.Errorf("error message should mention build plan, got %q", err.Error())
	}
}
