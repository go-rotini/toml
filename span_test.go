package toml

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// spanDoc exercises every node shape a splice editor meets: comments in each
// position, dotted and quoted keys, inline tables, arrays (single- and
// multi-line), arrays of tables and multi-line strings.
const spanDoc = `# head of file

title = "demo" # trailing comment
"quoted key" = 'literal'
a.b.c = 1
site."example.com".port = 8080
dotted . spaced = true

[server] # header comment
host = "0.0.0.0"
ports = [ 8000, 8001 ]
limits = { cpu = 2, mem.max = "1Gi" }
banner = """
Welcome,
  friend"""
path = '''C:\temp'''
when = 1979-05-27T07:32:00Z

# servers below
[[fruit]]
name = "apple"
tags = [
  "red", # a comment inside the array
  "round",
]

[[fruit]]
name = "banana"

[ "a b" . c ]
x = -1.5e3
`

func spanText(data []byte, s Span) string {
	return string(data[s.Start.Offset:s.End.Offset])
}

// lookup descends by key segments, stepping through inline-table values.
func lookup(t *testing.T, root *Node, path ...string) *Node {
	t.Helper()
	cur := root
	for i, seg := range path {
		if cur.Kind == KeyValueNode && len(cur.Children) == 1 && cur.Children[0].Kind == InlineTableNode {
			cur = cur.Children[0]
		}
		var next *Node
		for _, c := range cur.Children {
			if len(c.Key) > 0 && c.Key[0] == seg {
				next = c
			}
		}
		if next == nil {
			t.Fatalf("no node at %q", strings.Join(path[:i+1], "."))
		}
		cur = next
	}
	return cur
}

// checkLineCol confirms each span position's Line and Column agree with its
// Offset in data.
func checkLineCol(t *testing.T, data []byte, root *Node) {
	t.Helper()
	bom := 0
	if strings.HasPrefix(string(data), "\uFEFF") {
		bom = 3
	}
	want := func(off int) (int, int) {
		line, col := 1, 1
		for i := bom; i < off; {
			r, size := utf8.DecodeRune(data[i:])
			switch r {
			case '\n':
				line++
				col = 1
			case '\r':
			default:
				col++
			}
			i += size
		}
		return line, col
	}
	Walk(root, func(n *Node) bool {
		for _, sp := range []Span{n.Span, n.KeySpan} {
			if sp.IsZero() {
				continue
			}
			for _, p := range []Position{sp.Start, sp.End} {
				line, col := want(p.Offset)
				if p.Line != line || p.Column != col {
					t.Errorf("%s: position %+v, want line %d column %d", n, p, line, col)
				}
			}
		}
		return true
	})
}

func TestSpanText(t *testing.T) {
	data := []byte(spanDoc)
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	root := f.Root

	tests := []struct {
		path     []string
		pair     string // the KeyValueNode's Span, or the table header
		key      string // KeySpan
		valueTxt string // the value child's Span; empty for tables
	}{
		{[]string{"title"}, `title = "demo"`, `title`, `"demo"`},
		{[]string{"quoted key"}, `"quoted key" = 'literal'`, `"quoted key"`, `'literal'`},
		{[]string{"a", "b", "c"}, `a.b.c = 1`, `a.b.c`, `1`},
		{[]string{"site", "example.com", "port"}, `site."example.com".port = 8080`, `site."example.com".port`, `8080`},
		{[]string{"dotted", "spaced"}, `dotted . spaced = true`, `dotted . spaced`, `true`},
		{[]string{"server"}, `[server]`, `server`, ``},
		{[]string{"server", "host"}, `host = "0.0.0.0"`, `host`, `"0.0.0.0"`},
		{[]string{"server", "ports"}, `ports = [ 8000, 8001 ]`, `ports`, `[ 8000, 8001 ]`},
		{[]string{"server", "limits"}, `limits = { cpu = 2, mem.max = "1Gi" }`, `limits`, `{ cpu = 2, mem.max = "1Gi" }`},
		{[]string{"server", "limits", "cpu"}, `cpu = 2`, `cpu`, `2`},
		{[]string{"server", "limits", "mem", "max"}, `mem.max = "1Gi"`, `mem.max`, `"1Gi"`},
		{[]string{"server", "banner"}, "banner = \"\"\"\nWelcome,\n  friend\"\"\"", `banner`, "\"\"\"\nWelcome,\n  friend\"\"\""},
		{[]string{"server", "path"}, `path = '''C:\temp'''`, `path`, `'''C:\temp'''`},
		{[]string{"server", "when"}, `when = 1979-05-27T07:32:00Z`, `when`, `1979-05-27T07:32:00Z`},
		{[]string{"a b", "c"}, `[ "a b" . c ]`, `"a b" . c`, ``},
		{[]string{"a b", "c", "x"}, `x = -1.5e3`, `x`, `-1.5e3`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.path, "."), func(t *testing.T) {
			n := lookup(t, root, tt.path...)
			if got := spanText(data, n.Span); got != tt.pair {
				t.Errorf("Span = %q, want %q", got, tt.pair)
			}
			if got := spanText(data, n.KeySpan); got != tt.key {
				t.Errorf("KeySpan = %q, want %q", got, tt.key)
			}
			if tt.valueTxt == "" {
				return
			}
			if got := spanText(data, n.Children[0].Span); got != tt.valueTxt {
				t.Errorf("value Span = %q, want %q", got, tt.valueTxt)
			}
		})
	}

	ports := lookup(t, root, "server", "ports").Children[0]
	if got := spanText(data, ports.Children[1].Span); got != "8001" {
		t.Errorf("array item Span = %q, want %q", got, "8001")
	}
	checkLineCol(t, data, root)
}

