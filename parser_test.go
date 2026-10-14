package toml

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// helper: parse TOML source through the public Parse API and return the root Node.
func mustParse(t *testing.T, src string) *Node {
	t.Helper()
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return f.Root
}

// helper: find a KeyValueNode child by key name (non-recursive, one level).
func findKV(n *Node, key string) *Node {
	for _, c := range n.Children {
		if c.Kind == KeyValueNode && len(c.Key) > 0 && c.Key[0] == key {
			return c
		}
	}
	return nil
}

// helper: find a table child by key name (non-recursive, one level).
func findTable(n *Node, key string) *Node {
	for _, c := range n.Children {
		if (c.Kind == TableNode || c.Kind == InlineTableNode) && len(c.Key) > 0 && c.Key[0] == key {
			return c
		}
	}
	return nil
}

// helper: find all ArrayTableNode children with a given key.
func findArrayTables(n *Node, key string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Kind == ArrayTableNode && len(c.Key) > 0 && c.Key[0] == key {
			out = append(out, c)
		}
	}
	return out
}

// helper: return the value node of a KeyValueNode (first child).
func kvValue(kv *Node) *Node {
	if kv == nil || len(kv.Children) == 0 {
		return nil
	}
	return kv.Children[0]
}

// ---------------------------------------------------------------------------
// 1. Simple key-value pairs
// ---------------------------------------------------------------------------

func TestParseSimpleKeyValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		key      string
		wantKind NodeKind
		wantVal  string
	}{
		{
			name:     "basic string",
			input:    `title = "hello"`,
			key:      "title",
			wantKind: StringNode,
			wantVal:  "hello",
		},
		{
			name:     "literal string",
			input:    `path = 'C:\Users\foo'`,
			key:      "path",
			wantKind: StringNode,
			wantVal:  `C:\Users\foo`,
		},
		{
			name:     "integer",
			input:    `count = 42`,
			key:      "count",
			wantKind: IntegerNode,
			wantVal:  "42",
		},
		{
			name:     "negative integer",
			input:    `offset = -17`,
			key:      "offset",
			wantKind: IntegerNode,
			wantVal:  "-17",
		},
		{
			name:     "hex integer",
			input:    `color = 0xDEADBEEF`,
			key:      "color",
			wantKind: IntegerNode,
			wantVal:  "0xDEADBEEF",
		},
		{
			name:     "octal integer",
			input:    `perm = 0o755`,
			key:      "perm",
			wantKind: IntegerNode,
			wantVal:  "0o755",
		},
		{
			name:     "binary integer",
			input:    `bits = 0b11010110`,
			key:      "bits",
			wantKind: IntegerNode,
			wantVal:  "0b11010110",
		},
		{
			name:     "float",
			input:    `pi = 3.14`,
			key:      "pi",
			wantKind: FloatNode,
			wantVal:  "3.14",
		},
		{
			name:     "float with exponent",
			input:    `avogadro = 6.02e23`,
			key:      "avogadro",
			wantKind: FloatNode,
			wantVal:  "6.02e23",
		},
		{
			name:     "inf",
			input:    `val = inf`,
			key:      "val",
			wantKind: FloatNode,
			wantVal:  "inf",
		},
		{
			name:     "negative inf",
			input:    `val = -inf`,
			key:      "val",
			wantKind: FloatNode,
			wantVal:  "-inf",
		},
		{
			name:     "nan",
			input:    `val = nan`,
			key:      "val",
			wantKind: FloatNode,
			wantVal:  "nan",
		},
		{
			name:     "boolean true",
			input:    `enabled = true`,
			key:      "enabled",
			wantKind: BooleanNode,
			wantVal:  "true",
		},
		{
			name:     "boolean false",
			input:    `enabled = false`,
			key:      "enabled",
			wantKind: BooleanNode,
			wantVal:  "false",
		},
		{
			name:     "offset datetime",
			input:    `ts = 1979-05-27T07:32:00Z`,
			key:      "ts",
			wantKind: DateTimeNode,
			wantVal:  "1979-05-27T07:32:00Z",
		},
		{
			name:     "offset datetime with offset",
			input:    `ts = 1979-05-27T07:32:00+05:30`,
			key:      "ts",
			wantKind: DateTimeNode,
			wantVal:  "1979-05-27T07:32:00+05:30",
		},
		{
			name:     "local datetime",
			input:    `ts = 1979-05-27T07:32:00`,
			key:      "ts",
			wantKind: LocalDateTimeNode,
			wantVal:  "1979-05-27T07:32:00",
		},
		{
			name:     "local date",
			input:    `d = 1979-05-27`,
			key:      "d",
			wantKind: LocalDateNode,
			wantVal:  "1979-05-27",
		},
		{
			name:     "local time",
			input:    `t = 07:32:00`,
			key:      "t",
			wantKind: LocalTimeNode,
			wantVal:  "07:32:00",
		},
		{
			name:     "multiline basic string",
			input:    "desc = \"\"\"\nhello\nworld\"\"\"",
			key:      "desc",
			wantKind: StringNode,
			wantVal:  "hello\nworld",
		},
		{
			name:     "multiline literal string",
			input:    "desc = '''\nhello\nworld'''",
			key:      "desc",
			wantKind: StringNode,
			wantVal:  "hello\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			kv := findKV(root, tt.key)
			if kv == nil {
				t.Fatalf("key %q not found in root", tt.key)
			}
			val := kvValue(kv)
			if val == nil {
				t.Fatalf("key %q has no value node", tt.key)
			}
			if val.Kind != tt.wantKind {
				t.Errorf("kind = %d, want %d", val.Kind, tt.wantKind)
			}
			if val.Value != tt.wantVal {
				t.Errorf("value = %q, want %q", val.Value, tt.wantVal)
			}
		})
	}
}

