package toml

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// PathString() compilation -- valid paths
// ---------------------------------------------------------------------------

func TestPathStringValidPaths(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"root only", "$"},
		{"single key", "$.key"},
		{"nested keys", "$.parent.child"},
		{"deeply nested", "$.a.b.c.d"},
		{"index zero", "$.arr[0]"},
		{"positive index", "$.arr[5]"},
		{"negative index", "$.arr[-1]"},
		{"wildcard", "$.table.*"},
		{"recursive descent", "$..key"},
		{"key then index", "$.arr[0]"},
		{"index then key", "$.arr[0].name"},
		{"multiple indexes", "$.arr[0][1]"},
		{"wildcard then key", "$.*..name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := PathString(tt.expr)
			if err != nil {
				t.Fatalf("PathString(%q): %v", tt.expr, err)
			}
			if p.String() != tt.expr {
				t.Errorf("String() = %q, want %q", p.String(), tt.expr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PathString() compilation -- error cases
// ---------------------------------------------------------------------------

func TestPathStringErrors(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"empty string", ""},
		{"no dollar prefix", "key.path"},
		{"dot prefix", ".key"},
		{"trailing dot", "$.key."},
		{"double dot empty key", "$..."},
		{"empty key after dot", "$."},
		{"unclosed bracket", "$.arr[0"},
		{"invalid index", "$.arr[abc]"},
		{"unexpected char", "$@key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PathString(tt.expr)
			if err == nil {
				t.Fatalf("PathString(%q): expected error, got nil", tt.expr)
			}
			if !errors.Is(err, ErrPathSyntax) {
				t.Errorf("error = %v, want ErrPathSyntax", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Key segments: $.key, $.parent.child
// ---------------------------------------------------------------------------

func TestPathKeySegment(t *testing.T) {
	root := mustParse(t, `
name = "Alice"
age = 30
`)
	p, err := PathString("$.name")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("Read returned %d nodes, want 1", len(nodes))
	}
	if nodes[0].Kind != KeyValueNode {
		t.Errorf("Kind = %d, want KeyValueNode", nodes[0].Kind)
	}
}

func TestPathNestedKeySegment(t *testing.T) {
	root := mustParse(t, `
[server]
host = "localhost"
`)
	p, err := PathString("$.server.host")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("Read returned %d nodes, want 1", len(nodes))
	}
}

func TestPathNonExistentKey(t *testing.T) {
	root := mustParse(t, `name = "Alice"`)
	p, _ := PathString("$.missing")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Index segments: $.arr[0], $.arr[-1], $.arr[999]
// ---------------------------------------------------------------------------

func TestPathIndexSegment(t *testing.T) {
	root := mustParse(t, `
arr = ["a", "b", "c"]
`)
	// Access value node's children via path to arr then index.
	p, err := PathString("$.arr")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read $.arr: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("Read $.arr returned %d nodes, want 1", len(nodes))
	}

	// Now test indexing on root children (which are KV nodes).
	pIdx, err := PathString("$[0]")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	idxNodes, err := pIdx.Read(root)
	if err != nil {
		t.Fatalf("Read $[0]: %v", err)
	}
	if len(idxNodes) != 1 {
		t.Fatalf("Read $[0] returned %d nodes, want 1", len(idxNodes))
	}
}

func TestPathNegativeIndex(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
c = 3
`)
	p, err := PathString("$[-1]")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read $[-1]: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	// Last child should be "c".
	if len(nodes[0].Key) > 0 && nodes[0].Key[0] != "c" {
		t.Errorf("last child key = %v, want [c]", nodes[0].Key)
	}
}

func TestPathIndexOutOfBounds(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, _ := PathString("$[999]")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

func TestPathNegativeIndexOutOfBounds(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, _ := PathString("$[-999]")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Wildcard segments: $.table.*
// ---------------------------------------------------------------------------

func TestPathWildcard(t *testing.T) {
	root := mustParse(t, `
[server]
host = "localhost"
port = 8080
`)
	p, err := PathString("$.server.*")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) < 2 {
		t.Errorf("wildcard returned %d nodes, want >= 2", len(nodes))
	}
}

func TestPathWildcardOnRoot(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
c = 3
`)
	p, _ := PathString("$.*")
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read $.*: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("wildcard on root returned %d nodes, want >= 3", len(nodes))
	}
}

func TestPathWildcardEmptyTable(t *testing.T) {
	root := mustParse(t, `
[empty]
[other]
k = 1
`)
	p, _ := PathString("$.empty.*")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound for empty table wildcard", err)
	}
}

// ---------------------------------------------------------------------------
// Recursive segments: $..key
// ---------------------------------------------------------------------------

func TestPathRecursive(t *testing.T) {
	root := mustParse(t, `
[a]
name = "first"
[a.b]
name = "second"
[c]
name = "third"
`)
	p, err := PathString("$..name")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("recursive descent found %d nodes, want >= 3", len(nodes))
	}
}

