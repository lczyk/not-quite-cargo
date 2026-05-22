package cargo

import (
	"encoding/json"
	"fmt"
	"os"
)

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

// loadPlanJSON reads and parses a build plan into a generic map for patching.
// `run` uses this too; the typed Invocation struct is only built after string
// replacements have been applied.
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
		blob, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("re-marshal invocation %d: %w", i, err)
		}
		var inv Invocation
		if err := json.Unmarshal(blob, &inv); err != nil {
			return nil, fmt.Errorf("decode invocation %d: %w", i, err)
		}
		inv.Number = i
		invs = append(invs, inv)
	}
	return invs, nil
}