func TestParseStringStyles(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantStyle StringStyle
	}{
		{"basic", `k = "v"`, BasicStyle},
		{"literal", `k = 'v'`, LiteralStyle},
		{"multiline basic", "k = \"\"\"\nv\"\"\"", MultiLineBasicStyle},
		{"multiline literal", "k = '''\nv'''", MultiLineLiteralStyle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			val := kvValue(findKV(root, "k"))
			if val.Style != tt.wantStyle {
				t.Errorf("style = %d, want %d", val.Style, tt.wantStyle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Tables
// ---------------------------------------------------------------------------

func TestParseTables(t *testing.T) {
	t.Run("simple table", func(t *testing.T) {
		root := mustParse(t, `
[server]
host = "localhost"
port = 8080
`)
		tbl := findTable(root, "server")
		if tbl == nil {
			t.Fatal("table [server] not found")
		}
		host := findKV(tbl, "host")
		if host == nil || kvValue(host).Value != "localhost" {
			t.Errorf("server.host mismatch")
		}
		port := findKV(tbl, "port")
		if port == nil || kvValue(port).Value != "8080" {
			t.Errorf("server.port mismatch")
		}
	})

	t.Run("nested table with dotted header", func(t *testing.T) {
		root := mustParse(t, `
[a.b.c]
key = "deep"
`)
		a := findTable(root, "a")
		if a == nil {
			t.Fatal("table [a] not found")
		}
		b := findTable(a, "b")
		if b == nil {
			t.Fatal("table [a.b] not found")
		}
		c := findTable(b, "c")
		if c == nil {
			t.Fatal("table [a.b.c] not found")
		}
		kv := findKV(c, "key")
		if kv == nil || kvValue(kv).Value != "deep" {
			t.Error("a.b.c.key mismatch")
		}
	})

	t.Run("empty table", func(t *testing.T) {
		root := mustParse(t, `
[empty]
[other]
k = 1
`)
		empty := findTable(root, "empty")
		if empty == nil {
			t.Fatal("table [empty] not found")
		}
		// The empty table should have no key-value children.
		kvCount := 0
		for _, c := range empty.Children {
			if c.Kind == KeyValueNode {
				kvCount++
			}
		}
		if kvCount != 0 {
			t.Errorf("[empty] has %d key-value children, want 0", kvCount)
		}
	})

	t.Run("multiple tables", func(t *testing.T) {
		root := mustParse(t, `
[a]
x = 1
[b]
y = 2
`)
		if findTable(root, "a") == nil {
			t.Error("[a] not found")
		}
		if findTable(root, "b") == nil {
			t.Error("[b] not found")
		}
	})
}

// ---------------------------------------------------------------------------
// 3. Array of tables
// ---------------------------------------------------------------------------

func TestParseArrayOfTables(t *testing.T) {
	t.Run("multiple entries", func(t *testing.T) {
		root := mustParse(t, `
[[products]]
name = "Hammer"
sku = 738594937

[[products]]
name = "Nail"
sku = 284758393
`)
		entries := findArrayTables(root, "products")
		if len(entries) != 2 {
			t.Fatalf("got %d products, want 2", len(entries))
		}
		name0 := findKV(entries[0], "name")
		if name0 == nil || kvValue(name0).Value != "Hammer" {
			t.Error("first product name mismatch")
		}
		name1 := findKV(entries[1], "name")
		if name1 == nil || kvValue(name1).Value != "Nail" {
			t.Error("second product name mismatch")
		}
	})

	t.Run("array of tables with nested keys", func(t *testing.T) {
		root := mustParse(t, `
[[fruit]]
name = "apple"
color = "red"

[[fruit]]
name = "banana"
color = "yellow"
`)
		entries := findArrayTables(root, "fruit")
		if len(entries) != 2 {
			t.Fatalf("got %d fruit entries, want 2", len(entries))
		}

		name0 := findKV(entries[0], "name")
		if name0 == nil || kvValue(name0).Value != "apple" {
			t.Error("fruit[0].name mismatch")
		}
		color0 := findKV(entries[0], "color")
		if color0 == nil || kvValue(color0).Value != "red" {
			t.Error("fruit[0].color mismatch")
		}

		name1 := findKV(entries[1], "name")
		if name1 == nil || kvValue(name1).Value != "banana" {
			t.Error("fruit[1].name mismatch")
		}
		color1 := findKV(entries[1], "color")
		if color1 == nil || kvValue(color1).Value != "yellow" {
			t.Error("fruit[1].color mismatch")
		}
	})
}

// ---------------------------------------------------------------------------
// 4. Dotted keys
// ---------------------------------------------------------------------------

func TestParseDottedKeys(t *testing.T) {
	t.Run("creates implicit tables", func(t *testing.T) {
		root := mustParse(t, `a.b.c = 1`)
		a := findTable(root, "a")
		if a == nil {
			t.Fatal("implicit table a not found")
		}
		b := findTable(a, "b")
		if b == nil {
			t.Fatal("implicit table a.b not found")
		}
		c := findKV(b, "c")
		if c == nil {
			t.Fatal("key c not found under a.b")
		}
		if kvValue(c).Value != "1" {
			t.Errorf("a.b.c = %q, want %q", kvValue(c).Value, "1")
		}
	})

	t.Run("dotted key alongside normal key", func(t *testing.T) {
		root := mustParse(t, `
name = "Orange"
physical.color = "orange"
physical.shape = "round"
`)
		name := findKV(root, "name")
		if name == nil || kvValue(name).Value != "Orange" {
			t.Error("name mismatch")
		}
		phys := findTable(root, "physical")
		if phys == nil {
			t.Fatal("implicit table physical not found")
		}
		color := findKV(phys, "color")
		if color == nil || kvValue(color).Value != "orange" {
			t.Error("physical.color mismatch")
		}
		shape := findKV(phys, "shape")
		if shape == nil || kvValue(shape).Value != "round" {
			t.Error("physical.shape mismatch")
		}
	})

	t.Run("quoted dotted key", func(t *testing.T) {
		root := mustParse(t, `"a"."b" = 42`)
		a := findTable(root, "a")
		if a == nil {
			t.Fatal("implicit table a not found")
		}
		b := findKV(a, "b")
		if b == nil || kvValue(b).Value != "42" {
			t.Error("a.b mismatch")
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Inline tables
// ---------------------------------------------------------------------------

func TestParseInlineTables(t *testing.T) {
	t.Run("simple inline table", func(t *testing.T) {
		root := mustParse(t, `point = { x = 1, y = 2 }`)
		kv := findKV(root, "point")
		if kv == nil {
			t.Fatal("key point not found")
		}
		val := kvValue(kv)
		if val.Kind != InlineTableNode {
			t.Fatalf("kind = %d, want InlineTableNode", val.Kind)
		}
		if !val.Inline {
			t.Error("Inline flag not set")
		}
		x := findKV(val, "x")
		y := findKV(val, "y")
		if x == nil || kvValue(x).Value != "1" {
			t.Error("x mismatch")
		}
		if y == nil || kvValue(y).Value != "2" {
			t.Error("y mismatch")
		}
	})

	t.Run("nested inline tables", func(t *testing.T) {
		root := mustParse(t, `point = { position = { x = 1, y = 2 }, color = "red" }`)
		kv := findKV(root, "point")
		val := kvValue(kv)
		if val.Kind != InlineTableNode {
			t.Fatal("expected InlineTableNode")
		}
		pos := findKV(val, "position")
		if pos == nil {
			t.Fatal("position not found")
		}
		posVal := kvValue(pos)
		if posVal.Kind != InlineTableNode {
			t.Fatal("expected nested InlineTableNode")
		}
		x := findKV(posVal, "x")
		if x == nil || kvValue(x).Value != "1" {
			t.Error("position.x mismatch")
		}
	})

	t.Run("inline table with trailing comma (TOML 1.1.0)", func(t *testing.T) {
		root := mustParse(t, `point = { x = 1, y = 2, }`)
		val := kvValue(findKV(root, "point"))
		if val.Kind != InlineTableNode {
			t.Fatal("expected InlineTableNode")
		}
		if len(val.Children) != 2 {
			t.Errorf("got %d children, want 2", len(val.Children))
		}
	})

	t.Run("inline table with newlines (TOML 1.1.0)", func(t *testing.T) {
		input := "point = {\n  x = 1,\n  y = 2\n}"
		root := mustParse(t, input)
		val := kvValue(findKV(root, "point"))
		if val.Kind != InlineTableNode {
			t.Fatal("expected InlineTableNode")
		}
		if len(val.Children) != 2 {
			t.Errorf("got %d children, want 2", len(val.Children))
		}
	})

	t.Run("empty inline table", func(t *testing.T) {
		root := mustParse(t, `empty = {}`)
		val := kvValue(findKV(root, "empty"))
		if val.Kind != InlineTableNode {
			t.Fatal("expected InlineTableNode")
		}
		if len(val.Children) != 0 {
			t.Errorf("got %d children, want 0", len(val.Children))
		}
	})
}

// ---------------------------------------------------------------------------
// 6. Arrays
// ---------------------------------------------------------------------------

func TestParseArrays(t *testing.T) {
	t.Run("simple array", func(t *testing.T) {
		root := mustParse(t, `arr = [1, 2, 3]`)
		val := kvValue(findKV(root, "arr"))
		if val.Kind != ArrayNode {
			t.Fatalf("kind = %d, want ArrayNode", val.Kind)
		}
		if len(val.Children) != 3 {
			t.Fatalf("got %d items, want 3", len(val.Children))
		}
		for i, want := range []string{"1", "2", "3"} {
			if val.Children[i].Value != want {
				t.Errorf("arr[%d] = %q, want %q", i, val.Children[i].Value, want)
			}
		}
	})

	t.Run("array with many elements", func(t *testing.T) {
		root := mustParse(t, `arr = [1, 2, 3, 4, 5]`)
		val := kvValue(findKV(root, "arr"))
		if val.Kind != ArrayNode {
			t.Fatalf("kind = %d, want ArrayNode", val.Kind)
		}
		if len(val.Children) != 5 {
			t.Fatalf("got %d items, want 5", len(val.Children))
		}
	})

	t.Run("trailing comma", func(t *testing.T) {
		root := mustParse(t, `arr = [1, 2, 3,]`)
		val := kvValue(findKV(root, "arr"))
		if len(val.Children) != 3 {
			t.Errorf("got %d items, want 3", len(val.Children))
		}
	})

	t.Run("nested arrays", func(t *testing.T) {
		root := mustParse(t, `arr = [[1, 2], [3, 4]]`)
		val := kvValue(findKV(root, "arr"))
		if val.Kind != ArrayNode {
			t.Fatal("expected ArrayNode")
		}
		if len(val.Children) != 2 {
			t.Fatalf("outer has %d children, want 2", len(val.Children))
		}
		inner0 := val.Children[0]
		if inner0.Kind != ArrayNode {
			t.Fatal("inner[0] expected ArrayNode")
		}
		if len(inner0.Children) != 2 {
			t.Errorf("inner[0] has %d children, want 2", len(inner0.Children))
		}
	})

	t.Run("mixed type array", func(t *testing.T) {
		// TOML 1.1.0 allows mixed type arrays
		root := mustParse(t, `arr = [1, "two", 3.0, true]`)
		val := kvValue(findKV(root, "arr"))
		if len(val.Children) != 4 {
			t.Fatalf("got %d items, want 4", len(val.Children))
		}
		if val.Children[0].Kind != IntegerNode {
			t.Errorf("arr[0] kind = %d, want IntegerNode", val.Children[0].Kind)
		}
		if val.Children[1].Kind != StringNode {
			t.Errorf("arr[1] kind = %d, want StringNode", val.Children[1].Kind)
		}
		if val.Children[2].Kind != FloatNode {
			t.Errorf("arr[2] kind = %d, want FloatNode", val.Children[2].Kind)
		}
		if val.Children[3].Kind != BooleanNode {
			t.Errorf("arr[3] kind = %d, want BooleanNode", val.Children[3].Kind)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		root := mustParse(t, `arr = []`)
		val := kvValue(findKV(root, "arr"))
		if val.Kind != ArrayNode {
			t.Fatal("expected ArrayNode")
		}
		if len(val.Children) != 0 {
			t.Errorf("got %d items, want 0", len(val.Children))
		}
	})

	t.Run("array of strings", func(t *testing.T) {
		root := mustParse(t, `arr = ["a", "b", "c"]`)
		val := kvValue(findKV(root, "arr"))
		if len(val.Children) != 3 {
			t.Fatalf("got %d items, want 3", len(val.Children))
		}
		for _, c := range val.Children {
			if c.Kind != StringNode {
				t.Errorf("expected StringNode, got %d", c.Kind)
			}
		}
	})

	t.Run("array of inline tables", func(t *testing.T) {
		root := mustParse(t, `arr = [{a = 1}, {a = 2}]`)
		val := kvValue(findKV(root, "arr"))
		if len(val.Children) != 2 {
			t.Fatalf("got %d items, want 2", len(val.Children))
		}
		for _, c := range val.Children {
			if c.Kind != InlineTableNode {
				t.Errorf("expected InlineTableNode, got %d", c.Kind)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Duplicate key detection
// ---------------------------------------------------------------------------

func TestParseDuplicateKeys(t *testing.T) {
	t.Run("same key twice", func(t *testing.T) {
		src := "a = 1\na = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for duplicate key")
		}
		var dupErr *DuplicateKeyError
		if !errors.As(err, &dupErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError; err = %v", err, err)
		}
	})

	t.Run("dotted key conflicts with existing key", func(t *testing.T) {
		src := "a.b = 1\na.b = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for duplicate dotted key")
		}
		var dupErr *DuplicateKeyError
		if !errors.As(err, &dupErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError; err = %v", err, err)
		}
	})

	t.Run("key in table duplicates", func(t *testing.T) {
		src := "[t]\nk = 1\nk = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for duplicate key in table")
		}
		var dupErr *DuplicateKeyError
		if !errors.As(err, &dupErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError; err = %v", err, err)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Table redefinition detection
// ---------------------------------------------------------------------------

func TestParseTableRedefinition(t *testing.T) {
	t.Run("same table header twice", func(t *testing.T) {
		src := "[a]\nk = 1\n[a]\nk = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for redefining table [a]")
		}
		var dupErr *DuplicateKeyError
		if !errors.As(err, &dupErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError; err = %v", err, err)
		}
	})

	t.Run("nested table redefinition", func(t *testing.T) {
		src := "[a.b]\nk = 1\n[a.b]\nk = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for redefining table [a.b]")
		}
	})
}

// ---------------------------------------------------------------------------
// 9. Inline table immutability
// ---------------------------------------------------------------------------

func TestParseInlineTableImmutability(t *testing.T) {
	t.Run("cannot add keys to inline table via table header", func(t *testing.T) {
		// The parser may report this as either a DuplicateKeyError (because
		// "a" is already in the defined map) or an InlineTableError. Either
		// way, re-opening an inline table via a table header is rejected.
		src := "a = {b = 1}\n[a]\nc = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for extending inline table via [a]")
		}
		var dupErr *DuplicateKeyError
		var inlErr *InlineTableError
		if !errors.As(err, &dupErr) && !errors.As(err, &inlErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError or *InlineTableError; err = %v", err, err)
		}
	})

	t.Run("cannot add keys to nested inline table", func(t *testing.T) {
		src := "a = {b = {c = 1}}\n[a.b]\nd = 2\n"
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatal("expected error for extending nested inline table")
		}
		var dupErr *DuplicateKeyError
		var inlErr *InlineTableError
		if !errors.As(err, &dupErr) && !errors.As(err, &inlErr) {
			t.Fatalf("error type = %T, want *DuplicateKeyError or *InlineTableError; err = %v", err, err)
		}
	})
}

// ---------------------------------------------------------------------------
// 10. Super-tables after sub-tables
// ---------------------------------------------------------------------------

func TestParseSuperTableAfterSubTable(t *testing.T) {
	t.Run("define sub-table then parent table", func(t *testing.T) {
		src := `
[a.b]
val = 1

[a]
other = 2
`
		root := mustParse(t, src)
		a := findTable(root, "a")
		if a == nil {
			t.Fatal("table [a] not found")
		}
		b := findTable(a, "b")
		if b == nil {
			t.Fatal("table [a.b] not found")
		}
		bVal := findKV(b, "val")
		if bVal == nil || kvValue(bVal).Value != "1" {
			t.Error("a.b.val mismatch")
		}
		aOther := findKV(a, "other")
		if aOther == nil || kvValue(aOther).Value != "2" {
			t.Error("a.other mismatch")
		}
	})

	t.Run("deep sub-table then ancestor", func(t *testing.T) {
		src := `
[x.y.z]
deep = true

[x.y]
mid = true

[x]
top = true
`
		root := mustParse(t, src)
		x := findTable(root, "x")
		if x == nil {
			t.Fatal("[x] not found")
		}
		if findKV(x, "top") == nil {
			t.Error("x.top not found")
		}
		y := findTable(x, "y")
		if y == nil {
			t.Fatal("[x.y] not found")
		}
		if findKV(y, "mid") == nil {
			t.Error("x.y.mid not found")
		}
		z := findTable(y, "z")
		if z == nil {
			t.Fatal("[x.y.z] not found")
		}
		if findKV(z, "deep") == nil {
			t.Error("x.y.z.deep not found")
		}
	})
}

// ---------------------------------------------------------------------------
// 11. Comments preserved in AST
// ---------------------------------------------------------------------------

func TestParseComments(t *testing.T) {
	t.Run("head comment attached to key-value", func(t *testing.T) {
		src := "# file header\nkey = \"value\"\n"
		root := mustParse(t, src)
		kv := findKV(root, "key")
		if kv == nil {
			t.Fatal("key 'key' not found")
		}
		if !strings.Contains(kv.HeadComment, "file header") {
			t.Errorf("HeadComment = %q, want to contain 'file header'", kv.HeadComment)
		}
	})

	t.Run("head comment attached to table header", func(t *testing.T) {
		src := "# section comment\n[section]\nk = 1\n"
		root := mustParse(t, src)
		sect := findTable(root, "section")
		if sect == nil {
			t.Fatal("[section] not found")
		}
		if !strings.Contains(sect.HeadComment, "section comment") {
			t.Errorf("HeadComment = %q, want to contain 'section comment'", sect.HeadComment)
		}
	})

	t.Run("head comment inside table attached to key", func(t *testing.T) {
		src := "[section]\n# key comment\nk = 1\n"
		root := mustParse(t, src)
		sect := findTable(root, "section")
		if sect == nil {
			t.Fatal("[section] not found")
		}
		kv := findKV(sect, "k")
		if kv == nil {
			t.Fatal("key 'k' not found in section")
		}
		if !strings.Contains(kv.HeadComment, "key comment") {
			t.Errorf("HeadComment = %q, want to contain 'key comment'", kv.HeadComment)
		}
	})

	t.Run("multiple head comment lines joined", func(t *testing.T) {
		src := "# line one\n# line two\nk = 1\n"
		root := mustParse(t, src)
		kv := findKV(root, "k")
		if kv == nil {
			t.Fatal("key 'k' not found")
		}
		if !strings.Contains(kv.HeadComment, "line one") || !strings.Contains(kv.HeadComment, "line two") {
			t.Errorf("HeadComment = %q, want both 'line one' and 'line two'", kv.HeadComment)
		}
	})
}

// ---------------------------------------------------------------------------
// 12. Node tree structure verification
// ---------------------------------------------------------------------------

func TestParseNodeTreeStructure(t *testing.T) {
	t.Run("root is TableNode", func(t *testing.T) {
		root := mustParse(t, `k = 1`)
		if root.Kind != TableNode {
			t.Errorf("root kind = %d, want TableNode", root.Kind)
		}
	})

	t.Run("KeyValue has correct key and value child", func(t *testing.T) {
		root := mustParse(t, `name = "Alice"`)
		kv := findKV(root, "name")
		if kv == nil {
			t.Fatal("key 'name' not found")
		}
		if len(kv.Key) != 1 || kv.Key[0] != "name" {
			t.Errorf("kv.Key = %v, want [name]", kv.Key)
		}
		if len(kv.Children) != 1 {
			t.Fatalf("kv has %d children, want 1", len(kv.Children))
		}
		val := kv.Children[0]
		if val.Kind != StringNode {
			t.Errorf("value kind = %d, want StringNode", val.Kind)
		}
		if val.Value != "Alice" {
			t.Errorf("value = %q, want %q", val.Value, "Alice")
		}
	})

	t.Run("table node has key path", func(t *testing.T) {
		root := mustParse(t, `
[server]
host = "localhost"
`)
		tbl := findTable(root, "server")
		if tbl == nil {
			t.Fatal("[server] not found")
		}
		if len(tbl.Key) == 0 || tbl.Key[0] != "server" {
			t.Errorf("table key = %v, want [server]", tbl.Key)
		}
	})

	t.Run("array table node has key path", func(t *testing.T) {
		root := mustParse(t, `
[[items]]
v = 1
`)
		entries := findArrayTables(root, "items")
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if len(entries[0].Key) == 0 || entries[0].Key[0] != "items" {
			t.Errorf("array table key = %v, want [items]", entries[0].Key)
		}
		if entries[0].Kind != ArrayTableNode {
			t.Errorf("kind = %d, want ArrayTableNode", entries[0].Kind)
		}
	})

	t.Run("inline table marked as Inline", func(t *testing.T) {
		root := mustParse(t, `t = {a = 1}`)
		val := kvValue(findKV(root, "t"))
		if !val.Inline {
			t.Error("Inline not set on inline table node")
		}
	})

	t.Run("array children ordered correctly", func(t *testing.T) {
		root := mustParse(t, `arr = ["a", "b", "c"]`)
		val := kvValue(findKV(root, "arr"))
		want := []string{"a", "b", "c"}
		for i, w := range want {
			if val.Children[i].Value != w {
				t.Errorf("arr[%d] = %q, want %q", i, val.Children[i].Value, w)
			}
		}
	})

	t.Run("multiple key-value pairs preserve order", func(t *testing.T) {
		root := mustParse(t, "a = 1\nb = 2\nc = 3\n")
		keys := []string{}
		for _, c := range root.Children {
			if c.Kind == KeyValueNode && len(c.Key) > 0 {
				keys = append(keys, c.Key[0])
			}
		}
		want := []string{"a", "b", "c"}
		if len(keys) != len(want) {
			t.Fatalf("got %d keys, want %d", len(keys), len(want))
		}
		for i := range want {
			if keys[i] != want[i] {
				t.Errorf("key[%d] = %q, want %q", i, keys[i], want[i])
			}
		}
	})

	t.Run("Node.String returns dotted key for tables", func(t *testing.T) {
		root := mustParse(t, `[a]
k = 1`)
		tbl := findTable(root, "a")
		if tbl.String() != "a" {
			t.Errorf("String() = %q, want %q", tbl.String(), "a")
		}
	})

	t.Run("Node.String returns value for scalars", func(t *testing.T) {
		root := mustParse(t, `k = "hello"`)
		val := kvValue(findKV(root, "k"))
		if val.String() != "hello" {
			t.Errorf("String() = %q, want %q", val.String(), "hello")
		}
	})
}

// ---------------------------------------------------------------------------
// 13. maxNodes enforcement
// ---------------------------------------------------------------------------

func TestParseMaxNodes(t *testing.T) {
	t.Run("exceeds maxNodes", func(t *testing.T) {
		// Build a TOML document with many key-value pairs.
		var b strings.Builder
		for i := 0; i < 50; i++ {
			b.WriteString("k")
			b.WriteString(strings.Repeat("x", i))
			b.WriteString(" = ")
			b.WriteString(`"v"`)
			b.WriteByte('\n')
		}
		src := b.String()

		sc, err := newScanner([]byte(src))
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		tokens, err := sc.scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		p := newParser(tokens)
		p.maxNodes = 5 // very low limit
		_, err = p.parse()
		if err == nil {
			t.Fatal("expected error from maxNodes enforcement")
		}
		var synErr *SyntaxError
		if !errors.As(err, &synErr) {
			t.Fatalf("error type = %T, want *SyntaxError", err)
		}
		if !strings.Contains(synErr.Message, "maximum node count") {
			t.Errorf("error message = %q, want to contain 'maximum node count'", synErr.Message)
		}
	})

	t.Run("within maxNodes succeeds", func(t *testing.T) {
		src := "a = 1\nb = 2\n"
		sc, err := newScanner([]byte(src))
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		tokens, err := sc.scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		p := newParser(tokens)
		p.maxNodes = 100
		_, err = p.parse()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("maxNodes zero means no limit", func(t *testing.T) {
		src := "a = 1\nb = 2\nc = 3\n"
		sc, err := newScanner([]byte(src))
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		tokens, err := sc.scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		p := newParser(tokens)
		p.maxNodes = 0
		_, err = p.parse()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 14. Error cases
// ---------------------------------------------------------------------------

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "missing value after equals",
			src:  "k = \n",
		},
		{
			name: "missing equals sign",
			src:  "k 42\n",
		},
		{
			name: "unterminated table header",
			src:  "[table\nk = 1\n",
		},
		{
			name: "unterminated array table header",
			src:  "[[table\nk = 1\n",
		},
		{
			name: "empty key",
			src:  "= 1\n",
		},
		{
			name: "bare key with invalid character",
			src:  "k@ = 1\n",
		},
		{
			name: "unterminated string",
			src:  `k = "hello`,
		},
		{
			name: "unterminated literal string",
			src:  `k = 'hello`,
		},
		{
			name: "unterminated array",
			src:  `k = [1, 2`,
		},
		{
			name: "unterminated inline table",
			src:  `k = {a = 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src))
			if err == nil {
				t.Fatalf("Parse(%q): expected error, got nil", tt.src)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestParseEmptyDocument(t *testing.T) {
	root := mustParse(t, "")
	if root.Kind != TableNode {
		t.Errorf("root kind = %d, want TableNode", root.Kind)
	}
}

func TestParseOnlyComments(t *testing.T) {
	// A document with only comments should parse successfully.
	// Comments are stored as pending and may not produce CommentNode children
	// in the current implementation, but the parse must not error.
	root := mustParse(t, "# just a comment\n# another one\n")
	if root.Kind != TableNode {
		t.Errorf("root kind = %d, want TableNode", root.Kind)
	}
}

func TestParseOnlyWhitespace(t *testing.T) {
	root := mustParse(t, "   \n\n   \n")
	if root.Kind != TableNode {
		t.Errorf("root kind = %d, want TableNode", root.Kind)
	}
}

func TestParseWalkAndFilter(t *testing.T) {
	root := mustParse(t, `
a = 1
b = "hello"
c = true
[sect]
d = 2
`)
	// Walk should visit all nodes
	count := 0
	Walk(root, func(n *Node) bool {
		count++
		return true
	})
	if count < 5 {
		t.Errorf("Walk visited %d nodes, expected at least 5", count)
	}

	// Filter for only IntegerNode values
	ints := Filter(root, func(n *Node) bool {
		return n.Kind == IntegerNode
	})
	if len(ints) != 2 {
		t.Errorf("Filter found %d IntegerNodes, want 2", len(ints))
	}
}

func TestParseValidFunction(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", `k = 1`, true},
		{"valid table", "[t]\nk = 1", true},
		{"invalid duplicate", "k = 1\nk = 2", false},
		{"invalid syntax", "k = ", false},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Valid([]byte(tt.input))
			if got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFileWarnings(t *testing.T) {
	// Parse should return a File with a non-nil Warnings slice (possibly empty).
	f, err := Parse([]byte(`k = 1`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f == nil {
		t.Fatal("Parse returned nil File")
	}
	if f.Root == nil {
		t.Fatal("File.Root is nil")
	}
}

func TestParseComplexDocument(t *testing.T) {
	src := `
# This is a full TOML document.

title = "TOML Example"

[owner]
name = "Tom Preston-Werner"
dob = 1979-05-27T07:32:00-08:00

[database]
enabled = true
ports = [8001, 8001, 8002]
data = [["delta", "phi"], [3.14]]
temp_targets = {cpu = 79.5, case = 72.0}

[servers]

[servers.alpha]
ip = "10.0.0.1"
role = "frontend"

[servers.beta]
ip = "10.0.0.2"
role = "backend"
`
	root := mustParse(t, src)

	// Verify title
	titleKV := findKV(root, "title")
	if titleKV == nil || kvValue(titleKV).Value != "TOML Example" {
		t.Error("title mismatch")
	}

	// Verify owner
	owner := findTable(root, "owner")
	if owner == nil {
		t.Fatal("[owner] not found")
	}
	nameKV := findKV(owner, "name")
	if nameKV == nil || kvValue(nameKV).Value != "Tom Preston-Werner" {
		t.Error("owner.name mismatch")
	}
	dobKV := findKV(owner, "dob")
	if dobKV == nil {
		t.Fatal("owner.dob not found")
	}
	if kvValue(dobKV).Kind != DateTimeNode {
		t.Errorf("dob kind = %d, want DateTimeNode", kvValue(dobKV).Kind)
	}

	// Verify database
	db := findTable(root, "database")
	if db == nil {
		t.Fatal("[database] not found")
	}
	enabled := findKV(db, "enabled")
	if enabled == nil || kvValue(enabled).Value != "true" {
		t.Error("database.enabled mismatch")
	}
	ports := findKV(db, "ports")
	if ports == nil {
		t.Fatal("database.ports not found")
	}
	portsVal := kvValue(ports)
	if portsVal.Kind != ArrayNode || len(portsVal.Children) != 3 {
		t.Errorf("ports: kind=%d, len=%d", portsVal.Kind, len(portsVal.Children))
	}

	// Verify nested data array
	data := findKV(db, "data")
	if data == nil {
		t.Fatal("database.data not found")
	}
	dataVal := kvValue(data)
	if dataVal.Kind != ArrayNode || len(dataVal.Children) != 2 {
		t.Errorf("data: kind=%d, len=%d", dataVal.Kind, len(dataVal.Children))
	}

	// Verify temp_targets inline table
	tt := findKV(db, "temp_targets")
	if tt == nil {
		t.Fatal("database.temp_targets not found")
	}
	ttVal := kvValue(tt)
	if ttVal.Kind != InlineTableNode {
		t.Errorf("temp_targets kind = %d, want InlineTableNode", ttVal.Kind)
	}

	// Verify servers
	servers := findTable(root, "servers")
	if servers == nil {
		t.Fatal("[servers] not found")
	}
	alpha := findTable(servers, "alpha")
	if alpha == nil {
		t.Fatal("[servers.alpha] not found")
	}
	beta := findTable(servers, "beta")
	if beta == nil {
		t.Fatal("[servers.beta] not found")
	}
}

func TestParseArrayTableResetNamespace(t *testing.T) {
	// Each [[array]] entry should have its own key namespace.
	src := `
[[item]]
name = "first"

[[item]]
name = "second"
`
	root := mustParse(t, src)
	entries := findArrayTables(root, "item")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Both entries can have "name" without duplicate error.
	for i, entry := range entries {
		kv := findKV(entry, "name")
		if kv == nil {
			t.Errorf("entry[%d] missing key 'name'", i)
		}
	}
}

func TestParseNodeValidate(t *testing.T) {
	t.Run("valid tree passes", func(t *testing.T) {
		root := mustParse(t, `
a = 1
b = 2
[t]
c = 3
`)
		if err := root.Validate(); err != nil {
			t.Errorf("Validate returned error: %v", err)
		}
	})
}

func TestParseIntegerFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		key   string
		want  string
	}{
		{"decimal", "k = 42", "k", "42"},
		{"positive", "k = +42", "k", "+42"},
		{"negative", "k = -42", "k", "-42"},
		{"hex", "k = 0xDEAD", "k", "0xDEAD"},
		{"oct", "k = 0o777", "k", "0o777"},
		{"bin", "k = 0b101", "k", "0b101"},
		{"underscore", "k = 1_000_000", "k", "1_000_000"},
		{"zero", "k = 0", "k", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			val := kvValue(findKV(root, tt.key))
			if val.Kind != IntegerNode {
				t.Errorf("kind = %d, want IntegerNode", val.Kind)
			}
			if val.Value != tt.want {
				t.Errorf("value = %q, want %q", val.Value, tt.want)
			}
		})
	}
}

func TestParseFloatFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "k = 3.14", "3.14"},
		{"positive exponent", "k = 1e10", "1e10"},
		{"negative exponent", "k = 1e-10", "1e-10"},
		{"full", "k = 6.626e-34", "6.626e-34"},
		{"positive inf", "k = +inf", "+inf"},
		{"negative inf", "k = -inf", "-inf"},
		{"nan", "k = nan", "nan"},
		{"positive nan", "k = +nan", "+nan"},
		{"negative nan", "k = -nan", "-nan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			val := kvValue(findKV(root, "k"))
			if val.Kind != FloatNode {
				t.Errorf("kind = %d, want FloatNode", val.Kind)
			}
			if val.Value != tt.want {
				t.Errorf("value = %q, want %q", val.Value, tt.want)
			}
		})
	}
}

func TestParseDateTimeFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKind NodeKind
		wantVal  string
	}{
		{
			"offset datetime Z",
			"k = 1979-05-27T07:32:00Z",
			DateTimeNode,
			"1979-05-27T07:32:00Z",
		},
		{
			"offset datetime with offset",
			"k = 1979-05-27T07:32:00+05:30",
			DateTimeNode,
			"1979-05-27T07:32:00+05:30",
		},
		{
			"offset datetime with fractional",
			"k = 1979-05-27T07:32:00.999999Z",
			DateTimeNode,
			"1979-05-27T07:32:00.999999Z",
		},
		{
			"local datetime",
			"k = 1979-05-27T07:32:00",
			LocalDateTimeNode,
			"1979-05-27T07:32:00",
		},
		{
			"local date",
			"k = 1979-05-27",
			LocalDateNode,
			"1979-05-27",
		},
		{
			"local time",
			"k = 07:32:00",
			LocalTimeNode,
			"07:32:00",
		},
		{
			"local time with fractional",
			"k = 07:32:00.123",
			LocalTimeNode,
			"07:32:00.123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			val := kvValue(findKV(root, "k"))
			if val.Kind != tt.wantKind {
				t.Errorf("kind = %d, want %d", val.Kind, tt.wantKind)
			}
			if val.Value != tt.wantVal {
				t.Errorf("value = %q, want %q", val.Value, tt.wantVal)
			}
		})
	}
}

func TestParseEscapeSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"newline", `k = "hello\nworld"`, "hello\nworld"},
		{"tab", `k = "hello\tworld"`, "hello\tworld"},
		{"backslash", `k = "c:\\path"`, "c:\\path"},
		{"quote", `k = "say \"hi\""`, `say "hi"`},
		{"unicode 4", `k = "A"`, "A"},
		{"unicode 8", `k = "\U00000041"`, "A"},
		{"carriage return", `k = "hello\rworld"`, "hello\rworld"},
		{"backspace", `k = "hello\bworld"`, "hello\bworld"},
		{"form feed", `k = "hello\fworld"`, "hello\fworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mustParse(t, tt.input)
			val := kvValue(findKV(root, "k"))
			if val.Value != tt.want {
				t.Errorf("value = %q, want %q", val.Value, tt.want)
			}
		})
	}
}

func TestParseBOM(t *testing.T) {
	// UTF-8 BOM followed by valid TOML
	src := "\xEF\xBB\xBFk = 1\n"
	root := mustParse(t, src)
	kv := findKV(root, "k")
	if kv == nil {
		t.Fatal("key 'k' not found after BOM")
	}
}

func TestParseCRLF(t *testing.T) {
	src := "k = 1\r\nj = 2\r\n"
	root := mustParse(t, src)
	if findKV(root, "k") == nil {
		t.Error("key 'k' not found")
	}
	if findKV(root, "j") == nil {
		t.Error("key 'j' not found")
	}
}

func TestParseMultipleKeyValuePairsInRootAndTables(t *testing.T) {
	src := `
a = 1
b = 2

[t1]
c = 3
d = 4

[t2]
e = 5
`
	root := mustParse(t, src)

	// Root keys
	if findKV(root, "a") == nil {
		t.Error("root key 'a' not found")
	}
	if findKV(root, "b") == nil {
		t.Error("root key 'b' not found")
	}

	t1 := findTable(root, "t1")
	if t1 == nil {
		t.Fatal("[t1] not found")
	}
	if findKV(t1, "c") == nil {
		t.Error("t1.c not found")
	}
	if findKV(t1, "d") == nil {
		t.Error("t1.d not found")
	}

	t2 := findTable(root, "t2")
	if t2 == nil {
		t.Fatal("[t2] not found")
	}
	if findKV(t2, "e") == nil {
		t.Error("t2.e not found")
	}
}

func TestParseLeadingZeroError(t *testing.T) {
	_, err := Parse([]byte(`k = 07`))
	if err == nil {
		t.Fatal("expected error for leading zero")
	}
}

func TestParseDottedKeyInTable(t *testing.T) {
	src := `
[fruit]
apple.color = "red"
apple.taste.sweet = true
`
	root := mustParse(t, src)
	fruit := findTable(root, "fruit")
	if fruit == nil {
		t.Fatal("[fruit] not found")
	}
	apple := findTable(fruit, "apple")
	if apple == nil {
		t.Fatal("fruit.apple implicit table not found")
	}
	color := findKV(apple, "color")
	if color == nil || kvValue(color).Value != "red" {
		t.Error("fruit.apple.color mismatch")
	}
	taste := findTable(apple, "taste")
	if taste == nil {
		t.Fatal("fruit.apple.taste implicit table not found")
	}
	sweet := findKV(taste, "sweet")
	if sweet == nil || kvValue(sweet).Value != "true" {
		t.Error("fruit.apple.taste.sweet mismatch")
	}
}

func TestParseInlineTableInArray(t *testing.T) {
	src := `points = [{x = 1, y = 2}, {x = 3, y = 4}]`
	root := mustParse(t, src)
	val := kvValue(findKV(root, "points"))
	if val.Kind != ArrayNode {
		t.Fatal("expected ArrayNode")
	}
	if len(val.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(val.Children))
	}
	for i, c := range val.Children {
		if c.Kind != InlineTableNode {
			t.Errorf("child[%d] kind = %d, want InlineTableNode", i, c.Kind)
		}
	}
}

func TestParseSpaceDelimitedDateTime(t *testing.T) {
	src := `k = 1979-05-27 07:32:00Z`
	root := mustParse(t, src)
	val := kvValue(findKV(root, "k"))
	if val.Kind != DateTimeNode {
		t.Errorf("kind = %d, want DateTimeNode", val.Kind)
	}
}

func TestParseArrayWithTrailingCommaLong(t *testing.T) {
	src := `arr = [1, 2, 3, 4, 5,]`
	root := mustParse(t, src)
	val := kvValue(findKV(root, "arr"))
	if val.Kind != ArrayNode {
		t.Fatal("expected ArrayNode")
	}
	if len(val.Children) != 5 {
		t.Errorf("got %d children, want 5", len(val.Children))
	}
}

func TestParseDeepNestingRejected(t *testing.T) {
	const n = 1_000_000
	docs := map[string]string{
		"arrays": "a = " + strings.Repeat("[", n) + strings.Repeat("]", n) + "\n",
		"inline": "a = " + strings.Repeat("{b=", n) + "1" + strings.Repeat("}", n) + "\n",
		"dotted": strings.Repeat("a.", n) + "a = 1\n",
		"header": "[" + strings.Repeat("a.", n) + "a]\n",
	}
	for name, doc := range docs {
		t.Run(name, func(t *testing.T) {
			data := []byte(doc)
			if _, err := Parse(data); !errors.Is(err, ErrSyntax) {
				t.Errorf("Parse: err = %v, want SyntaxError", err)
			}
			if Valid(data) {
				t.Error("Valid = true, want false")
			}
			var m map[string]any
			if err := UnmarshalWithOptions(data, &m, WithMaxDepth(0)); !errors.Is(err, ErrSyntax) {
				t.Errorf("Unmarshal without a depth limit: err = %v, want SyntaxError", err)
			}
		})
	}
}

func TestParseMaxDepthCountsArraysAndInlineTables(t *testing.T) {
	deepInline := "a = " + strings.Repeat("{b=", 200) + "1" + strings.Repeat("}", 200) + "\n"
	var m map[string]any
	if err := Unmarshal([]byte(deepInline), &m); !errors.Is(err, ErrSyntax) {
		t.Errorf("inline tables 200 deep with the default limit: err = %v, want SyntaxError", err)
	}
	if !Valid([]byte(deepInline)) {
		t.Error("Valid rejected inline tables 200 deep")
	}

	cases := []struct {
		doc   string
		limit int
		ok    bool
	}{
		{"a = 1\n", 1, true},
		{"a = [1]\n", 1, false},
		{"a = [1]\n", 2, true},
		{"a = [[1]]\n", 2, false},
		{"a = {b = 1}\n", 2, true},
		{"a = {b = {c = 1}}\n", 2, false},
		{"a = {b.c = 1}\n", 2, false},
		{"a.b = 1\n", 2, true},
		{"a.b.c = 1\n", 2, false},
		{"[a]\nb = [1]\n", 2, false},
		{"[a]\nb = [1]\n", 3, true},
		{"[[a]]\n", 1, false},
		{"[[a]]\n", 2, true},
	}
	for _, c := range cases {
		var m map[string]any
		err := UnmarshalWithOptions([]byte(c.doc), &m, WithMaxDepth(c.limit))
		if (err == nil) != c.ok {
			t.Errorf("%q with WithMaxDepth(%d): err = %v, want ok=%v", c.doc, c.limit, err, c.ok)
		}
	}
}

// TestParseLinearInKeys checks the property that keeps parsing linear in the
// number of keys: every key lookup is either a scan of at most indexThreshold
// children or a map hit. It counts structure instead of timing, so it holds
// under the race detector and on a loaded machine.
func TestParseLinearInKeys(t *testing.T) {
	const n = 5000
	var flat, inline, tables, dotted strings.Builder
	inline.WriteString("a = {")
	for i := range n {
		fmt.Fprintf(&flat, "k%d = %d\n", i, i)
		if i > 0 {
			inline.WriteString(", ")
		}
		fmt.Fprintf(&inline, "k%d = %d", i, i)
		fmt.Fprintf(&tables, "[t%d]\nx = 1\n", i)
		fmt.Fprintf(&dotted, "a.k%d = %d\n", i, i)
	}
	inline.WriteString("}\n")
	for name, doc := range map[string]string{
		"flat": flat.String(), "inline": inline.String(),
		"tables": tables.String(), "dotted": dotted.String(),
	} {
		_, root, err := parseDocument([]byte(doc), 0, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var keyed int
		var check func(*node)
		check = func(nd *node) {
			k := 0
			for _, c := range nd.children {
				if len(c.key) > 0 {
					k++
					if nd.index != nil && nd.index[c.key[0]] == nil {
						t.Errorf("%s: key %q missing from its parent's index", name, c.key[0])
					}
				}
				check(c)
			}
			keyed += k
			if k > indexThreshold && nd.index == nil {
				t.Errorf("%s: node with %d keyed children has no index", name, k)
			}
		}
		check(root)
		if keyed < n {
			t.Errorf("%s: found %d keyed nodes, want at least %d", name, keyed, n)
		}
	}
}

func TestParseArrayTableEntriesStartClean(t *testing.T) {
	valid := []struct {
		doc  string
		want map[string]any
	}{
		{"[[arr]]\na = [1]\n[[arr]]\n[[arr.a]]\nx = 1\n",
			map[string]any{"arr": []any{
				map[string]any{"a": []any{int64(1)}},
				map[string]any{"a": []any{map[string]any{"x": int64(1)}}},
			}}},
		{"[[arr]]\na = {x = 1}\n[[arr]]\n[arr.a]\nx = 2\n",
			map[string]any{"arr": []any{
				map[string]any{"a": map[string]any{"x": int64(1)}},
				map[string]any{"a": map[string]any{"x": int64(2)}},
			}}},
		{"[[arr]]\n[[arr.sub]]\n[[arr]]\n[arr.sub]\nx = 1\n",
			map[string]any{"arr": []any{
				map[string]any{"sub": []any{map[string]any{}}},
				map[string]any{"sub": map[string]any{"x": int64(1)}},
			}}},
		{"[[arr]]\n[arr.t]\n[[arr]]\n[[arr.t]]\n",
			map[string]any{"arr": []any{
				map[string]any{"t": map[string]any{}},
				map[string]any{"t": []any{map[string]any{}}},
			}}},
		{"[[arr]]\na.b = 1\n[[arr]]\n[[arr.a]]\n",
			map[string]any{"arr": []any{
				map[string]any{"a": map[string]any{"b": int64(1)}},
				map[string]any{"a": []any{map[string]any{}}},
			}}},
	}
	for _, c := range valid {
		var m map[string]any
		if err := Unmarshal([]byte(c.doc), &m); err != nil {
			t.Errorf("%q: %v", c.doc, err)
			continue
		}
		if !reflect.DeepEqual(m, c.want) {
			t.Errorf("%q:\n got %v\nwant %v", c.doc, m, c.want)
		}
	}

	invalid := []string{
		"[[arr]]\na.0 = 0\n[[arr]]\na = 0\na.0 = 0\n",
		"[[arr]]\nx = 1\n[[arr]]\nx = 1\nx = 2\n",
		"[[arr]]\n[arr.t]\n[[arr]]\n[arr.t]\n[arr.t]\n",
	}
	for _, doc := range invalid {
		if Valid([]byte(doc)) {
			t.Errorf("Valid(%q) = true, want false", doc)
		}
	}
}

func TestParseNulKeyIsNotAPathSeparator(t *testing.T) {
	var m map[string]any
	if err := Unmarshal([]byte("\"a\\u0000b\" = 1\na.b = 2\n"), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := map[string]any{"a\x00b": int64(1), "a": map[string]any{"b": int64(2)}}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("got %v, want %v", m, want)
	}

	_, err := Parse([]byte("t = {a.b = 1, a.b = 2}\n"))
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) || dup.Key != "a.b" {
		t.Errorf("err = %v, want duplicate key \"a.b\"", err)
	}
}

func TestParseHeaderAfterDottedKeyOnImpliedTable(t *testing.T) {
	_, err := Parse([]byte("[a.b.c]\n[a]\nb.d = 1\n[a.b]\n"))
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) || dup.Key != "a.b" {
		t.Errorf("header for a table extended by dotted keys: err = %v, want DuplicateKeyError for a.b", err)
	}
	for _, doc := range []string{
		"[a.b.c]\n[a]\nb.d = 1\n[a.b.e]\n",
		"[a.b.c]\n[a]\nb.d = 1\nb.e = 2\n",
		"[a.b.c]\n[a.b]\n[a]\nx = 1\n",
	} {
		if _, err := Parse([]byte(doc)); err != nil {
			t.Errorf("%q: %v", doc, err)
		}
	}
}
