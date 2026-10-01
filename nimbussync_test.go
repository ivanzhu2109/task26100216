package nimbussync

import "testing"

// Scenario from the spec: A<->B cycle, recovery, replay of stale and
// duplicate events, and a low-priority edge entering the cycle.
func TestScenarioCycleBreakAndRestore(t *testing.T) {
	s := New(3)

	s.Apply(1, 2, 10) // A -> B
	s.Apply(2, 1, 20) // B -> A, cycle {1->2 (p10), 2->1 (p20)}
	if got := s.Parent(1); got != 0 {
		t.Fatalf("weakest edge 1->2 must be cut: Parent(1)=%d, want 0", got)
	}
	if got := s.Parent(2); got != 1 {
		t.Fatalf("stronger edge 2->1 must stay: Parent(2)=%d, want 1", got)
	}

	s.Apply(2, 0, 30) // B moves back to root, breaking the cycle
	if got := s.Parent(1); got != 2 {
		t.Fatalf("suppressed move must re-activate: Parent(1)=%d, want 2", got)
	}
	if got := s.Parent(2); got != 0 {
		t.Fatalf("Parent(2)=%d, want 0", got)
	}

	// Stale and duplicate events must not regress anything.
	s.Apply(2, 1, 20) // stale: lower than 30
	s.Apply(1, 2, 10) // exact duplicate of current best
	s.Apply(2, 0, 30) // exact duplicate of current best
	if got := s.Parent(1); got != 2 {
		t.Fatalf("after stale/duplicate replay Parent(1)=%d, want 2", got)
	}
	if got := s.Parent(2); got != 0 {
		t.Fatalf("after stale/duplicate replay Parent(2)=%d, want 0", got)
	}

	// Recreate the cycle with a fresh higher-priority event.
	s.Apply(2, 1, 40)
	if got := s.Parent(1); got != 0 {
		t.Fatalf("cycle re-formed, weakest edge cut: Parent(1)=%d, want 0", got)
	}
	if got := s.Parent(2); got != 1 {
		t.Fatalf("Parent(2)=%d, want 1", got)
	}

	// A low-priority edge entering the cycle but not on it must not be cut.
	s.Apply(3, 1, 5)
	if got := s.Parent(3); got != 1 {
		t.Fatalf("edge into cycle must stay: Parent(3)=%d, want 1", got)
	}
	if got := s.Parent(1); got != 0 {
		t.Fatalf("cycle cut unchanged: Parent(1)=%d, want 0", got)
	}
	if got := s.Parent(2); got != 1 {
		t.Fatalf("Parent(2)=%d, want 1", got)
	}
}

// A folder moved into itself forms a cycle of length 1 and is shown at root.
func TestSelfLoop(t *testing.T) {
	s := New(2)
	s.Apply(1, 1, 7)
	if got := s.Parent(1); got != 0 {
		t.Fatalf("self-loop must be cut: Parent(1)=%d, want 0", got)
	}
	s.Apply(1, 2, 9) // real move replaces the self-loop candidate
	if got := s.Parent(1); got != 2 {
		t.Fatalf("Parent(1)=%d, want 2", got)
	}
}

// The root ignores every move event.
func TestRootNeverMoves(t *testing.T) {
	s := New(1)
	s.Apply(0, 1, 100)
	if got := s.Parent(0); got != 0 {
		t.Fatalf("Parent(0)=%d, want 0", got)
	}
	if got := s.Parent(1); got != 0 {
		t.Fatalf("Parent(1)=%d, want 0", got)
	}
}

// With no events every folder belongs to the root.
func TestInitialSnapshot(t *testing.T) {
	s := New(4)
	snap := s.Snapshot()
	if len(snap) != 5 {
		t.Fatalf("len(Snapshot())=%d, want 5", len(snap))
	}
	for f, p := range snap {
		if p != 0 {
			t.Fatalf("Snapshot()[%d]=%d, want 0", f, p)
		}
	}
}
