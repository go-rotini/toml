package toml

import (
	"fmt"
	"strings"
)

type nodeKind int

const (
	nodeTable nodeKind = iota
	nodeArrayTable
	nodeKeyValue
	nodeArray
	nodeInlineTable
	nodeString
	nodeInteger
	nodeFloat
	nodeBoolean
	nodeDateTime
	nodeLocalDateTime
	nodeLocalDate
	nodeLocalTime
	nodeComment
)

type stringStyle int

const (
	styleBasic stringStyle = iota
	styleMultiLineBasic
	styleLiteral
	styleMultiLineLiteral
)

type node struct {
	kind        nodeKind
	key         []string
	value       string
	style       stringStyle
	children    []*node
	pos         Position
	comment     string
	headComment string
	footComment string
}

// NodeKind identifies the type of a TOML [Node] in the AST.
type NodeKind int

const (
	TableNode         NodeKind = iota // a TOML table ([table])
	ArrayTableNode                    // an array of tables ([[array]])
	KeyValueNode                      // a key/value pair
	ArrayNode                         // an array value
	InlineTableNode                   // an inline table ({...})
	StringNode                        // a string scalar
	IntegerNode                       // an integer scalar
	FloatNode                         // a float scalar
	BooleanNode                       // a boolean scalar
	DateTimeNode                      // an offset date-time
	LocalDateTimeNode                 // a local date-time
	LocalDateNode                     // a local date
	LocalTimeNode                     // a local time
	CommentNode                       // a comment
)

// StringStyle indicates how a string was (or should be) represented in TOML.
type StringStyle int

const (
	BasicStyle           StringStyle = iota // double-quoted ("...")
	MultiLineBasicStyle                     // triple double-quoted ("""...""")
	LiteralStyle                            // single-quoted ('...')
	MultiLineLiteralStyle                   // triple single-quoted ('''...''')
)

// Node is a TOML AST node.
type Node struct {
	Kind        NodeKind
	Key         []string    // key path segments for KeyValueNode/TableNode/ArrayTableNode
	Value       string      // raw string representation of the value (for scalar nodes)
	Style       StringStyle // for StringNode: which string style was used
	Inline      bool        // for tables: inline table format ({...})
	Children    []*Node     // child nodes
	Pos         Position
	Comment     string // line comment (after value on same line)
	HeadComment string // comment block before this node
	FootComment string // comment block after this node
}

// String returns the dotted key path for table/key-value nodes, or the
// value for scalar nodes.
func (n *Node) String() string {
	switch n.Kind {
	case TableNode, ArrayTableNode, KeyValueNode:
		return strings.Join(n.Key, ".")
	case ArrayNode:
		return fmt.Sprintf("[array: %d items]", len(n.Children))
	case InlineTableNode:
		return fmt.Sprintf("{table: %d pairs}", len(n.Children))
	default:
		return n.Value
	}
}

// Validate checks structural invariants of the node tree: key uniqueness,
// table/array-of-table conflicts, inline table immutability. Returns nil
// if the tree is valid.
func (n *Node) Validate() error {
	var err error
	Walk(n, func(node *Node) bool {
		if err != nil {
			return false
		}
		if node.Kind == TableNode || node.Kind == InlineTableNode {
			seen := make(map[string]bool)
			for _, child := range node.Children {
				if child.Kind == KeyValueNode && len(child.Key) > 0 {
					k := child.Key[0]
					if seen[k] {
						err = &DuplicateKeyError{Key: k, Pos: child.Pos}
						return false
					}
					seen[k] = true
				}
			}
		}
		return true
	})
	return err
}

// File is the result of parsing a TOML byte stream.
type File struct {
	Root     *Node
	Warnings []string
}

// Parse tokenizes and parses data into an AST.
func Parse(data []byte) (*File, error) {
	s, err := newScanner(data)
	if err != nil {
		return nil, err
	}
	tokens, err := s.scan()
	if err != nil {
		return nil, err
	}

	p := newParser(tokens)
	root, err := p.parse()
	if err != nil {
		return nil, err
	}

	file := &File{
		Root:     exportNode(root),
		Warnings: p.warnings,
	}
	return file, nil
}

// WalkFunc is the callback for [Walk]. Return true to recurse into the
// node's children, or false to skip the subtree.
type WalkFunc func(n *Node) bool

// Walk traverses the AST rooted at n in depth-first pre-order, calling fn
// for each node. If fn returns false, the node's children are not visited.
func Walk(n *Node, fn WalkFunc) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for _, child := range n.Children {
		Walk(child, fn)
	}
}

