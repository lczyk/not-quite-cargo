package cargo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to a temp file in the destination directory then
// renames over the target. If the process is interrupted mid-write the
// original file is preserved. Used by the --inplace patch path.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".nqc-patch-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// Invocation is a single build step from a Cargo build plan.
type Invocation struct {
	Number         int               `json:"-"`
	PackageName    string            `json:"package_name"`
	PackageVersion string            `json:"package_version"`
	TargetKind     []string          `json:"target_kind"`
	Kind           *string           `json:"kind"`
	CompileMode    string            `json:"compile_mode"`
	Deps           []int             `json:"deps"`
	Outputs        []string          `json:"outputs"`
	Links          map[string]string `json:"links"`
	Program        string            `json:"program"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	Cwd            string            `json:"cwd"`
}

// LoadPlanJSON reads and parses a build plan into a generic map for patching.
// `run` uses this too; the typed Invocation struct is only built after string
// replacements have been applied.
func LoadPlanJSON(path string) (map[string]any, error) {
	return loadPlanJSON(path)
}

func loadPlanJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read build plan: %w", err)
	}
	var plan map[string]any
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse build plan: %w", err)
	}
	if _, ok := plan["invocations"].([]any); !ok {
		return nil, fmt.Errorf("%s does not look like a Cargo build plan (no invocations array)", path)
	}
	return plan, nil
}

// decodeInvocations converts the generic invocations slice into typed Invocations.
func decodeInvocations(raw []any) ([]Invocation, error) {
	invs := make([]Invocation, 0, len(raw))
	for i, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invocation %d has unexpected shape", i)
		}
		blob, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("re-marshal invocation %d: %w", i, err)
		}
		var inv Invocation
		if err := json.Unmarshal(blob, &inv); err != nil {
			return nil, fmt.Errorf("decode invocation %d: %w", i, err)
		}
		if _, ok := obj["package_name"].(string); !ok {
			return nil, fmt.Errorf("invocation %d: missing or non-string field 'package_name'", i)
		}
		if _, ok := obj["package_version"].(string); !ok {
			return nil, fmt.Errorf("invocation %d: missing or non-string field 'package_version'", i)
		}
		if inv.CompileMode == "" {
			inv.CompileMode = "build"
		}
		inv.Number = i
		invs = append(invs, inv)
	}
	return invs, nil
}
