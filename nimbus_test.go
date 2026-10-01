package nimbussync

import (
	"math/rand"
	"reflect"
	"testing"
)

// reference recomputes the displayed parents from scratch using a
// deliberately simple, independent implementation of the spec.
func reference(n int, events []Event) map[int]int {
	best := make(map[int]Event)
	for _, e := range events {
		if e.Folder <= 0 || e.Folder > n {
			continue
		}
		if cur, ok := best[e.Folder]; !ok || e.Priority > cur.Priority {
			best[e.Folder] = e
		}
	}
	cand := make([]int, n+1)
	for f := 1; f <= n; f++ {
		if e, ok := best[f]; ok {
			cand[f] = e.Parent
		}
	}
	disp := make([]int, n+1)
	copy(disp, cand)

	// Find cycles by walking from every node with a local visited set.
	seenCycle := make(map[int]bool) // keyed by smallest member id
	for s := 1; s <= n; s++ {
		pos := make(map[int]int)
		var path []int
		cur := s
		for cur != 0 {
			if idx, ok := pos[cur]; ok {
				cycle := append([]int(nil), path[idx:]...)
				minMember := cycle[0]
				for _, m := range cycle[1:] {
					if m < minMember {
						minMember = m
					}
				}
				if !seenCycle[minMember] {
					seenCycle[minMember] = true
					cut := cycle[0]
					for _, m := range cycle[1:] {
						if best[m].Priority < best[cut].Priority {
							cut = m
						}
					}
					disp[cut] = 0
				}
				break
			}
			pos[cur] = len(path)
			path = append(path, cur)
			cur = cand[cur]
		}
	}

	out := make(map[int]int, n)
	for f := 1; f <= n; f++ {
		out[f] = disp[f]
	}
	return out
}

func assertTree(t *testing.T, snap map[int]int) {
	t.Helper()
	for f := range snap {
		seen := map[int]bool{}
		cur := f
		for cur != 0 {
			if seen[cur] {
				t.Fatalf("folder %d: cycle detected at %d", f, cur)
			}
			seen[cur] = true
			p, ok := snap[cur]
			if !ok {
				t.Fatalf("folder %d: parent chain leaves folder set at %d", f, cur)
			}
			cur = p
		}
	}
}

func TestCycleBreakAndRestore(t *testing.T) {
	const (
		A = 1
		B = 2
		C = 3
	)
	s := New(3)

	// Increasing priority: A->B then B->A forms a 2-cycle.
	s.Apply(A, B, 10)
	s.Apply(B, A, 20)
	if got := s.Parent(A); got != 0 {
		t.Fatalf("A should be temporarily shown under root, got parent %d", got)
	}
	if got := s.Parent(B); got != A {
		t.Fatalf("B should keep candidate parent A, got %d", got)
	}

	// B moves back to root; A's suppressed move must re-activate.
	s.Apply(B, 0, 30)
	if got := s.Parent(A); got != B {
		t.Fatalf("A should be restored under B, got %d", got)
	}
	if got := s.Parent(B); got != 0 {
		t.Fatalf("B should be under root, got %d", got)
	}

	// Old and duplicate events must not roll anything back.
	s.Apply(B, A, 20) // stale event, lower priority than 30
	s.Apply(A, B, 10) // duplicate of an old event
	s.Apply(A, B, 10) // duplicate again
	if got := s.Parent(A); got != B {
		t.Fatalf("after stale/duplicate events A should stay under B, got %d", got)
	}
	if got := s.Parent(B); got != 0 {
		t.Fatalf("after stale/duplicate events B should stay under root, got %d", got)
	}

	// Recreate the A<->B cycle with fresh higher priorities.
	s.Apply(A, B, 40)
	s.Apply(B, A, 50)
	if got := s.Parent(A); got != 0 {
		t.Fatalf("A (min edge) should be cut to root, got %d", got)
	}
	if got := s.Parent(B); got != A {
		t.Fatalf("B should keep candidate parent A, got %d", got)
	}

	// A low-priority edge entering the cycle but not on it (C->A)
	// must not be cut.
	s.Apply(C, A, 5)
	if got := s.Parent(C); got != A {
		t.Fatalf("C is not on the cycle and must keep parent A, got %d", got)
	}
	if got := s.Parent(A); got != 0 {
		t.Fatalf("A should still be the cut edge, got %d", got)
	}
	assertTree(t, s.Snapshot())
}

func TestSelfLoop(t *testing.T) {
	s := New(2)
	s.Apply(1, 1, 7) // self loop: cycle of length 1
	if got := s.Parent(1); got != 0 {
		t.Fatalf("self-loop must be cut to root, got %d", got)
	}
	s.Apply(2, 1, 8) // 2 hangs under 1, not on the cycle
	if got := s.Parent(2); got != 1 {
		t.Fatalf("folder 2 should stay under 1, got %d", got)
	}
	s.Apply(1, 2, 9) // self loop replaced by 1->2, no cycle remains
	if got := s.Parent(1); got != 2 {
		t.Fatalf("folder 1 should move under 2, got %d", got)
	}
	assertTree(t, s.Snapshot())
}

func TestRandomAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const n = 6
	const numEvents = 60

	prios := rng.Perm(numEvents)
	var events []Event
	for i := 0; i < numEvents; i++ {
		events = append(events, Event{
			Folder:   1 + rng.Intn(n),
			Parent:   rng.Intn(n + 1),
			Priority: int64(prios[i] + 1),
		})
	}

	s := New(n)
	var applied []Event
	for i, e := range events {
		s.Apply(e.Folder, e.Parent, e.Priority)
		applied = append(applied, e)
		want := reference(n, applied)
		for f := 1; f <= n; f++ {
			if got := s.Parent(f); got != want[f] {
				t.Fatalf("after event %d %+v: Parent(%d)=%d, reference=%d", i, e, f, got, want[f])
			}
		}
	}
	assertTree(t, s.Snapshot())
}

func TestOrderAndDuplicateInvariance(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const n = 8
	const numEvents = 50

	prios := rng.Perm(numEvents)
	base := make([]Event, numEvents)
	for i := range base {
		base[i] = Event{
			Folder:   1 + rng.Intn(n),
			Parent:   rng.Intn(n + 1),
			Priority: int64(prios[i] + 1),
		}
	}

	want := reference(n, base)

	for trial := 0; trial < 20; trial++ {
		order := rng.Perm(numEvents)
		s := New(n)
		for _, idx := range order {
			e := base[idx]
			s.Apply(e.Folder, e.Parent, e.Priority)
			// Randomly re-deliver some events as duplicates.
			for rng.Intn(3) == 0 {
				d := base[rng.Intn(numEvents)]
				s.Apply(d.Folder, d.Parent, d.Priority)
			}
		}
		got := s.Snapshot()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trial %d: snapshot mismatch\ngot:  %v\nwant: %v", trial, got, want)
		}
		assertTree(t, got)
	}
}
