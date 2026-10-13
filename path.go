package toml

import (
	"fmt"
	"strconv"
	"strings"
)

// Path is a compiled query over a [Node] tree, built by [PathString] or
// [PathPointer].
type Path struct {
	segments []pathSegment
	raw      string
}

type pathSegment interface {
	match(n *Node) []*Node
}

type keySegment struct {
	key string
}

func (s keySegment) match(n *Node) []*Node {
	var result []*Node
	for _, child := range n.Children {
		if len(child.Key) > 0 && child.Key[len(child.Key)-1] == s.key {
			result = append(result, child)
		}
	}
	return result
}

type indexSegment struct {
	index int
}

func (s indexSegment) match(n *Node) []*Node {
	if arr := arrayValue(n); arr != nil {
		n = arr
	}
	idx := s.index
	if idx < 0 {
		idx = len(n.Children) + idx
	}
	if idx >= 0 && idx < len(n.Children) {
		return []*Node{n.Children[idx]}
	}
	return nil
}

type wildcardSegment struct{}

func (s wildcardSegment) match(n *Node) []*Node {
	return n.Children
}

type recursiveSegment struct {
	key string
}

func (s recursiveSegment) match(n *Node) []*Node {
	var result []*Node
	Walk(n, func(node *Node) bool {
		if len(node.Key) > 0 && node.Key[len(node.Key)-1] == s.key {
			result = append(result, node)
		}
		return true
	})
	return result
}

// PathString compiles a path expression string into a [Path].
//
// Each segment selects from the children of the nodes matched so far:
//   - $.key — children whose key is key
//   - $.parent.child — nested key access
//   - [n] — the child at index n; a negative n counts from the end
//   - .* — all children
//   - $..key — nodes at any depth whose key is key
//   - ['key'] or ["key"] — children whose key is key, which may hold any
//     character but the quote, such as "." or "["
//
// A key matches a [KeyValueNode], whose only child is its value. Indexing a
// pair whose value is an array selects the array's elements, so $.arr[0] is
// the first element of arr and $.arr[0][1] the second element of that one.
// Indexing any other node selects among its children. An array of tables
// ([[name]]) is a run of [ArrayTableNode] siblings, so $.name matches every
// entry and $.name[i] the i-th child of each.
func PathString(expr string) (*Path, error) {
	if expr == "" || expr[0] != '$' {
		return nil, fmt.Errorf("%w: path must start with $", ErrPathSyntax)
	}

	p := &Path{raw: expr}
	rest := expr[1:]

	for rest != "" {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			if rest == "" {
				return nil, fmt.Errorf("%w: trailing dot", ErrPathSyntax)
			}
			switch rest[0] {
			case '.':
				rest = rest[1:]
				end := indexOfAny(rest, ".[")
				if end < 0 {
					end = len(rest)
				}
				key := rest[:end]
				if key == "" {
					return nil, fmt.Errorf("%w: empty key in recursive descent", ErrPathSyntax)
				}
				p.segments = append(p.segments, recursiveSegment{key: key})
				rest = rest[end:]
			case '*':
				p.segments = append(p.segments, wildcardSegment{})
				rest = rest[1:]
			default:
				end := indexOfAny(rest, ".[")
				if end < 0 {
					end = len(rest)
				}
				key := rest[:end]
				if key == "" {
					return nil, fmt.Errorf("%w: empty key", ErrPathSyntax)
				}
				p.segments = append(p.segments, keySegment{key: key})
				rest = rest[end:]
			}
		case '[':
			if key, after, ok, err := quotedKey(rest); ok {
				if err != nil {
					return nil, err
				}
				p.segments = append(p.segments, keySegment{key: key})
				rest = after
				continue
			}
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return nil, fmt.Errorf("%w: unclosed [", ErrPathSyntax)
			}
			idxStr := rest[1:end]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid index %q", ErrPathSyntax, idxStr)
			}
			p.segments = append(p.segments, indexSegment{index: idx})
			rest = rest[end+1:]
		default:
			return nil, fmt.Errorf("%w: unexpected character %q", ErrPathSyntax, rest[0])
		}
	}

	return p, nil
}

// PathPointer compiles an RFC 6901 JSON Pointer into a [Path]. The empty
// string selects the document root; any other pointer must begin with "/".
// Reference tokens are unescaped per RFC 6901 ("~1" → "/", then "~0" → "~").
//
// A JSON Pointer token can address either a table key or an array index — the
// document decides which — so each token resolves as a key first and, when it
// is a non-negative integer and no key matched, as an array index. (An array
// of tables written as [[name]] is addressed by its key; indexing across
// multiple [[name]] entries is not supported.)
//
// Returns [ErrPointerSyntax] (wrapped) when a non-empty pointer does not begin
// with "/".
func PathPointer(ptr string) (*Path, error) {
	p := &Path{raw: ptr}
	if ptr == "" {
		return p, nil
	}
	if ptr[0] != '/' {
		return nil, fmt.Errorf("%w: pointer must start with /", ErrPointerSyntax)
	}
	for tok := range strings.SplitSeq(ptr[1:], "/") {
		key := decodePointerToken(tok)
		seg := pointerSegment{key: key}
		if i, err := strconv.Atoi(key); err == nil && i >= 0 {
			seg.index, seg.hasIndex = i, true
		}
		p.segments = append(p.segments, seg)
	}
	return p, nil
}

// decodePointerToken applies the RFC 6901 unescaping rules to one reference
// token: "~1" → "/" then "~0" → "~" (order matters).
func decodePointerToken(tok string) string {
	tok = strings.ReplaceAll(tok, "~1", "/")
	tok = strings.ReplaceAll(tok, "~0", "~")
	return tok
}