func TestPathRecursiveNoMatch(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, _ := PathString("$..missing")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Read() -- single and multiple results
// ---------------------------------------------------------------------------

func TestPathReadSingle(t *testing.T) {
	root := mustParse(t, `name = "test"`)
	p, _ := PathString("$.name")
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("got %d results, want 1", len(nodes))
	}
}

func TestPathReadMultiple(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
c = 3
`)
	p, _ := PathString("$.*")
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("got %d results, want >= 3", len(nodes))
	}
}

func TestPathReadNilNode(t *testing.T) {
	p, _ := PathString("$.key")
	_, err := p.Read(nil)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

func TestPathReadEmptyPath(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$")
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read $: %v", err)
	}
	// With no segments, Read should return the root itself.
	if len(nodes) != 1 || nodes[0] != root {
		t.Errorf("Read $ should return root node")
	}
}

// ---------------------------------------------------------------------------
// ReadString()
// ---------------------------------------------------------------------------

func TestPathReadString(t *testing.T) {
	root := mustParse(t, `name = "Alice"`)
	p, _ := PathString("$.name")
	val, err := p.ReadString(root)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if val != "Alice" {
		t.Errorf("ReadString = %q, want %q", val, "Alice")
	}
}

func TestPathReadStringInteger(t *testing.T) {
	root := mustParse(t, `port = 8080`)
	p, _ := PathString("$.port")
	val, err := p.ReadString(root)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if val != "8080" {
		t.Errorf("ReadString = %q, want %q", val, "8080")
	}
}

func TestPathReadStringNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$.missing")
	_, err := p.ReadString(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// ReadPositions()
// ---------------------------------------------------------------------------

func TestPathReadPositions(t *testing.T) {
	root := mustParse(t, `name = "Alice"
age = 30
`)
	p, _ := PathString("$.name")
	positions, err := p.ReadPositions(root)
	if err != nil {
		t.Fatalf("ReadPositions: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("got %d positions, want 1", len(positions))
	}
	if positions[0].Line <= 0 {
		t.Errorf("Pos.Line = %d, want > 0", positions[0].Line)
	}
}

func TestPathReadPositionsMultiple(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
c = 3
`)
	p, _ := PathString("$.*")
	positions, err := p.ReadPositions(root)
	if err != nil {
		t.Fatalf("ReadPositions: %v", err)
	}
	if len(positions) < 3 {
		t.Errorf("got %d positions, want >= 3", len(positions))
	}
}

func TestPathReadPositionsNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$.missing")
	_, err := p.ReadPositions(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Replace()
// ---------------------------------------------------------------------------

func TestPathReplace(t *testing.T) {
	root := mustParse(t, `
name = "Alice"
age = 30
`)
	p, _ := PathString("$.name")
	replacement := &Node{
		Kind: KeyValueNode,
		Key:  []string{"name"},
		Children: []*Node{
			{Kind: StringNode, Value: "Bob"},
		},
	}
	err := p.Replace(root, replacement)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}

	// Verify replacement.
	nodes, _ := p.Read(root)
	if len(nodes) != 1 {
		t.Fatalf("Read after Replace: got %d nodes", len(nodes))
	}
	if nodes[0].Children[0].Value != "Bob" {
		t.Errorf("Value after Replace = %q, want %q", nodes[0].Children[0].Value, "Bob")
	}
}

func TestPathReplaceNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$.missing")
	err := p.Replace(root, &Node{Kind: KeyValueNode, Key: []string{"missing"}})
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

func TestPathReplaceEmptyPath(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$")
	err := p.Replace(root, &Node{Kind: TableNode})
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("Replace with empty path: error = %v, want ErrPathNotFound", err)
	}
}

func TestPathReplaceNestedKey(t *testing.T) {
	root := mustParse(t, `
[server]
host = "old"
port = 8080
`)
	p, _ := PathString("$.server.host")
	replacement := &Node{
		Kind: KeyValueNode,
		Key:  []string{"host"},
		Children: []*Node{
			{Kind: StringNode, Value: "new"},
		},
	}
	err := p.Replace(root, replacement)
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	nodes, _ := p.Read(root)
	if len(nodes) != 1 || nodes[0].Children[0].Value != "new" {
		t.Errorf("nested replace failed")
	}
}

