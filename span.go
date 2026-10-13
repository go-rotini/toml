package toml

import (
	"errors"
	"math"
	"slices"
)

// Span is the source extent of a [Node] returned by [Parse]: Start is the
// position of its first byte and End the position just past its last byte,
// so data[n.Span.Start.Offset:n.Span.End.Offset] is the node's text exactly
// as written. Replacing that range with other text edits the node in place
// and leaves every other byte, comments and layout included, untouched.
//
// Span offsets, [Node.Pos] offsets and error positions such as
// [SyntaxError.Pos] all index the bytes passed to [Parse], including a
// leading byte-order mark and the \r of each \r\n line ending; Line and
// Column are 1-based, with Column counted in characters. They differ in
// what they mark: [Node.Pos] marks where a node's token ends, and an error
// position marks where the offending key or token begins, or where
// scanning stopped in a malformed one.
//
// What a node's Span covers depends on its kind:
//   - a scalar: the literal with its quotes or delimiters, all lines of a
//     multi-line string included;
//   - an [ArrayNode]: from "[" through "]", or "{" through "}" for an
//     [InlineTableNode];
//   - a [KeyValueNode]: from the first byte of the key to the last byte of
//     the value, without a trailing comment or the line ending;
//   - a [TableNode] or [ArrayTableNode]: the header line's "[...]" or
//     "[[...]]", not the keys under it.
//
// A node with no source text of its own, such as the root table or a table
// implied by a dotted key or a nested header, has a zero Span. So does every
// node of a document larger than 2 GiB.
//
// Spans describe the input to [Parse]; they are not updated when a node is
// modified or re-encoded.
type Span struct {
	Start Position
	End   Position
}

// IsZero reports whether the span is empty, as it is for a node with no
// source text of its own or one built in code.
func (s Span) IsZero() bool {
	return s == Span{}
}

// srcSpan is a node's extent as byte offsets in the scanner's normalized
// source; [scanner.spanOf] turns it into a [Span].
type srcSpan struct {
	start, end int
}

func tokenSpan(t token) srcSpan {
	return srcSpan{start: int(t.start), end: t.pos.Offset}
}

// keyPathSpan covers a key as written, from its first segment to its last,
// dots and quotes included.
func keyPathSpan(keys []token) srcSpan {
	return srcSpan{start: int(keys[0].start), end: keys[len(keys)-1].pos.Offset}
}

// spanOf converts sp into positions in the bytes the caller passed in.
func (s *scanner) spanOf(sp srcSpan) Span {
	if sp == (srcSpan{}) || len(s.src) > math.MaxInt32 {
		return Span{}
	}
	return Span{Start: s.inputPosition(sp.start), End: s.inputPosition(sp.end)}
}

// inputPosition returns the position of off, an offset in the normalized
// source, with Offset mapped back to the input: the byte-order mark and the
// \r of each \r\n before it restored.
func (s *scanner) inputPosition(off int) Position {
	if s.lineStarts == nil {
		s.lineStarts = []int{0}
		s.chunkRunes = make([]int, len(s.src)/runeChunk+1)
		runes := 0
		for i, b := range s.src {
			if i%runeChunk == 0 {
				s.chunkRunes[i/runeChunk] = runes
			}
			if b == '\n' {
				s.lineStarts = append(s.lineStarts, i+1)
			}
			if b&0xC0 != 0x80 {
				runes++
			}
		}
		if len(s.src)%runeChunk == 0 {
			s.chunkRunes[len(s.src)/runeChunk] = runes
		}
	}
	line, found := slices.BinarySearch(s.lineStarts, off)
	if !found {
		line--
	}
	return Position{
		Line:   line + 1,
		Column: s.runesBetween(s.lineStarts[line], off) + 1,
		Offset: s.inputOffset(off),
	}
}

// inputOffset maps off, an offset in the normalized source, back to the
// input: the byte-order mark and the \r of each \r\n before it restored.
func (s *scanner) inputOffset(off int) int {
	crs, _ := slices.BinarySearch(s.crlf, off)
	return off + crs + s.bomLen
}

// mapsOffsets reports whether offsets in the normalized source differ from
// offsets in the input.
func (s *scanner) mapsOffsets() bool {
	return s.bomLen > 0 || len(s.crlf) > 0
}

// inputPositions maps the Offset of every position in the tree at n to
// the input. Line and Column need no mapping.
func (s *scanner) inputPositions(n *node) {
	n.pos.Offset = s.inputOffset(n.pos.Offset)
	for _, c := range n.children {
		s.inputPositions(c)
	}
}

// inputErrorPos maps the Offset of a positioned error from the scanner or
// parser to the input.
func (s *scanner) inputErrorPos(err error) error {
	var pos *Position
	if e, ok := errors.AsType[*SyntaxError](err); ok {
		pos = &e.Pos
	} else if e, ok := errors.AsType[*DuplicateKeyError](err); ok {
		pos = &e.Pos
	} else if e, ok := errors.AsType[*ConflictError](err); ok {
		pos = &e.Pos
	} else if e, ok := errors.AsType[*InlineTableError](err); ok {
		pos = &e.Pos
	}
	if pos != nil && pos.Line > 0 {
		pos.Offset = s.inputOffset(pos.Offset)
	}
	return err
}

// runeChunk is the stride of the rune counts inputPosition records, so a
// column costs at most two short counts however long its line is.
const runeChunk = 1024

// runesBefore returns the number of characters in src[:off]. A source that
// parsed is valid UTF-8, so each byte that is not a continuation byte starts
// one character.
func (s *scanner) runesBefore(off int) int {
	c := off / runeChunk
	n := s.chunkRunes[c]
	for _, b := range s.src[c*runeChunk : off] {
		if b&0xC0 != 0x80 {
			n++
		}
	}
	return n
}

// runesBetween returns the number of characters in src[from:to].
func (s *scanner) runesBetween(from, to int) int {
	if to-from > runeChunk {
		return s.runesBefore(to) - s.runesBefore(from)
	}
	n := 0
	for _, b := range s.src[from:to] {
		if b&0xC0 != 0x80 {
			n++
		}
	}
	return n
}