func TestSpanArrayOfTables(t *testing.T) {
	data := []byte(spanDoc)
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entries := Filter(f.Root, func(n *Node) bool { return n.Kind == ArrayTableNode })
	if len(entries) != 2 {
		t.Fatalf("got %d [[fruit]] entries, want 2", len(entries))
	}
	for i, want := range []string{`"apple"`, `"banana"`} {
		e := entries[i]
		if got := spanText(data, e.Span); got != "[[fruit]]" {
			t.Errorf("entry %d Span = %q", i, got)
		}
		if got := spanText(data, e.KeySpan); got != "fruit" {
			t.Errorf("entry %d KeySpan = %q", i, got)
		}
		if got := spanText(data, e.Children[0].Children[0].Span); got != want {
			t.Errorf("entry %d name = %q, want %q", i, got, want)
		}
	}
	tags := entries[0].Children[1]
	wantTags := "[\n  \"red\", # a comment inside the array\n  \"round\",\n]"
	if got := spanText(data, tags.Children[0].Span); got != wantTags {
		t.Errorf("multi-line array Span = %q, want %q", got, wantTags)
	}
}

func TestSpanZeroForImpliedTables(t *testing.T) {
	f, err := Parse([]byte("a.b = 1\n[x.y]\nz = 2\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, path := range [][]string{{"a"}, {"x"}} {
		n := lookup(t, f.Root, path...)
		if !n.Span.IsZero() || !n.KeySpan.IsZero() {
			t.Errorf("%v: implied table has spans %+v %+v", path, n.Span, n.KeySpan)
		}
	}
	if !f.Root.Span.IsZero() {
		t.Errorf("root Span = %+v, want zero", f.Root.Span)
	}
}

func TestSpanExplicitAfterImplied(t *testing.T) {
	data := []byte("[x.y]\nz = 2\n[x]\nw = 1\n")
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := spanText(data, lookup(t, f.Root, "x").Span); got != "[x]" {
		t.Errorf("Span = %q, want [x]", got)
	}
}

// TestSpanOffsetsIndexInput checks that offsets index the caller's bytes even
// when the scanner drops a byte-order mark and the \r of each \r\n.
func TestSpanOffsetsIndexInput(t *testing.T) {
	lf := spanDoc
	inputs := map[string]string{
		"crlf":     strings.ReplaceAll(lf, "\n", "\r\n"),
		"bom":      "\uFEFF" + lf,
		"bom+crlf": "\uFEFF" + strings.ReplaceAll(lf, "\n", "\r\n"),
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			data := []byte(in)
			f, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			kv := lookup(t, f.Root, "title")
			if got := spanText(data, kv.Span); got != `title = "demo"` {
				t.Errorf("Span = %q", got)
			}
			x := lookup(t, f.Root, "a b", "c", "x")
			if got := spanText(data, x.Children[0].Span); got != "-1.5e3" {
				t.Errorf("last value Span = %q", got)
			}
			banner := lookup(t, f.Root, "server", "banner").Children[0]
			want := "\"\"\"\nWelcome,\n  friend\"\"\""
			if strings.Contains(name, "crlf") {
				want = strings.ReplaceAll(want, "\n", "\r\n")
			}
			if got := spanText(data, banner.Span); got != want {
				t.Errorf("multi-line string Span = %q, want %q", got, want)
			}
			checkLineCol(t, data, f.Root)
		})
	}
}