// ---------------------------------------------------------------------------
// Append()
// ---------------------------------------------------------------------------

func TestPathAppend(t *testing.T) {
	root := mustParse(t, `
[server]
host = "localhost"
`)
	p, _ := PathString("$.server")
	newKV := &Node{
		Kind: KeyValueNode,
		Key:  []string{"port"},
		Children: []*Node{
			{Kind: IntegerNode, Value: "9090"},
		},
	}
	err := p.Append(root, newKV)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Verify appended node.
	serverP, _ := PathString("$.server.port")
	nodes, err := serverP.Read(root)
	if err != nil {
		t.Fatalf("Read after Append: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
}

func TestPathAppendNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$.missing")
	err := p.Append(root, &Node{Kind: IntegerNode, Value: "2"})
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

func TestPathAppendToRoot(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, _ := PathString("$")
	newKV := &Node{
		Kind: KeyValueNode,
		Key:  []string{"b"},
		Children: []*Node{
			{Kind: IntegerNode, Value: "2"},
		},
	}
	err := p.Append(root, newKV)
	if err != nil {
		t.Fatalf("Append to root: %v", err)
	}

	bp, _ := PathString("$.b")
	nodes, err := bp.Read(root)
	if err != nil {
		t.Fatalf("Read $.b: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("got %d nodes for $.b, want 1", len(nodes))
	}
}

// ---------------------------------------------------------------------------
// Delete()
// ---------------------------------------------------------------------------

func TestPathDelete(t *testing.T) {
	root := mustParse(t, `
name = "Alice"
age = 30
`)
	p, _ := PathString("$.name")
	err := p.Delete(root)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify deletion.
	_, err = p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("after Delete: error = %v, want ErrPathNotFound", err)
	}

	// age should still be there.
	ageP, _ := PathString("$.age")
	nodes, err := ageP.Read(root)
	if err != nil {
		t.Fatalf("Read $.age after Delete: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("age missing after deleting name")
	}
}

func TestPathDeleteNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$.missing")
	err := p.Delete(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}

func TestPathDeleteEmptyPath(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathString("$")
	err := p.Delete(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("Delete with empty path: error = %v, want ErrPathNotFound", err)
	}
}

func TestPathDeleteNestedKey(t *testing.T) {
	root := mustParse(t, `
[server]
host = "localhost"
port = 8080
`)
	p, _ := PathString("$.server.host")
	err := p.Delete(root)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("after Delete: error = %v, want ErrPathNotFound", err)
	}
	// port should remain.
	portP, _ := PathString("$.server.port")
	nodes, _ := portP.Read(root)
	if len(nodes) != 1 {
		t.Errorf("port missing after deleting host")
	}
}

func TestPathDeleteReducesChildren(t *testing.T) {
	root := mustParse(t, `
a = 1
b = 2
c = 3
`)
	before := len(root.Children)
	p, _ := PathString("$.b")
	if err := p.Delete(root); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	after := len(root.Children)
	if after != before-1 {
		t.Errorf("Children count: before=%d, after=%d, want %d", before, after, before-1)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestPathOnEmptyTree(t *testing.T) {
	root := mustParse(t, "")
	p, _ := PathString("$.key")
	_, err := p.Read(root)
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound on empty tree", err)
	}
}

func TestPathStringMethod(t *testing.T) {
	p, _ := PathString("$.server.host")
	if p.String() != "$.server.host" {
		t.Errorf("String() = %q, want %q", p.String(), "$.server.host")
	}
}

func TestPathIndexOfAny(t *testing.T) {
	tests := []struct {
		s     string
		chars string
		want  int
	}{
		{"hello.world", ".", 5},
		{"hello[0]", "[", 5},
		{"hello", ".[", -1},
		{"", ".", -1},
		{".start", ".", 0},
	}
	for _, tt := range tests {
		got := indexOfAny(tt.s, tt.chars)
		if got != tt.want {
			t.Errorf("indexOfAny(%q, %q) = %d, want %d", tt.s, tt.chars, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Segment match behavior
// ---------------------------------------------------------------------------

func TestPathKeySegmentMatch(t *testing.T) {
	parent := &Node{
		Kind: TableNode,
		Children: []*Node{
			{Kind: KeyValueNode, Key: []string{"alpha"}},
			{Kind: KeyValueNode, Key: []string{"beta"}},
			{Kind: KeyValueNode, Key: []string{"alpha"}},
		},
	}
	seg := keySegment{key: "alpha"}
	matches := seg.match(parent)
	if len(matches) != 2 {
		t.Errorf("keySegment matched %d, want 2", len(matches))
	}
}

func TestPathKeySegmentNoMatch(t *testing.T) {
	parent := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: []string{"other"}},
	}}
	seg := keySegment{key: "missing"}
	matches := seg.match(parent)
	if len(matches) != 0 {
		t.Errorf("keySegment matched %d, want 0", len(matches))
	}
}

func TestPathKeySegmentEmptyKey(t *testing.T) {
	parent := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: nil},
		{Kind: KeyValueNode, Key: []string{}},
	}}
	seg := keySegment{key: "any"}
	matches := seg.match(parent)
	if len(matches) != 0 {
		t.Errorf("keySegment matched %d for empty-key children, want 0", len(matches))
	}
}

func TestPathIndexSegmentMatch(t *testing.T) {
	parent := &Node{
		Kind: ArrayNode,
		Children: []*Node{
			{Kind: StringNode, Value: "a"},
			{Kind: StringNode, Value: "b"},
			{Kind: StringNode, Value: "c"},
		},
	}
	seg := indexSegment{index: 1}
	matches := seg.match(parent)
	if len(matches) != 1 || matches[0].Value != "b" {
		t.Errorf("indexSegment[1] = %v, want [b]", matches)
	}

	segNeg := indexSegment{index: -1}
	matches = segNeg.match(parent)
	if len(matches) != 1 || matches[0].Value != "c" {
		t.Errorf("indexSegment[-1] = %v, want [c]", matches)
	}
}

func TestPathIndexSegmentOutOfBounds(t *testing.T) {
	parent := &Node{Kind: ArrayNode, Children: []*Node{
		{Kind: StringNode, Value: "a"},
	}}
	seg := indexSegment{index: 5}
	matches := seg.match(parent)
	if len(matches) != 0 {
		t.Errorf("indexSegment[5] matched %d, want 0", len(matches))
	}
}

func TestPathWildcardSegmentMatch(t *testing.T) {
	parent := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: []string{"a"}},
		{Kind: KeyValueNode, Key: []string{"b"}},
	}}
	seg := wildcardSegment{}
	matches := seg.match(parent)
	if len(matches) != 2 {
		t.Errorf("wildcardSegment matched %d, want 2", len(matches))
	}
}

