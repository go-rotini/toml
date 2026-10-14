package toml

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Parse() with various TOML inputs
// ---------------------------------------------------------------------------

func TestNodeParseBasicDocument(t *testing.T) {
	input := `
title = "My App"
version = 1

[server]
host = "0.0.0.0"
port = 443
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if file == nil {
		t.Fatal("Parse returned nil File")
	}
	if file.Root == nil {
		t.Fatal("File.Root is nil")
	}
	if file.Root.Kind != TableNode {
		t.Errorf("Root.Kind = %d, want TableNode", file.Root.Kind)
	}
}

func TestNodeParseReturnsError(t *testing.T) {
	_, err := Parse([]byte(`key = `))
	if err == nil {
		t.Fatal("expected error for incomplete key-value")
	}
}

func TestNodeParseNilInput(t *testing.T) {
	file, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if file == nil || file.Root == nil {
		t.Fatal("expected non-nil File and Root for nil input")
	}
}

func TestNodeParseEmptyInput(t *testing.T) {
	file, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("Parse empty: %v", err)
	}
	if file.Root.Kind != TableNode {
		t.Errorf("Root.Kind = %d, want TableNode", file.Root.Kind)
	}
}

// ---------------------------------------------------------------------------
// File struct
// ---------------------------------------------------------------------------

func TestNodeFileFields(t *testing.T) {
	f, err := Parse([]byte(`k = "v"`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Root == nil {
		t.Error("File.Root is nil")
	}
	// Warnings should be a valid (possibly nil) slice.
	if f.Warnings == nil {
		// nil is acceptable, just ensure we don't panic.
		_ = len(f.Warnings)
	}
}

// ---------------------------------------------------------------------------
// Walk() traversal
// ---------------------------------------------------------------------------

func TestNodeWalkVisitsAllNodes(t *testing.T) {
	root := mustParse(t, `
a = 1
b = "hello"
[tbl]
c = true
`)
	var visited []NodeKind
	Walk(root, func(n *Node) bool {
		visited = append(visited, n.Kind)
		return true
	})
	// We expect at least: root(Table), KV(a), Int(1), KV(b), Str("hello"),
	// Table(tbl), KV(c), Bool(true) = 8 nodes minimum.
	if len(visited) < 8 {
		t.Errorf("Walk visited %d nodes, expected at least 8", len(visited))
	}
}

func TestNodeWalkStopsOnFalse(t *testing.T) {
	root := mustParse(t, `
a = 1
[tbl]
b = 2
c = 3
`)
	count := 0
	Walk(root, func(n *Node) bool {
		count++
		// Stop recursion into the table.
		if n.Kind == TableNode && len(n.Key) > 0 {
			return false
		}
		return true
	})
	// Should visit root + root-level children (a KV, a Int, tbl Table)
	// but not tbl's children because we returned false for tbl.
	// Without descending into tbl, we should not see b or c.
	if count > 6 {
		t.Errorf("Walk visited %d nodes, expected fewer due to stopping", count)
	}
}

func TestNodeWalkNilRoot(t *testing.T) {
	// Walk on nil should not panic.
	Walk(nil, func(n *Node) bool {
		t.Fatal("should not be called")
		return true
	})
}

func TestNodeWalkDepthFirstOrder(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
`)
	var keys []string
	Walk(root, func(n *Node) bool {
		if n.Kind == KeyValueNode && len(n.Key) > 0 {
			keys = append(keys, n.Key[0])
		}
		return true
	})
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("Walk order = %v, want [a b]", keys)
	}
}

// ---------------------------------------------------------------------------
// Filter()
// ---------------------------------------------------------------------------

func TestNodeFilterByKind(t *testing.T) {
	root := mustParse(t, `
a = 1
b = "hello"
c = true
d = 3.14
`)
	ints := Filter(root, func(n *Node) bool {
		return n.Kind == IntegerNode
	})
	if len(ints) != 1 {
		t.Errorf("Filter IntegerNode: got %d, want 1", len(ints))
	}

	strings := Filter(root, func(n *Node) bool {
		return n.Kind == StringNode
	})
	if len(strings) != 1 {
		t.Errorf("Filter StringNode: got %d, want 1", len(strings))
	}

	bools := Filter(root, func(n *Node) bool {
		return n.Kind == BooleanNode
	})
	if len(bools) != 1 {
		t.Errorf("Filter BooleanNode: got %d, want 1", len(bools))
	}

	floats := Filter(root, func(n *Node) bool {
		return n.Kind == FloatNode
	})
	if len(floats) != 1 {
		t.Errorf("Filter FloatNode: got %d, want 1", len(floats))
	}
}