// TestSpanSplice replaces values through their spans, the way a config
// editor does, and checks that every other byte survives.
func TestSpanSplice(t *testing.T) {
	data := []byte(spanDoc)
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edits := []struct {
		path []string
		with string
	}{
		{[]string{"title"}, `"renamed"`},
		{[]string{"server", "limits", "cpu"}, `4`},
		{[]string{"server", "banner"}, `"one line"`},
		{[]string{"site", "example.com", "port"}, `9090`},
	}
	type cut struct {
		span Span
		with string
	}
	var cuts []cut
	for _, e := range edits {
		cuts = append(cuts, cut{lookup(t, f.Root, e.path...).Children[0].Span, e.with})
	}
	// Apply from the end so earlier offsets stay valid.
	slices.SortFunc(cuts, func(a, b cut) int { return b.span.Start.Offset - a.span.Start.Offset })
	out := string(data)
	for _, c := range cuts {
		out = out[:c.span.Start.Offset] + c.with + out[c.span.End.Offset:]
	}

	want := strings.NewReplacer(
		`title = "demo"`, `title = "renamed"`,
		`cpu = 2`, `cpu = 4`,
		"banner = \"\"\"\nWelcome,\n  friend\"\"\"", `banner = "one line"`,
		`port = 8080`, `port = 9090`,
	).Replace(spanDoc)
	if out != want {
		t.Fatalf("spliced document:\n%s\nwant:\n%s", out, want)
	}

	var got struct {
		Title  string `toml:"title"`
		Server struct {
			Banner string         `toml:"banner"`
			Limits map[string]any `toml:"limits"`
		} `toml:"server"`
	}
	if err := Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("spliced document does not parse: %v", err)
	}
	if got.Title != "renamed" || got.Server.Banner != "one line" || got.Server.Limits["cpu"] != int64(4) {
		t.Errorf("decoded %+v", got)
	}
}

func TestSpanKeepsErrorPositions(t *testing.T) {
	_, err := Parse([]byte("a = 1\na = 2\n"))
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) {
		t.Fatalf("err = %v, want DuplicateKeyError", err)
	}
	if dup.Pos.Line != 2 || dup.Pos.Column != 1 || dup.Pos.Offset != 6 {
		t.Errorf("Pos = %+v, want 2:1 at offset 6, where the key begins", dup.Pos)
	}
}

func isScalar(k NodeKind) bool {
	switch k {
	case StringNode, IntegerNode, FloatNode, BooleanNode, DateTimeNode, LocalDateTimeNode, LocalDateNode, LocalTimeNode:
		return true
	default:
		return false
	}
}

// checkSpans asserts the invariants a splice editor relies on: every span is
// inside data, a pair's span runs from its key to the end of its value, and a
// scalar's or key's text parses back to the same scalar or key.
func checkSpans(t *testing.T, data []byte, root *Node) {
	t.Helper()
	inBounds := func(s Span) bool {
		return s.Start.Offset >= 0 && s.Start.Offset <= s.End.Offset && s.End.Offset <= len(data)
	}
	Walk(root, func(n *Node) bool {
		if !inBounds(n.Span) || !inBounds(n.KeySpan) {
			t.Fatalf("%s: span out of bounds: %+v %+v (len %d)", n, n.Span, n.KeySpan, len(data))
		}
		if n.Span.IsZero() {
			return true
		}
		switch {
		case isScalar(n.Kind):
			text := spanText(data, n.Span)
			f, err := Parse([]byte("v = " + text + "\n"))
			if err != nil {
				t.Fatalf("%s: value text %q does not parse: %v", n, text, err)
			}
			got := f.Root.Children[0].Children[0]
			if got.Kind != n.Kind || got.Value != n.Value {
				t.Fatalf("value text %q parses to %v %q, want %v %q", text, got.Kind, got.Value, n.Kind, n.Value)
			}
		case n.Kind == KeyValueNode:
			v := n.Children[0]
			if n.KeySpan.Start != n.Span.Start || v.Span.End != n.Span.End {
				t.Fatalf("%s: pair span %+v does not run from key %+v to value %+v", n, n.Span, n.KeySpan, v.Span)
			}
		}
		if n.Kind == KeyValueNode || n.Kind == TableNode || n.Kind == ArrayTableNode {
			text := spanText(data, n.KeySpan)
			f, err := Parse([]byte(text + " = 0\n"))
			if err != nil {
				t.Fatalf("%s: key text %q does not parse: %v", n, text, err)
			}
			leaf := f.Root
			for leaf.Kind != KeyValueNode {
				leaf = leaf.Children[0]
			}
			if leaf.Key[0] != n.Key[len(n.Key)-1] {
				t.Fatalf("key text %q ends in %q, want %q", text, leaf.Key[0], n.Key[len(n.Key)-1])
			}
		}
		return true
	})
}