func TestPathWildcardSegmentNoChildren(t *testing.T) {
	parent := &Node{Kind: TableNode}
	seg := wildcardSegment{}
	matches := seg.match(parent)
	if len(matches) != 0 {
		t.Errorf("wildcardSegment matched %d on empty node, want 0", len(matches))
	}
}

func TestPathRecursiveSegmentMatch(t *testing.T) {
	root := &Node{
		Kind: TableNode,
		Children: []*Node{
			{Kind: KeyValueNode, Key: []string{"name"}, Children: []*Node{{Kind: StringNode, Value: "top"}}},
			{
				Kind: TableNode, Key: []string{"sub"},
				Children: []*Node{
					{Kind: KeyValueNode, Key: []string{"name"}, Children: []*Node{{Kind: StringNode, Value: "nested"}}},
				},
			},
		},
	}
	seg := recursiveSegment{key: "name"}
	matches := seg.match(root)
	if len(matches) != 2 {
		t.Errorf("recursiveSegment matched %d, want 2", len(matches))
	}
}

func TestPathRecursiveSegmentNoMatch(t *testing.T) {
	root := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: []string{"other"}},
	}}
	seg := recursiveSegment{key: "missing"}
	matches := seg.match(root)
	if len(matches) != 0 {
		t.Errorf("recursiveSegment matched %d, want 0", len(matches))
	}
}

// ---------------------------------------------------------------------------
// Complex path operations
// ---------------------------------------------------------------------------

func TestPathReplaceAndSerialize(t *testing.T) {
	root := mustParse(t, `
[config]
debug = false
`)
	p, _ := PathString("$.config.debug")
	replacement := &Node{
		Kind: KeyValueNode,
		Key:  []string{"debug"},
		Children: []*Node{
			{Kind: BooleanNode, Value: "true"},
		},
	}
	if err := p.Replace(root, replacement); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	out, err := NodeToBytes(root)
	if err != nil {
		t.Fatalf("NodeToBytes: %v", err)
	}
	if !strings.Contains(string(out), "true") {
		t.Errorf("serialized output missing 'true': %s", string(out))
	}
}

