package reflex

import (
	"encoding/binary"
	"hash/maphash"
	"sort"
	"sync/atomic"
)

type BackEdgeID uint64
type BackEdgeSet struct {
	Set map[BackEdgeID]struct{}
}

func (b *BackEdgeSet) matches(edges []BackEdgeID) bool {
	if b == nil {
		return false
	}
	if len(edges) != len(b.Set) {
		return false
	}
	for _, x := range edges {
		if _, ok := b.Set[x]; !ok {
			return false
		}
	}
	return true
}

func (b *BackEdgeSet) Contains(id BackEdgeID) bool {
	if b == nil {
		return false
	}
	_, ok := b.Set[id]
	return ok
}

type BackEdges struct {
	counter atomic.Uint64

	// Custom hashmap so that we can hash sets of back edge IDs
	seed maphash.Seed
	sets map[uint64][]*BackEdgeSet

	merges map[[2]*BackEdgeSet]*BackEdgeSet
}

// NewBackEdges creates a new ID space of back edges and cache of back edge sets.
func NewBackEdges() *BackEdges {
	return &BackEdges{
		sets:   map[uint64][]*BackEdgeSet{},
		merges: map[[2]*BackEdgeSet]*BackEdgeSet{},
		seed:   maphash.MakeSeed(),
	}
}

// MakeEdge creates a new, unique back edge.
func (b *BackEdges) MakeEdgeID() BackEdgeID {
	return BackEdgeID(b.counter.Add(1))
}

// MakeSet creates or returns a unque set for the back edges.
func (b *BackEdges) MakeSet(edges ...BackEdgeID) *BackEdgeSet {
	sorted := append([]BackEdgeID{}, edges...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return b.makeSetSorted(sorted)
}

// MakeSetMap creates or returns a unque set for the back edges.
func (b *BackEdges) MakeSetMap(e map[BackEdgeID]struct{}) *BackEdgeSet {
	edges := make([]BackEdgeID, 0, len(e))
	for x := range e {
		edges = append(edges, x)
	}
	return b.MakeSet(edges...)
}

func (b *BackEdges) makeSetSorted(sorted []BackEdgeID) *BackEdgeSet {
	var hash maphash.Hash
	hash.SetSeed(b.seed)
	for _, id := range sorted {
		var x [8]byte
		binary.LittleEndian.PutUint64(x[:], uint64(id))
		hash.Write(x[:])
	}
	sum := hash.Sum64()
	for _, set := range b.sets[sum] {
		if set.matches(sorted) {
			return set
		}
	}

	newSet := &BackEdgeSet{Set: map[BackEdgeID]struct{}{}}
	for _, x := range sorted {
		newSet.Set[x] = struct{}{}
	}
	if len(newSet.Set) != len(sorted) {
		panic("duplicate BackEdgeID passed to MakeSet")
	}
	b.sets[sum] = append(b.sets[sum], newSet)
	return newSet
}

// Merge creates a new, unique BackEdgeSet containing the union of s1 and s2.
func (b *BackEdges) Merge(s1, s2 *BackEdgeSet) *BackEdgeSet {
	if result, ok := b.merges[[2]*BackEdgeSet{s1, s2}]; ok {
		return result
	}
	if result, ok := b.merges[[2]*BackEdgeSet{s2, s1}]; ok {
		return result
	}
	allEdges := make([]BackEdgeID, 0, len(s1.Set)+len(s2.Set))
	contains := map[BackEdgeID]bool{}
	for _, s := range []*BackEdgeSet{s1, s2} {
		for x := range s.Set {
			if !contains[x] {
				allEdges = append(allEdges, x)
				contains[x] = true
			}
		}
	}
	result := b.MakeSet(allEdges...)
	b.merges[[2]*BackEdgeSet{s1, s2}] = result
	return result
}
