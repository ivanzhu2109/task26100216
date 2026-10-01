// Package nimbussync implements an offline directory move-merge library.
// All state is kept in memory and callers are single-threaded.
package nimbussync

// moveEvent is the winning (highest-priority) move event for one folder.
type moveEvent struct {
	parent   int
	priority int
}

// Sync tracks move events for a fixed set of folders and computes the
// currently displayed parent of each folder.
type Sync struct {
	n int
	// best[f] is the highest-priority event received for folder f.
	// has[f] is false while f has received no event (candidate parent 0).
	best []moveEvent
	has  []bool

	dirty     bool
	displayed []int
}

// New creates a Sync for folders 0..n. Folder 0 is the root and can never
// be moved. Every folder initially belongs directly to the root.
func New(n int) *Sync {
	s := &Sync{
		n:    n,
		best: make([]moveEvent, n+1),
		has:  make([]bool, n+1),
	}
	s.dirty = true
	return s
}

// Apply records one move event. Events may arrive out of order and may be
// delivered more than once; duplicates carry identical fields.
func (s *Sync) Apply(folder, parent, priority int) {
	if folder == 0 {
		return // the root can never be moved
	}
	if !s.has[folder] || priority > s.best[folder].priority {
		s.best[folder] = moveEvent{parent: parent, priority: priority}
		s.has[folder] = true
		s.dirty = true
	}
}

// Parent returns the currently displayed parent of folder.
func (s *Sync) Parent(folder int) int {
	s.refresh()
	return s.displayed[folder]
}

// Snapshot returns the currently displayed parent of every folder 0..n.
func (s *Sync) Snapshot() []int {
	s.refresh()
	out := make([]int, s.n+1)
	copy(out, s.displayed)
	return out
}

// candidateParent returns the parent implied by the folder's winning event,
// or the root when no event has been received for it.
func (s *Sync) candidateParent(folder int) int {
	if !s.has[folder] {
		return 0
	}
	return s.best[folder].parent
}

// refresh recomputes the displayed parents from the candidate relation:
// every directed cycle keeps all edges except its minimum-priority edge,
// which is temporarily redirected to the root. Edges outside cycles are
// never rewritten.
func (s *Sync) refresh() {
	if !s.dirty {
		return
	}
	s.dirty = false

	s.displayed = make([]int, s.n+1)
	for f := 1; f <= s.n; f++ {
		s.displayed[f] = s.candidateParent(f)
	}

	// The candidate relation is a functional graph over folders 1..n
	// (folder 0 has no outgoing edge, so cycles live entirely in 1..n).
	// Find each cycle via three-state coloring and cut its weakest edge.
	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	state := make([]int, s.n+1)
	for start := 1; start <= s.n; start++ {
		if state[start] != unvisited {
			continue
		}
		var path []int
		cur := start
		for cur != 0 && state[cur] == unvisited {
			state[cur] = inStack
			path = append(path, cur)
			cur = s.candidateParent(cur)
		}
		if cur != 0 && state[cur] == inStack {
			// The tail of path starting at cur forms a cycle.
			weak := -1
			weakPrio := 0
			inCycle := false
			for _, f := range path {
				if f == cur {
					inCycle = true
				}
				if !inCycle {
					continue
				}
				prio := s.best[f].priority
				if weak == -1 || prio < weakPrio {
					weak = f
					weakPrio = prio
				}
			}
			s.displayed[weak] = 0
		}
		for _, f := range path {
			state[f] = done
		}
	}
}