func TestNodeFilterNoMatch(t *testing.T) {
	root := mustParse(t, `a = 1`)
	matches := Filter(root, func(n *Node) bool {
		return n.Kind == DateTimeNode
	})
	if len(matches) != 0 {
		t.Errorf("Filter expected 0 matches, got %d", len(matches))
	}
}

func TestNodeFilterKeyValueNodes(t *testing.T) {
	root := mustParse(t, `
x = 1
y = 2
[t]
z = 3
`)
	kvs := Filter(root, func(n *Node) bool {
		return n.Kind == KeyValueNode
	})
	if len(kvs) != 3 {
		t.Errorf("Filter KeyValueNode: got %d, want 3", len(kvs))
	}
}

func TestNodeFilterTableNodes(t *testing.T) {
	root := mustParse(t, `
[a]
k = 1
[b]
k = 2
`)
	tables := Filter(root, func(n *Node) bool {
		return n.Kind == TableNode && len(n.Key) > 0
	})
	if len(tables) != 2 {
		t.Errorf("Filter TableNode: got %d, want 2", len(tables))
	}
}

// ---------------------------------------------------------------------------
// NodeToBytes()
// ---------------------------------------------------------------------------

func TestNodeToBytes(t *testing.T) {
	input := `title = "hello"
count = 42
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytes(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "title") {
		t.Errorf("output missing 'title': %s", s)
	}
	if !strings.Contains(s, "hello") {
		t.Errorf("output missing 'hello': %s", s)
	}
	if !strings.Contains(s, "42") {
		t.Errorf("output missing '42': %s", s)
	}
}

func TestNodeToBytesNil(t *testing.T) {
	out, err := NodeToBytes(nil)
	if err != nil {
		t.Fatalf("NodeToBytes(nil): %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output for nil node, got %q", string(out))
	}
}

func TestNodeToBytesTable(t *testing.T) {
	input := `[server]
host = "localhost"
port = 8080
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytes(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "[server]") {
		t.Errorf("output missing [server]: %s", s)
	}
	if !strings.Contains(s, "host") {
		t.Errorf("output missing host: %s", s)
	}
}

func TestNodeToBytesArray(t *testing.T) {
	input := `arr = [1, 2, 3]
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytes(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "arr") {
		t.Errorf("output missing 'arr': %s", s)
	}
}

func TestNodeToBytesInlineTable(t *testing.T) {
	input := `point = {x = 1, y = 2}
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytes(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "point") {
		t.Errorf("output missing 'point': %s", s)
	}
}

// ---------------------------------------------------------------------------
// NodeToBytesWithOptions()
// ---------------------------------------------------------------------------

func TestNodeToBytesWithOptions(t *testing.T) {
	input := `title = "test"
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// With default options (no options passed).
	out, err := NodeToBytesWithOptions(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytesWithOptions: %v", err)
	}
	if !strings.Contains(string(out), "title") {
		t.Errorf("output missing 'title': %s", string(out))
	}
}

func TestNodeToBytesWithOptionsIndent(t *testing.T) {
	input := `[server]
host = "localhost"
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytesWithOptions(file.Root, WithIndent(2))
	if err != nil {
		t.Fatalf("NodeToBytesWithOptions: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "host") {
		t.Errorf("output missing 'host': %s", s)
	}
}

// ---------------------------------------------------------------------------
// Node.Validate()
// ---------------------------------------------------------------------------

func TestNodeValidateValid(t *testing.T) {
	root := &Node{
		Kind: TableNode,
		Children: []*Node{
			{Kind: KeyValueNode, Key: []string{"a"}, Children: []*Node{{Kind: IntegerNode, Value: "1"}}},
			{Kind: KeyValueNode, Key: []string{"b"}, Children: []*Node{{Kind: IntegerNode, Value: "2"}}},
		},
	}
	if err := root.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestNodeValidateDuplicateKey(t *testing.T) {
	root := &Node{
		Kind: TableNode,
		Children: []*Node{
			{Kind: KeyValueNode, Key: []string{"a"}, Children: []*Node{{Kind: IntegerNode, Value: "1"}}},
			{Kind: KeyValueNode, Key: []string{"a"}, Children: []*Node{{Kind: IntegerNode, Value: "2"}}},
		},
	}
	err := root.Validate()
	if err == nil {
		t.Fatal("expected DuplicateKeyError")
	}
	if _, ok := err.(*DuplicateKeyError); !ok {
		t.Errorf("error type = %T, want *DuplicateKeyError", err)
	}
}

func TestNodeValidateNestedDuplicate(t *testing.T) {
	root := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: TableNode,
				Key:  []string{"tbl"},
				Children: []*Node{
					{Kind: KeyValueNode, Key: []string{"k"}, Children: []*Node{{Kind: IntegerNode, Value: "1"}}},
					{Kind: KeyValueNode, Key: []string{"k"}, Children: []*Node{{Kind: IntegerNode, Value: "2"}}},
				},
			},
		},
	}
	err := root.Validate()
	if err == nil {
		t.Fatal("expected DuplicateKeyError in nested table")
	}
}