func TestSpanInvariants(t *testing.T) {
	for _, in := range []string{spanDoc, "\uFEFF" + strings.ReplaceAll(spanDoc, "\n", "\r\n")} {
		f, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		checkSpans(t, []byte(in), f.Root)
	}
}

// TestSpanTestSuite checks the span invariants over every valid document in
// the toml-test suite.
func TestSpanTestSuite(t *testing.T) {
	valid, _ := loadTestFiles(t)
	for _, file := range valid {
		data, err := os.ReadFile(filepath.Join(testSuiteDir, "tests", file))
		if err != nil {
			t.Fatal(err)
		}
		f, err := Parse(data)
		if err != nil {
			continue // the conformance test reports parse failures
		}
		t.Run(file, func(t *testing.T) {
			checkSpans(t, data, f.Root)
			checkLineCol(t, data, f.Root)
		})
	}
}

func TestSpanColumnsAcrossRuneChunks(t *testing.T) {
	for _, size := range []int{runeChunk - 1, runeChunk, runeChunk + 1, 2 * runeChunk, 4 * runeChunk} {
		for _, prefix := range []string{"", "é"} {
			// s = "é…x", total size bytes, no trailing newline.
			body := strings.Repeat("x", size-len(`s = ""`)-len(prefix))
			doc := `s = "` + prefix + body + `"`
			f, err := Parse([]byte(doc))
			if err != nil {
				t.Fatal(err)
			}
			v := f.Root.Children[0].Children[0]
			if want := utf8.RuneCountInString(doc) + 1; v.Span.End.Column != want || v.Span.End.Offset != len(doc) {
				t.Errorf("size %d prefix %q: End = %+v, want column %d offset %d", size, prefix, v.Span.End, want, len(doc))
			}
			if mid := v.Span.Start.Column; mid != 5 {
				t.Errorf("size %d prefix %q: Start column = %d, want 5", size, prefix, mid)
			}
		}
	}
}

func TestErrorOffsetsInInput(t *testing.T) {
	const pre = "\xef\xbb\xbfa = 1\r\nb = 1\r\n"
	for _, c := range []struct {
		doc  string
		want string // the input text at the error's Offset
	}{
		{pre + "c = @\r\n", "@\r\n"},
		{pre + "c = 1\r\nc = 2\r\n", "c = 2\r\n"},
		{pre + "c = 1e400\r\n", "\r\n"},
		{pre + "c = 1\rd = 2\n", "\rd = 2\n"},
	} {
		var m map[string]any
		err := Unmarshal([]byte(c.doc), &m)
		var se *SyntaxError
		var de *DuplicateKeyError
		var pos Position
		switch {
		case errors.As(err, &se):
			pos = se.Pos
		case errors.As(err, &de):
			pos = de.Pos
		default:
			t.Fatalf("%q: err = %v", c.doc, err)
		}
		if got := c.doc[pos.Offset:]; got != c.want {
			t.Errorf("%q: %v: text at Offset %d = %q, want %q", c.doc, err, pos.Offset, got, c.want)
		}
	}

	doc := pre + "c = 300\r\n"
	var v struct{ C int8 }
	var oe *OverflowError
	if err := Unmarshal([]byte(doc), &v); !errors.As(err, &oe) || !strings.HasSuffix(doc[:oe.Pos.Offset], "300") {
		t.Errorf("OverflowError = %v at %+v", err, oe)
	}

	// A scalar's Pos is where it ends, as is its Span.
	f, err := Parse([]byte(pre + "[t]\r\nx = 'v'\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	Walk(f.Root, func(n *Node) bool {
		if len(n.Children) == 0 && n.Kind != TableNode && n.Pos != n.Span.End {
			t.Errorf("%v: Pos %+v, Span.End %+v", n, n.Pos, n.Span.End)
		}
		return true
	})
}
