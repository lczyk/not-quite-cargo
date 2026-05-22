package cargo

import (
	"fmt"
	"sort"
)

// ResolveInvocationOrder returns invs in dependency order.
//
// The traversal is deterministic: when several invocations are simultaneously
// ready, the one with the smallest Number is emitted first. Returns an error
// on circular or missing dependencies, rather than aborting the process.
func ResolveInvocationOrder(invs []Invocation) ([]Invocation, error) {
	known := make(map[int]struct{}, len(invs))
	for _, inv := range invs {
		known[inv.Number] = struct{}{}
	}
	for _, inv := range invs {
		for _, dep := range inv.Deps {
			if _, ok := known[dep]; !ok {
				return nil, fmt.Errorf("invocation %d depends on unknown invocation %d", inv.Number, dep)
			}
		}
	}

	pending := make([]Invocation, len(invs))
	copy(pending, invs)
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].Number < pending[j].Number })

	satisfied := make(map[int]bool, len(invs))
	ordered := make([]Invocation, 0, len(invs))

	for len(pending) > 0 {
		idx := -1
		for i, inv := range pending {
			ready := true
			for _, dep := range inv.Deps {
				if !satisfied[dep] {
					ready = false
					break
				}
			}
			if ready {
				idx = i
				break
			}
		}
		if idx == -1 {
			remaining := make([]int, len(pending))
			for i, inv := range pending {
				remaining[i] = inv.Number
			}
			return nil, fmt.Errorf("circular dependency among invocations %v", remaining)
		}
		ordered = append(ordered, pending[idx])
		satisfied[pending[idx].Number] = true
		pending = append(pending[:idx], pending[idx+1:]...)
	}
	return ordered, nil
}