func TestNodeValidateInlineTable(t *testing.T) {
	root := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"t"},
				Children: []*Node{
					{
						Kind: InlineTableNode,
						Children: []*Node{
							{Kind: KeyValueNode, Key: []string{"x"}, Children: []*Node{{Kind: IntegerNode, Value: "1"}}},
							{Kind: KeyValueNode, Key: []string{"x"}, Children: []*Node{{Kind: IntegerNode, Value: "2"}}},
						},
					},
				},
			},
		},
	}
	err := root.Validate()
	if err == nil {
		t.Fatal("expected DuplicateKeyError in inline table")
	}
}

func TestNodeValidateEmptyTree(t *testing.T) {
	root := &Node{Kind: TableNode}
	if err := root.Validate(); err != nil {
		t.Errorf("Validate empty tree: %v", err)
	}
}

// ---------------------------------------------------------------------------
// exportNode/importNode round-trip
// ---------------------------------------------------------------------------

func TestNodeExportImportRoundTrip(t *testing.T) {
	internal := &node{
		kind:  nodeTable,
		key:   nil,
		value: "",
		children: []*node{
			{
				kind:        nodeKeyValue,
				key:         []string{"name"},
				headComment: "a comment",
				comment:     "inline",
				children: []*node{
					{kind: nodeString, value: "Alice", style: styleBasic},
				},
				pos: Position{Line: 2, Column: 1},
			},
			{
				kind: nodeKeyValue,
				key:  []string{"count"},
				children: []*node{
					{kind: nodeInteger, value: "42"},
				},
			},
		},
	}

	// Export to public Node.
	pub := exportNode(internal, (&scanner{}).spanOf)
	if pub.Kind != TableNode {
		t.Errorf("exported Kind = %d, want TableNode", pub.Kind)
	}
	if len(pub.Children) != 2 {
		t.Fatalf("exported Children = %d, want 2", len(pub.Children))
	}
	kv := pub.Children[0]
	if kv.Kind != KeyValueNode {
		t.Errorf("child Kind = %d, want KeyValueNode", kv.Kind)
	}
	if kv.HeadComment != "a comment" {
		t.Errorf("HeadComment = %q, want %q", kv.HeadComment, "a comment")
	}
	if kv.Comment != "inline" {
		t.Errorf("Comment = %q, want %q", kv.Comment, "inline")
	}
	if kv.Pos.Line != 2 || kv.Pos.Column != 1 {
		t.Errorf("Pos = %v, want {2, 1, 0}", kv.Pos)
	}
	val := kv.Children[0]
	if val.Kind != StringNode || val.Value != "Alice" {
		t.Errorf("value = {%d, %q}, want {StringNode, Alice}", val.Kind, val.Value)
	}
	if val.Style != BasicStyle {
		t.Errorf("Style = %d, want BasicStyle", val.Style)
	}

	// Import back to internal.
	reimported := importNode(pub)
	if reimported.kind != nodeTable {
		t.Errorf("reimported kind = %d, want nodeTable", reimported.kind)
	}
	if len(reimported.children) != 2 {
		t.Fatalf("reimported children = %d, want 2", len(reimported.children))
	}
	rkv := reimported.children[0]
	if rkv.kind != nodeKeyValue {
		t.Errorf("reimported child kind = %d, want nodeKeyValue", rkv.kind)
	}
	if rkv.headComment != "a comment" {
		t.Errorf("reimported headComment = %q", rkv.headComment)
	}
	if rkv.comment != "inline" {
		t.Errorf("reimported comment = %q", rkv.comment)
	}
	rval := rkv.children[0]
	if rval.kind != nodeString || rval.value != "Alice" || rval.style != styleBasic {
		t.Errorf("reimported value = {%d, %q, %d}", rval.kind, rval.value, rval.style)
	}
}