// Filter walks the AST rooted at n and returns all nodes for which fn
// returns true.
func Filter(n *Node, fn func(*Node) bool) []*Node {
	var result []*Node
	Walk(n, func(node *Node) bool {
		if fn(node) {
			result = append(result, node)
		}
		return true
	})
	return result
}

// NodeToBytes serializes a [Node] tree back into TOML bytes using default
// encoding options.
func NodeToBytes(n *Node) ([]byte, error) {
	internal := importNode(n)
	enc := newEncoder(defaultEncodeOptions())
	return enc.encodeNode(internal)
}

// NodeToBytesWithOptions serializes a [Node] tree back into TOML bytes using
// the provided encoding options.
func NodeToBytesWithOptions(n *Node, opts ...EncodeOption) ([]byte, error) {
	o := defaultEncodeOptions()
	for _, opt := range opts {
		opt(o)
	}
	internal := importNode(n)
	enc := newEncoder(o)
	return enc.encodeNode(internal)
}

func exportNode(n *node) *Node {
	if n == nil {
		return nil
	}
	pub := &Node{
		Kind:        exportKind(n.kind),
		Key:         n.key,
		Value:       n.value,
		Style:       exportStyle(n.style),
		Inline:      n.kind == nodeInlineTable,
		Pos:         n.pos,
		Comment:     n.comment,
		HeadComment: n.headComment,
		FootComment: n.footComment,
	}
	for _, child := range n.children {
		pub.Children = append(pub.Children, exportNode(child))
	}
	return pub
}

func exportKind(k nodeKind) NodeKind {
	switch k {
	case nodeTable:
		return TableNode
	case nodeArrayTable:
		return ArrayTableNode
	case nodeKeyValue:
		return KeyValueNode
	case nodeArray:
		return ArrayNode
	case nodeInlineTable:
		return InlineTableNode
	case nodeString:
		return StringNode
	case nodeInteger:
		return IntegerNode
	case nodeFloat:
		return FloatNode
	case nodeBoolean:
		return BooleanNode
	case nodeDateTime:
		return DateTimeNode
	case nodeLocalDateTime:
		return LocalDateTimeNode
	case nodeLocalDate:
		return LocalDateNode
	case nodeLocalTime:
		return LocalTimeNode
	case nodeComment:
		return CommentNode
	default:
		return StringNode
	}
}

func exportStyle(s stringStyle) StringStyle {
	switch s {
	case styleBasic:
		return BasicStyle
	case styleMultiLineBasic:
		return MultiLineBasicStyle
	case styleLiteral:
		return LiteralStyle
	case styleMultiLineLiteral:
		return MultiLineLiteralStyle
	default:
		return BasicStyle
	}
}

func importNode(n *Node) *node {
	if n == nil {
		return nil
	}
	internal := &node{
		kind:        importKind(n.Kind),
		key:         n.Key,
		value:       n.Value,
		style:       importStyle(n.Style),
		pos:         n.Pos,
		comment:     n.Comment,
		headComment: n.HeadComment,
		footComment: n.FootComment,
	}
	for _, child := range n.Children {
		internal.children = append(internal.children, importNode(child))
	}
	return internal
}

func importKind(k NodeKind) nodeKind {
	switch k {
	case TableNode:
		return nodeTable
	case ArrayTableNode:
		return nodeArrayTable
	case KeyValueNode:
		return nodeKeyValue
	case ArrayNode:
		return nodeArray
	case InlineTableNode:
		return nodeInlineTable
	case StringNode:
		return nodeString
	case IntegerNode:
		return nodeInteger
	case FloatNode:
		return nodeFloat
	case BooleanNode:
		return nodeBoolean
	case DateTimeNode:
		return nodeDateTime
	case LocalDateTimeNode:
		return nodeLocalDateTime
	case LocalDateNode:
		return nodeLocalDate
	case LocalTimeNode:
		return nodeLocalTime
	case CommentNode:
		return nodeComment
	default:
		return nodeString
	}
}

func importStyle(s StringStyle) stringStyle {
	switch s {
	case BasicStyle:
		return styleBasic
	case MultiLineBasicStyle:
		return styleMultiLineBasic
	case LiteralStyle:
		return styleLiteral
	case MultiLineLiteralStyle:
		return styleMultiLineLiteral
	default:
		return styleBasic
	}
}
