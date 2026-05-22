package cargo

import (
	"math/rand"
	"testing"
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
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2}
	if got := numbers(got); !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveInvocationOrder_Diamond(t *testing.T) {
	// 0 -> 1, 0 -> 2, 1+2 -> 3
	in := []Invocation{mkInv(0), mkInv(1, 0), mkInv(2, 0), mkInv(3, 1, 2)}
	got, err := ResolveInvocationOrder(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := numbers(got); !equal(got, []int{0, 1, 2, 3}) {
		t.Errorf("got %v, want diamond order [0 1 2 3]", got)
	}
}

func TestResolveInvocationOrder_Cycle(t *testing.T) {
	in := []Invocation{mkInv(0, 1), mkInv(1, 0)}
	if _, err := ResolveInvocationOrder(in); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestResolveInvocationOrder_MissingDep(t *testing.T) {
	in := []Invocation{mkInv(0, 99)}
	if _, err := ResolveInvocationOrder(in); err == nil {
		t.Fatal("expected missing-dep error, got nil")
	}
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
	if err != nil {
		t.Fatal(err)
	}
	want := numbers(first)
	for i := 0; i < 50; i++ {
		shuffled := make([]Invocation, len(base))
		copy(shuffled, base)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got, err := ResolveInvocationOrder(shuffled)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if got := numbers(got); !equal(got, want) {
			t.Fatalf("iter %d: order %v, want %v", i, got, want)
		}
	}
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
