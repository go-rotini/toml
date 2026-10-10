package toml

import (
	"math"
	"slices"
	"unicode/utf8"
)

// Span is the source extent of a [Node] returned by [Parse]: Start is the
// position of its first byte and End the position just past its last byte,
// so data[n.Span.Start.Offset:n.Span.End.Offset] is the node's text exactly
// as written. Replacing that range with other text edits the node in place
// and leaves every other byte, comments and layout included, untouched.
//
// Span offsets index the bytes passed to [Parse], including a leading
// byte-order mark and the \r of each \r\n line ending; Line and Column are
// 1-based, with Column counted in characters.
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
		for i, b := range s.src {
			if b == '\n' {
				s.lineStarts = append(s.lineStarts, i+1)
			}
		}
	}
	line, found := slices.BinarySearch(s.lineStarts, off)
	if !found {
		line--
	}
	crs, _ := slices.BinarySearch(s.crlf, off)
	return Position{
		Line:   line + 1,
		Column: utf8.RuneCount(s.src[s.lineStarts[line]:off]) + 1,
		Offset: off + crs + s.bomLen,
	}
}
