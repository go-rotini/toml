package toml

import (
	"fmt"
	"strconv"
	"strings"
)

// Path represents a compiled path expression for querying TOML AST nodes.
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
// Syntax:
//   - $.key — root child access
//   - $.parent.child — nested key access
//   - $.array[0] — array index access
//   - $.array[-1] — negative index (from end)
//   - $.table.* — wildcard (all children)
//   - $..key — recursive descent
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

// Read returns all nodes matching the path expression.
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

// ReadString reads a single scalar node matching the path and returns its value.
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

// ReadPositions returns the positions of all matching nodes.
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

// Replace replaces the first matching node with the given replacement.
func (p *Path) Replace(n, replacement *Node) error {
	if len(p.segments) == 0 {
		return ErrPathNotFound
	}

	// Navigate to the parent of the target
	parent := n
	for _, seg := range p.segments[:len(p.segments)-1] {
		matches := seg.match(parent)
		if len(matches) == 0 {
			return ErrPathNotFound
		}
		parent = matches[0]
	}

	lastSeg := p.segments[len(p.segments)-1]
	matches := lastSeg.match(parent)
	if len(matches) == 0 {
		return ErrPathNotFound
	}

	target := matches[0]
	for i, child := range parent.Children {
		if child == target {
			parent.Children[i] = replacement
			return nil
		}
	}
	return ErrPathNotFound
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

// Delete removes the first matching node.
func (p *Path) Delete(n *Node) error {
	if len(p.segments) == 0 {
		return ErrPathNotFound
	}

	parent := n
	for _, seg := range p.segments[:len(p.segments)-1] {
		matches := seg.match(parent)
		if len(matches) == 0 {
			return ErrPathNotFound
		}
		parent = matches[0]
	}

	lastSeg := p.segments[len(p.segments)-1]
	matches := lastSeg.match(parent)
	if len(matches) == 0 {
		return ErrPathNotFound
	}

	target := matches[0]
	for i, child := range parent.Children {
		if child == target {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return nil
		}
	}
	return ErrPathNotFound
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
