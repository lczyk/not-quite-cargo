package cargo

import (
	"math/rand"
	"testing"

	"github.com/lczyk/assert"
)

func mkInv(num int, deps ...int) Invocation {
	return Invocation{Number: num, Deps: deps}
}

func numbers(invs []Invocation) []int {
	out := make([]int, len(invs))
	for i, inv := range invs {
		out[i] = inv.Number
	}
	return out
}

func TestResolveInvocationOrder_Linear(t *testing.T) {
	in := []Invocation{mkInv(0), mkInv(1, 0), mkInv(2, 1)}
	got, err := ResolveInvocationOrder(in)
	assert.NoError(t, err)
	assert.EqualArrays(t, numbers(got), []int{0, 1, 2})
}

func TestResolveInvocationOrder_Diamond(t *testing.T) {
	// 0 -> 1, 0 -> 2, 1+2 -> 3
	in := []Invocation{mkInv(0), mkInv(1, 0), mkInv(2, 0), mkInv(3, 1, 2)}
	got, err := ResolveInvocationOrder(in)
	assert.NoError(t, err)
	assert.EqualArrays(t, numbers(got), []int{0, 1, 2, 3})
}

func TestResolveInvocationOrder_Cycle(t *testing.T) {
	in := []Invocation{mkInv(0, 1), mkInv(1, 0)}
	_, err := ResolveInvocationOrder(in)
	assert.Error(t, err, assert.AnyError)
}

func TestResolveInvocationOrder_MissingDep(t *testing.T) {
	in := []Invocation{mkInv(0, 99)}
	_, err := ResolveInvocationOrder(in)
	assert.Error(t, err, assert.AnyError)
}

func TestResolveInvocationOrder_Deterministic(t *testing.T) {
	// Regression: previous implementation iterated a map, so when multiple
	// invocations were simultaneously ready the order could vary. Shuffle the
	// input N times and assert the output is identical every run.
	base := []Invocation{
		mkInv(0), mkInv(1), mkInv(2), mkInv(3), mkInv(4),
		mkInv(5, 0, 1), mkInv(6, 2, 3), mkInv(7, 4),
		mkInv(8, 5, 6, 7),
	}
	rng := rand.New(rand.NewSource(42))
	first, err := ResolveInvocationOrder(base)
	assert.NoError(t, err)
	want := numbers(first)
	for i := 0; i < 50; i++ {
		shuffled := make([]Invocation, len(base))
		copy(shuffled, base)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got, err := ResolveInvocationOrder(shuffled)
		assert.NoError(t, err, "iter %d", i)
		assert.EqualArrays(t, numbers(got), want, "iter %d", i)
	}
}
