package reflex

const gapStackSize = 8

// Pos is assumed to be defined elsewhere in this package.
// type Pos struct { ... }

type GapStack struct {
	prefix [gapStackSize]Pos
	suffix [gapStackSize]Pos
	size   int // logical size: prefix + middle + suffix
}

// NewGapStack returns an empty GapStack.
func NewGapStack() GapStack {
	return GapStack{}
}

// Len returns the logical number of elements pushed so far
// (including those omitted in the middle).
func (g *GapStack) Len() int {
	return g.size
}

// Push adds a new position to the top of the logical stack.
// It fills prefix first; once prefix is full, it writes into the
// suffix ring buffer at index size % gapStackSize, overwriting
// older suffix entries as needed.
func (g *GapStack) Push(p Pos) {
	if g.size < gapStackSize {
		// Still filling the prefix.
		g.prefix[g.size] = p
	} else {
		// In the suffix region: use ring-buffer indexing.
		idx := g.size % gapStackSize
		g.suffix[idx] = p
	}
	g.size++
}

// Slice returns the visible contents of the stack as a slice:
//
//   - Up to gapStackSize prefix elements from the bottom.
//   - Optionally a zero Pos{} sentinel if there is an omitted middle.
//   - Up to gapStackSize suffix elements from the top.
//
// The sentinel appears iff the middle (omitted) region is non-empty.
func (g *GapStack) Slice() []Pos {
	if g.size <= 0 {
		return nil
	}

	// Compute lengths.
	prefixLen := g.prefixLen()
	suffixLen := g.suffixLen()
	middleLen := g.size - prefixLen - suffixLen

	// At most 2*gapStackSize visible + 1 sentinel.
	out := make([]Pos, 0, prefixLen+suffixLen+1)

	// Prefix: always in order from bottom.
	if prefixLen > 0 {
		out = append(out, g.prefix[:prefixLen]...)
	}

	// Sentinel if middle is omitted.
	if middleLen > 0 {
		out = append(out, Pos{}) // zero sentinel
	}

	// Suffix: we reconstruct from logical indices using the ring mapping.
	if suffixLen > 0 {
		// Logical index of the bottom of the suffix.
		bottomLogical := g.size - suffixLen
		for i := 0; i < suffixLen; i++ {
			logicalIdx := bottomLogical + i
			physIdx := logicalIdx % gapStackSize
			out = append(out, g.suffix[physIdx])
		}
	}

	return out
}

// prefixLen returns how many elements are currently in the prefix window.
func (g *GapStack) prefixLen() int {
	if g.size < gapStackSize {
		return g.size
	}
	return gapStackSize
}

// suffixLen returns how many elements are currently in the suffix window.
func (g *GapStack) suffixLen() int {
	if g.size <= gapStackSize {
		return 0
	}
	remaining := g.size - gapStackSize
	if remaining > gapStackSize {
		return gapStackSize
	}
	return remaining
}