func TestNodeExportImportNil(t *testing.T) {
	if exportNode(nil, (&scanner{}).spanOf) != nil {
		t.Error("exportNode(nil, (&scanner{}).spanOf) should return nil")
	}
	if importNode(nil) != nil {
		t.Error("importNode(nil) should return nil")
	}
}

// ---------------------------------------------------------------------------
// exportKind/importKind for all node kinds
// ---------------------------------------------------------------------------

func TestNodeExportKindAllKinds(t *testing.T) {
	tests := []struct {
		internal nodeKind
		public   NodeKind
	}{
		{nodeTable, TableNode},
		{nodeArrayTable, ArrayTableNode},
		{nodeKeyValue, KeyValueNode},
		{nodeArray, ArrayNode},
		{nodeInlineTable, InlineTableNode},
		{nodeString, StringNode},
		{nodeInteger, IntegerNode},
		{nodeFloat, FloatNode},
		{nodeBoolean, BooleanNode},
		{nodeDateTime, DateTimeNode},
		{nodeLocalDateTime, LocalDateTimeNode},
		{nodeLocalDate, LocalDateNode},
		{nodeLocalTime, LocalTimeNode},
		{nodeComment, CommentNode},
	}
	for _, tt := range tests {
		got := exportKind(tt.internal)
		if got != tt.public {
			t.Errorf("exportKind(%d) = %d, want %d", tt.internal, got, tt.public)
		}
	}
}

func TestNodeImportKindAllKinds(t *testing.T) {
	tests := []struct {
		public   NodeKind
		internal nodeKind
	}{
		{TableNode, nodeTable},
		{ArrayTableNode, nodeArrayTable},
		{KeyValueNode, nodeKeyValue},
		{ArrayNode, nodeArray},
		{InlineTableNode, nodeInlineTable},
		{StringNode, nodeString},
		{IntegerNode, nodeInteger},
		{FloatNode, nodeFloat},
		{BooleanNode, nodeBoolean},
		{DateTimeNode, nodeDateTime},
		{LocalDateTimeNode, nodeLocalDateTime},
		{LocalDateNode, nodeLocalDate},
		{LocalTimeNode, nodeLocalTime},
		{CommentNode, nodeComment},
	}
	for _, tt := range tests {
		got := importKind(tt.public)
		if got != tt.internal {
			t.Errorf("importKind(%d) = %d, want %d", tt.public, got, tt.internal)
		}
	}
}

func TestNodeExportKindDefault(t *testing.T) {
	// Unknown kind should default to StringNode.
	got := exportKind(nodeKind(999))
	if got != StringNode {
		t.Errorf("exportKind(999) = %d, want StringNode", got)
	}
}

func TestNodeImportKindDefault(t *testing.T) {
	// Unknown kind should default to nodeString.
	got := importKind(NodeKind(999))
	if got != nodeString {
		t.Errorf("importKind(999) = %d, want nodeString", got)
	}
}

// ---------------------------------------------------------------------------
// exportStyle/importStyle for all string styles
// ---------------------------------------------------------------------------

func TestNodeExportStyleAllStyles(t *testing.T) {
	tests := []struct {
		internal stringStyle
		public   StringStyle
	}{
		{styleBasic, BasicStyle},
		{styleMultiLineBasic, MultiLineBasicStyle},
		{styleLiteral, LiteralStyle},
		{styleMultiLineLiteral, MultiLineLiteralStyle},
	}
	for _, tt := range tests {
		got := exportStyle(tt.internal)
		if got != tt.public {
			t.Errorf("exportStyle(%d) = %d, want %d", tt.internal, got, tt.public)
		}
	}
}

func TestNodeImportStyleAllStyles(t *testing.T) {
	tests := []struct {
		public   StringStyle
		internal stringStyle
	}{
		{BasicStyle, styleBasic},
		{MultiLineBasicStyle, styleMultiLineBasic},
		{LiteralStyle, styleLiteral},
		{MultiLineLiteralStyle, styleMultiLineLiteral},
	}
	for _, tt := range tests {
		got := importStyle(tt.public)
		if got != tt.internal {
			t.Errorf("importStyle(%d) = %d, want %d", tt.public, got, tt.internal)
		}
	}
}

func TestNodeExportStyleDefault(t *testing.T) {
	got := exportStyle(stringStyle(999))
	if got != BasicStyle {
		t.Errorf("exportStyle(999) = %d, want BasicStyle", got)
	}
}

