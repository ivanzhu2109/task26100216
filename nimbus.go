// Package nimbussync implements an offline, in-memory directory move
// merge engine for NimbusSync.
//
// The folder set is fixed: folder 0 is the root, folders 1..n are
// regular folders. All state lives in memory and the API is
// single-threaded. Move events carry a globally unique priority
// (duplicates are exact re-deliveries); each folder keeps only the
// highest-priority event as its candidate move. The displayed tree
// breaks every directed cycle in the candidate graph by temporarily
// redirecting exactly the cycle's minimum-priority edge to the root.
package nimbussync

// Event is a single offline move event.
type Event struct {
	Folder   int
	Parent   int
	Priority int64
}

// Store holds the merged state for a fixed set of folders.
// Folder 0 is the root and can never be moved.
type Store struct {
	n    int
	best []Event // best[i] is the max-priority event seen for folder i
	seen []bool  // seen[i] reports whether folder i has any event
}

// New creates a Store for folders 0..n. All folders initially
// belong directly to the root.
func New(n int) *Store {
	if n < 0 {
		panic("nimbussync: negative folder count")
	}
	return &Store{
		n:    n,
		best: make([]Event, n+1),
		seen: make([]bool, n+1),
	}
}

// Apply ingests one move event. Events may arrive out of order
// and may be duplicated. The root can never be moved; moves of
// folder 0 are ignored.
func (s *Store) Apply(folder, parent int, priority int64) {
	if folder <= 0 || folder > s.n {
		return
	}
	if parent < 0 || parent > s.n {
		return
	}
	if !s.seen[folder] || priority > s.best[folder].Priority {
		s.best[folder] = Event{Folder: folder, Parent: parent, Priority: priority}
		s.seen[folder] = true
	}
}

// Parent returns the currently displayed parent of folder.
func (s *Store) Parent(folder int) int {
	if folder <= 0 || folder > s.n {
		return 0
	}
	return s.display()[folder]
}

// Snapshot returns the displayed parent of every folder 1..n
// as a map keyed by folder id. The result is always a single
// acyclic tree rooted at 0.
func (s *Store) Snapshot() map[int]int {
	disp := s.display()
	out := make(map[int]int, s.n)
	for f := 1; f <= s.n; f++ {
		out[f] = disp[f]
	}
	return out
}

// display computes the displayed parent of every folder by taking
// the candidate graph and cutting the minimum-priority edge of
// each directed cycle back to the root.
func (s *Store) display() []int {
	cand := make([]int, s.n+1)
	for f := 1; f <= s.n; f++ {
		if s.seen[f] {
			cand[f] = s.best[f].Parent
		}
	}

	// Find every cycle in the functional graph and record, for each
	// cycle, the folder owning its minimum-priority edge.
	cut := make([]bool, s.n+1)
	const (
		unvisited = 0
		visiting  = 1
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
			state[cur] = visiting
			path = append(path, cur)
			cur = cand[cur]
		}
		if cur != 0 && state[cur] == visiting {
			// cur is part of a cycle; walk it once.
			minFolder := cur
			for node := cand[cur]; ; node = cand[node] {
				if s.best[node].Priority < s.best[minFolder].Priority {
					minFolder = node
				}
				if node == cur {
					break
				}
			}
			cut[minFolder] = true
		}
		for _, node := range path {
			state[node] = done
		}
	}

	disp := make([]int, s.n+1)
	for f := 1; f <= s.n; f++ {
		if cut[f] {
			disp[f] = 0
		} else {
			disp[f] = cand[f]
		}
	}
	return disp
}