func TestPathDeleteAndVerifyStructure(t *testing.T) {
	root := mustParse(t, `
[server]
host = "localhost"
port = 8080
debug = true
`)
	// Delete port.
	p, _ := PathString("$.server.port")
	if err := p.Delete(root); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The remaining server children should not include port.
	serverP, _ := PathString("$.server")
	serverNodes, err := serverP.Read(root)
	if err != nil {
		t.Fatalf("Read $.server: %v", err)
	}
	for _, child := range serverNodes[0].Children {
		if child.Kind == KeyValueNode && len(child.Key) > 0 && child.Key[0] == "port" {
			t.Error("port still exists after Delete")
		}
	}
}

func TestPathAppendAndRead(t *testing.T) {
	root := mustParse(t, `
[settings]
theme = "dark"
`)
	settingsP, _ := PathString("$.settings")
	newNode := &Node{
		Kind: KeyValueNode,
		Key:  []string{"font_size"},
		Children: []*Node{
			{Kind: IntegerNode, Value: "14"},
		},
	}
	if err := settingsP.Append(root, newNode); err != nil {
		t.Fatalf("Append: %v", err)
	}

	fontP, _ := PathString("$.settings.font_size")
	val, err := fontP.ReadString(root)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if val != "14" {
		t.Errorf("ReadString = %q, want %q", val, "14")
	}
}

// ---------------------------------------------------------------------------
// ReadString on scalar node directly
// ---------------------------------------------------------------------------

func TestPathReadStringScalarDirect(t *testing.T) {
	// ReadString on a node that is itself a scalar (not wrapped in KV).
	n := &Node{Kind: StringNode, Value: "direct"}
	p, _ := PathString("$")
	val, err := p.ReadString(n)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if val != "direct" {
		t.Errorf("ReadString = %q, want %q", val, "direct")
	}
}

// ---------------------------------------------------------------------------
// Replace with nested path that has intermediate missing segments
// ---------------------------------------------------------------------------

func TestPathReplaceIntermediateMissing(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, _ := PathString("$.missing.child")
	err := p.Replace(root, &Node{Kind: IntegerNode, Value: "2"})
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("Replace with missing intermediate: error = %v, want ErrPathNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// PathPointer (RFC 6901)
// ---------------------------------------------------------------------------

func TestPathPointerKeyAndIndex(t *testing.T) {
	root := mustParse(t, `
[server]
ports = [80, 443, 8080]
`)
	p, err := PathPointer("/server/ports/1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadString(root)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if got != "443" {
		t.Errorf("got %q, want 443", got)
	}
}

func TestPathPointerArrayOfInlineTables(t *testing.T) {
	root := mustParse(t, `flags = [{name = "x"}, {name = "y"}]`)
	p, _ := PathPointer("/flags/1/name")
	got, err := p.ReadString(root)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if got != "y" {
		t.Errorf("got %q, want y", got)
	}
}

func TestPathPointerNestedTablePositions(t *testing.T) {
	root := mustParse(t, `
[server]
host = "h"
`)
	p, _ := PathPointer("/server/host")
	positions, err := p.ReadPositions(root)
	if err != nil {
		t.Fatalf("ReadPositions: %v", err)
	}
	if len(positions) != 1 || positions[0].Line <= 0 {
		t.Errorf("got positions %+v", positions)
	}
}

func TestPathPointerEmptySelectsRoot(t *testing.T) {
	root := mustParse(t, `a = 1`)
	p, err := PathPointer("")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := p.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Kind != TableNode {
		t.Errorf("empty pointer should select the root table, got %+v", nodes)
	}
}

func TestPathPointerEscapes(t *testing.T) {
	// Keys containing "/" and "~" are encoded as ~1 and ~0 in the pointer.
	root := mustParse(t, `"a/b" = 1
"c~d" = 2`)
	p1, _ := PathPointer("/a~1b")
	if got, err := p1.ReadString(root); err != nil || got != "1" {
		t.Errorf("/a~1b = %q (err %v); want 1", got, err)
	}
	p2, _ := PathPointer("/c~0d")
	if got, err := p2.ReadString(root); err != nil || got != "2" {
		t.Errorf("/c~0d = %q (err %v); want 2", got, err)
	}
}

func TestPathPointerInvalid(t *testing.T) {
	if _, err := PathPointer("foo"); !errors.Is(err, ErrPointerSyntax) {
		t.Errorf("expected ErrPointerSyntax, got %v", err)
	}
}

func TestPathPointerNotFound(t *testing.T) {
	root := mustParse(t, `k = 1`)
	p, _ := PathPointer("/missing")
	if _, err := p.ReadPositions(root); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("error = %v, want ErrPathNotFound", err)
	}
}