func TestNodeImportStyleDefault(t *testing.T) {
	got := importStyle(StringStyle(999))
	if got != styleBasic {
		t.Errorf("importStyle(999) = %d, want styleBasic", got)
	}
}

// ---------------------------------------------------------------------------
// Node fields
// ---------------------------------------------------------------------------

func TestNodeFields(t *testing.T) {
	root := mustParse(t, `
# header comment
title = "My App" # inline comment

[server]
host = "localhost"
`)
	// Check root Kind.
	if root.Kind != TableNode {
		t.Errorf("Root.Kind = %d, want TableNode", root.Kind)
	}

	// Find title kv.
	titleKV := findKV(root, "title")
	if titleKV == nil {
		t.Fatal("title kv not found")
	}
	if len(titleKV.Key) != 1 || titleKV.Key[0] != "title" {
		t.Errorf("Key = %v, want [title]", titleKV.Key)
	}

	titleVal := kvValue(titleKV)
	if titleVal.Value != "My App" {
		t.Errorf("Value = %q, want %q", titleVal.Value, "My App")
	}
	if titleVal.Style != BasicStyle {
		t.Errorf("Style = %d, want BasicStyle", titleVal.Style)
	}

	// Check that Pos has been set (line > 0).
	if titleKV.Pos.Line <= 0 {
		t.Errorf("Pos.Line = %d, want > 0", titleKV.Pos.Line)
	}

	// Check HeadComment on title.
	if !strings.Contains(titleKV.HeadComment, "header comment") {
		t.Errorf("HeadComment = %q, want to contain 'header comment'", titleKV.HeadComment)
	}

	// Check Children of root.
	if len(root.Children) < 2 {
		t.Errorf("root.Children = %d, want >= 2", len(root.Children))
	}
}

// ---------------------------------------------------------------------------
// Node.String()
// ---------------------------------------------------------------------------

func TestNodeStringTable(t *testing.T) {
	n := &Node{Kind: TableNode, Key: []string{"a", "b"}}
	if n.String() != "a.b" {
		t.Errorf("String() = %q, want %q", n.String(), "a.b")
	}
}

func TestNodeStringArrayTable(t *testing.T) {
	n := &Node{Kind: ArrayTableNode, Key: []string{"items"}}
	if n.String() != "items" {
		t.Errorf("String() = %q, want %q", n.String(), "items")
	}
}

func TestNodeStringKeyValue(t *testing.T) {
	n := &Node{Kind: KeyValueNode, Key: []string{"key"}}
	if n.String() != "key" {
		t.Errorf("String() = %q, want %q", n.String(), "key")
	}
}

func TestNodeStringScalar(t *testing.T) {
	n := &Node{Kind: StringNode, Value: "hello"}
	if n.String() != "hello" {
		t.Errorf("String() = %q, want %q", n.String(), "hello")
	}
}

func TestNodeStringArray(t *testing.T) {
	n := &Node{Kind: ArrayNode, Children: []*Node{{}, {}, {}}}
	s := n.String()
	if !strings.Contains(s, "3 items") {
		t.Errorf("String() = %q, want to contain '3 items'", s)
	}
}

func TestNodeStringInlineTable(t *testing.T) {
	n := &Node{Kind: InlineTableNode, Children: []*Node{{}, {}}}
	s := n.String()
	if !strings.Contains(s, "2 pairs") {
		t.Errorf("String() = %q, want to contain '2 pairs'", s)
	}
}

func TestNodeStringInteger(t *testing.T) {
	n := &Node{Kind: IntegerNode, Value: "42"}
	if n.String() != "42" {
		t.Errorf("String() = %q, want %q", n.String(), "42")
	}
}

func TestNodeStringBoolean(t *testing.T) {
	n := &Node{Kind: BooleanNode, Value: "true"}
	if n.String() != "true" {
		t.Errorf("String() = %q, want %q", n.String(), "true")
	}
}

// ---------------------------------------------------------------------------
// Inline flag on exported nodes
// ---------------------------------------------------------------------------

func TestNodeInlineFlag(t *testing.T) {
	internal := &node{kind: nodeInlineTable}
	pub := exportNode(internal, (&scanner{}).spanOf)
	if !pub.Inline {
		t.Error("Inline should be true for nodeInlineTable")
	}

	internal2 := &node{kind: nodeTable}
	pub2 := exportNode(internal2, (&scanner{}).spanOf)
	if pub2.Inline {
		t.Error("Inline should be false for nodeTable")
	}
}