// pointerSegment resolves a single RFC 6901 reference token. A JSON Pointer
// leaves array-vs-table resolution to the document, so the token matches a
// key first and falls back to an array index when it is numeric.
type pointerSegment struct {
	key      string
	index    int
	hasIndex bool
}

func (s pointerSegment) match(n *Node) []*Node {
	for _, child := range n.Children {
		if len(child.Key) > 0 && child.Key[len(child.Key)-1] == s.key {
			return []*Node{pointerValue(child)}
		}
	}
	if s.hasIndex && s.index >= 0 && s.index < len(n.Children) {
		return []*Node{pointerValue(n.Children[s.index])}
	}
	return nil
}

// arrayValue returns the array value of a key/value pair, or nil when n is
// not a pair holding an array.
func arrayValue(n *Node) *Node {
	if n.Kind == KeyValueNode && len(n.Children) == 1 && n.Children[0].Kind == ArrayNode {
		return n.Children[0]
	}
	return nil
}

// pointerValue unwraps a key/value pair to its value node so a subsequent
// pointer token descends into the value rather than the pair.
func pointerValue(n *Node) *Node {
	if n.Kind == KeyValueNode && len(n.Children) > 0 {
		return n.Children[0]
	}
	return n
}

// Read returns all nodes under n matching the path, or [ErrPathNotFound]
// when none do.
func (p *Path) Read(n *Node) ([]*Node, error) {
	if n == nil {
		return nil, ErrPathNotFound
	}

	current := []*Node{n}
	for _, seg := range p.segments {
		var next []*Node
		for _, node := range current {
			matches := seg.match(node)
			next = append(next, matches...)
		}
		if len(next) == 0 {
			return nil, ErrPathNotFound
		}
		current = next
	}
	return current, nil
}

// ReadString returns the [Node.Value] of the first matching node, or of its
// value node when that match is a key/value pair.
func (p *Path) ReadString(n *Node) (string, error) {
	nodes, err := p.Read(n)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", ErrPathNotFound
	}
	node := nodes[0]
	if node.Kind == KeyValueNode && len(node.Children) > 0 {
		return node.Children[0].Value, nil
	}
	return node.Value, nil
}

// ReadPositions returns the [Node.Pos] of every matching node.
func (p *Path) ReadPositions(n *Node) ([]Position, error) {
	nodes, err := p.Read(n)
	if err != nil {
		return nil, err
	}
	positions := make([]Position, len(nodes))
	for i, node := range nodes {
		positions[i] = node.Pos
	}
	return positions, nil
}

// Replace replaces the first matching node with the given replacement. When
// the match is a key/value pair's value, as a [PathPointer] key token
// selects, the value is replaced and the key kept.
func (p *Path) Replace(n, replacement *Node) error {
	parent, target, err := p.first(n)
	if err != nil {
		return err
	}
	owner, i := findOwner(parent, target)
	if owner == nil {
		return ErrPathNotFound
	}
	owner.Children[i] = replacement
	return nil
}

// Append adds a child node to the first matching node.
func (p *Path) Append(n, value *Node) error {
	nodes, err := p.Read(n)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return ErrPathNotFound
	}
	nodes[0].Children = append(nodes[0].Children, value)
	return nil
}

// Delete removes the first matching node. When the match is a key/value
// pair's value, as a [PathPointer] key token selects, the whole pair is
// removed.
func (p *Path) Delete(n *Node) error {
	parent, target, err := p.first(n)
	if err != nil {
		return err
	}
	owner, i := findOwner(parent, target)
	if owner == nil {
		return ErrPathNotFound
	}
	if owner.Kind == KeyValueNode {
		if pairOwner, j := findOwner(parent, owner); pairOwner != nil {
			owner, i = pairOwner, j
		}
	}
	owner.Children = append(owner.Children[:i], owner.Children[i+1:]...)
	return nil
}

// first follows the first match of each segment and returns the node the
// last segment was matched against and that segment's first match.
func (p *Path) first(n *Node) (parent, target *Node, err error) {
	if n == nil || len(p.segments) == 0 {
		return nil, nil, ErrPathNotFound
	}
	parent = n
	for _, seg := range p.segments[:len(p.segments)-1] {
		matches := seg.match(parent)
		if len(matches) == 0 {
			return nil, nil, ErrPathNotFound
		}
		parent = matches[0]
	}
	matches := p.segments[len(p.segments)-1].match(parent)
	if len(matches) == 0 {
		return nil, nil, ErrPathNotFound
	}
	return parent, matches[0], nil
}

// findOwner returns the node under root (root included) whose Children
// holds target, and target's index there, or nil if there is none.
func findOwner(root, target *Node) (*Node, int) {
	for i, child := range root.Children {
		if child == target {
			return root, i
		}
	}
	for _, child := range root.Children {
		if owner, i := findOwner(child, target); owner != nil {
			return owner, i
		}
	}
	return nil, 0
}

// String returns the original path expression.
func (p *Path) String() string {
	return p.raw
}

func indexOfAny(s, chars string) int {
	for i, ch := range s {
		if strings.ContainsRune(chars, ch) {
			return i
		}
	}
	return -1
}

// quotedKey reads a ['key'] or ["key"] segment at the start of s. It reports
// whether s starts one, and returns the key and the rest of s.
func quotedKey(s string) (key, rest string, ok bool, err error) {
	if len(s) < 2 || (s[1] != '\'' && s[1] != '"') {
		return "", s, false, nil
	}
	end := strings.IndexByte(s[2:], s[1])
	if end < 0 || 2+end+1 >= len(s) || s[2+end+1] != ']' {
		return "", s, true, fmt.Errorf("%w: unclosed quoted key", ErrPathSyntax)
	}
	return s[2 : 2+end], s[2+end+2:], true, nil
}
