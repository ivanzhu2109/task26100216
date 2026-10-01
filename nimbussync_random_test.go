package nimbussync

import (
	"math/rand"
	"testing"
)

type event struct {
	folder, parent, priority int
}

// referenceParents independently recomputes the displayed parents from an
// event multiset, following the spec rules directly:
//   - each folder's candidate is its highest-priority event (root if none);
//   - in every directed cycle of the candidate relation, exactly the
//     minimum-priority edge is redirected to the root; all other edges,
//     including edges merely entering a cycle, stay untouched.
func referenceParents(n int, events []event) []int {
	bestPrio := make([]int, n+1)
	has := make([]bool, n+1)
	cand := make([]int, n+1)
	for _, e := range events {
		if e.folder == 0 {
			continue
		}
		if !has[e.folder] || e.priority > bestPrio[e.folder] {
			has[e.folder] = true
			bestPrio[e.folder] = e.priority
			cand[e.folder] = e.parent
		}
	}

	// Collect every node that lies on a directed cycle by walking each
	// chain and detecting a repeat within the current walk.
	onCycle := make([]bool, n+1)
	for start := 1; start <= n; start++ {
		seen := map[int]int{} // node -> step index in this walk
		cur := start
		for step := 0; cur != 0; step++ {
			if idx, ok := seen[cur]; ok {
				// Nodes from idx onward form a cycle.
				node := cur
				for i := idx; i < step; i++ {
					onCycle[node] = true
					node = cand[node]
				}
				break
			}
			seen[cur] = step
			cur = cand[cur]
		}
	}

	// Cut the minimum-priority edge of each cycle; leave everything else.
	out := make([]int, n+1)
	copy(out, cand)
	visited := make([]bool, n+1)
	for f := 1; f <= n; f++ {
		if !onCycle[f] || visited[f] {
			continue
		}
		weak, weakPrio := f, bestPrio[f]
		for cur := cand[f]; cur != f; cur = cand[cur] {
			visited[cur] = true
			if bestPrio[cur] < weakPrio {
				weak, weakPrio = cur, bestPrio[cur]
			}
		}
		out[weak] = 0
	}
	return out
}

func genEvents(r *rand.Rand, n, count int) []event {
	events := make([]event, 0, count)
	prio := 1
	for i := 0; i < count; i++ {
		folder := 1 + r.Intn(n)
		parent := r.Intn(n + 1) // may be the folder itself (self move)
		events = append(events, event{folder, parent, prio})
		prio += 1 + r.Intn(3) // unique, not necessarily consecutive
	}
	return events
}

func TestRandomAgainstReference(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := 1 + r.Intn(6)
		events := genEvents(r, n, 1+r.Intn(30))

		s := New(n)
		for i, e := range events {
			s.Apply(e.folder, e.parent, e.priority)
			want := referenceParents(n, events[:i+1])
			for f := 0; f <= n; f++ {
				if got := s.Parent(f); got != want[f] {
					t.Fatalf("seed=%d after %d applies: Parent(%d)=%d, want %d",
						seed, i+1, f, got, want[f])
				}
			}
		}
	}
}

func TestRandomOrderAndDuplicatesConverge(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := 1 + r.Intn(6)
		events := genEvents(r, n, 5+r.Intn(20))
		want := referenceParents(n, events)

		for trial := 0; trial < 5; trial++ {
			tr := rand.New(rand.NewSource(seed*1000 + int64(trial)))
			// Shuffle, then inject random duplicate deliveries.
			perm := tr.Perm(len(events))
			var delivery []event
			for _, idx := range perm {
				delivery = append(delivery, events[idx])
				for tr.Intn(3) == 0 {
					delivery = append(delivery, events[tr.Intn(len(events))])
				}
			}
			s := New(n)
			for _, e := range delivery {
				s.Apply(e.folder, e.parent, e.priority)
			}
			snap := s.Snapshot()
			for f := 0; f <= n; f++ {
				if snap[f] != want[f] {
					t.Fatalf("seed=%d trial=%d: Snapshot()[%d]=%d, want %d",
						seed, trial, f, snap[f], want[f])
				}
			}
		}
	}
}