// ---------------------------------------------------------------------------
// FootComment round-trip
// ---------------------------------------------------------------------------

func TestNodeFootCommentRoundTrip(t *testing.T) {
	internal := &node{
		kind:        nodeKeyValue,
		key:         []string{"k"},
		footComment: "footer",
		children:    []*node{{kind: nodeString, value: "v"}},
	}
	pub := exportNode(internal, (&scanner{}).spanOf)
	if pub.FootComment != "footer" {
		t.Errorf("FootComment = %q, want %q", pub.FootComment, "footer")
	}
	re := importNode(pub)
	if re.footComment != "footer" {
		t.Errorf("reimported footComment = %q, want %q", re.footComment, "footer")
	}
}

// ---------------------------------------------------------------------------
// NodeToBytes round-trip: parse -> export -> bytes -> re-parse
// ---------------------------------------------------------------------------

func TestNodeToBytesRoundTrip(t *testing.T) {
	input := `[database]
host = "localhost"
port = 5432
enabled = true
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := NodeToBytes(file.Root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}

	// Re-parse the output.
	file2, err := Parse(out)
	if err != nil {
		t.Fatalf("Re-parse: %v", err)
	}
	db := findTable(file2.Root, "database")
	if db == nil {
		t.Fatal("[database] not found after round-trip")
	}
	hostKV := findKV(db, "host")
	if hostKV == nil || kvValue(hostKV).Value != "localhost" {
		t.Errorf("host mismatch after round-trip")
	}
}

// ---------------------------------------------------------------------------
// exportNode children ordering
// ---------------------------------------------------------------------------

func TestNodeExportNodePreservesChildOrder(t *testing.T) {
	internal := &node{
		kind: nodeTable,
		children: []*node{
			{kind: nodeKeyValue, key: []string{"first"}, children: []*node{{kind: nodeInteger, value: "1"}}},
			{kind: nodeKeyValue, key: []string{"second"}, children: []*node{{kind: nodeInteger, value: "2"}}},
			{kind: nodeKeyValue, key: []string{"third"}, children: []*node{{kind: nodeInteger, value: "3"}}},
		},
	}
	pub := exportNode(internal, (&scanner{}).spanOf)
	if len(pub.Children) != 3 {
		t.Fatalf("Children = %d, want 3", len(pub.Children))
	}
	keys := []string{pub.Children[0].Key[0], pub.Children[1].Key[0], pub.Children[2].Key[0]}
	want := []string{"first", "second", "third"}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("Children[%d].Key = %q, want %q", i, keys[i], want[i])
		}
	}
}

func TestParseFootComments(t *testing.T) {
	f, err := Parse([]byte("# file\n\na = 1\n# root foot\n\n# t head\n[t]\nx = 1\n# t foot\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := f.Root
	if root.FootComment != " root foot" {
		t.Errorf("root foot = %q", root.FootComment)
	}
	if root.Children[0].HeadComment != " file" {
		t.Errorf("a head = %q", root.Children[0].HeadComment)
	}
	tbl := root.Children[1]
	if tbl.HeadComment != " t head" || tbl.FootComment != " t foot" {
		t.Errorf("t head = %q foot = %q", tbl.HeadComment, tbl.FootComment)
	}
	if f.Warnings != nil {
		t.Errorf("Warnings = %v", f.Warnings)
	}
}

func TestNodeToBytesComments(t *testing.T) {
	src := "# a head\na = 1 # a line\n# root foot\n\n# t head\n[t] # t line\nx = 1\n# t foot\n"
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	want := "# a head\na = 1 # a line\n# root foot\n\n# t head\n[t] # t line\nx = 1\n# t foot\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestNodeToBytesDottedKeyAndOrder(t *testing.T) {
	kv := &Node{Kind: KeyValueNode, Key: []string{"a", "b c"}, Children: []*Node{{Kind: IntegerNode, Value: "1"}}}
	out, err := NodeToBytes(&Node{Kind: TableNode, Children: []*Node{kv}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "a.\"b c\" = 1\n" {
		t.Errorf("got %q", out)
	}

	f, err := Parse([]byte("[t]\nx.y = 1\nz = 2\n\n[[p]]\nn = 1\n[p.sub]\nq = 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, err = NodeToBytes(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := Unmarshal(out, &got); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if err := Unmarshal([]byte("[t]\nx.y = 1\nz = 2\n\n[[p]]\nn = 1\n[p.sub]\nq = 3\n"), &want); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("round trip = %v, want %v\n%s", got, want, out)
	}
}
