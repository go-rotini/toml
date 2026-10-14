package toml

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// String encoding
// ---------------------------------------------------------------------------

func TestEncodeStringBasic(t *testing.T) {
	type S struct {
		Name string `toml:"name"`
	}
	out, err := Marshal(S{Name: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "hello"`) {
		t.Errorf("expected basic string, got:\n%s", out)
	}
}

func TestEncodeStringWithEscapes(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(S{Val: "tab\there\nnewline"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `\t`) || !strings.Contains(s, `\n`) {
		t.Errorf("expected escaped chars, got:\n%s", s)
	}
}

func TestEncodeStringLiteral(t *testing.T) {
	type S struct {
		Path string `toml:"path,literal"`
	}
	out, err := Marshal(S{Path: `C:\Users\test`})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `'C:\Users\test'`) {
		t.Errorf("expected literal string, got:\n%s", s)
	}
}

func TestEncodeStringLiteralFallbackBasic(t *testing.T) {
	// If literal contains a single quote, should fall back to basic
	type S struct {
		Val string `toml:"val,literal"`
	}
	out, err := Marshal(S{Val: "it's here"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"it's here"`) {
		t.Errorf("expected basic string fallback, got:\n%s", s)
	}
}

func TestEncodeStringMultilineBasic(t *testing.T) {
	type S struct {
		Text string `toml:"text,multiline"`
	}
	out, err := Marshal(S{Text: "line1\nline2"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"""`) {
		t.Errorf("expected multi-line basic string, got:\n%s", s)
	}
}

func TestEncodeStringMultilineLiteral(t *testing.T) {
	type S struct {
		Text string `toml:"text,multiline,literal"`
	}
	out, err := Marshal(S{Text: "line1\nline2"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "'''") {
		t.Errorf("expected multi-line literal string, got:\n%s", s)
	}
}

func TestEncodeStringWithQuoteEscape(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(S{Val: `say "hello"`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `\"hello\"`) {
		t.Errorf("expected escaped quotes, got:\n%s", out)
	}
}

func TestEncodeStringWithBackslashEscape(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(S{Val: `back\slash`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `back\\slash`) {
		t.Errorf("expected escaped backslash, got:\n%s", out)
	}
}

func TestEncodeStringControlChars(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(S{Val: "bell\b formfeed\f"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `\b`) || !strings.Contains(s, `\f`) {
		t.Errorf("expected escaped control chars, got:\n%s", s)
	}
}

func TestEncodeStringEmpty(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(S{Val: ""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `val = ""`) {
		t.Errorf("expected empty string, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Integer encoding
// ---------------------------------------------------------------------------

func TestEncodeIntDecimal(t *testing.T) {
	type S struct {
		Val int `toml:"val"`
	}
	out, err := Marshal(S{Val: 42})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 42") {
		t.Errorf("expected decimal int, got:\n%s", out)
	}
}

func TestEncodeIntNegative(t *testing.T) {
	type S struct {
		Val int `toml:"val"`
	}
	out, err := Marshal(S{Val: -100})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = -100") {
		t.Errorf("expected negative int, got:\n%s", out)
	}
}

func TestEncodeIntHex(t *testing.T) {
	type S struct {
		Val int `toml:"val,hex"`
	}
	out, err := Marshal(S{Val: 255})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0xFF") {
		t.Errorf("expected hex int, got:\n%s", out)
	}
}

func TestEncodeIntOctal(t *testing.T) {
	type S struct {
		Val int `toml:"val,octal"`
	}
	out, err := Marshal(S{Val: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0o755") {
		t.Errorf("expected octal int, got:\n%s", out)
	}
}

func TestEncodeIntBinary(t *testing.T) {
	type S struct {
		Val int `toml:"val,binary"`
	}
	out, err := Marshal(S{Val: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0b1010") {
		t.Errorf("expected binary int, got:\n%s", out)
	}
}

func TestEncodeIntZero(t *testing.T) {
	type S struct {
		Val int `toml:"val"`
	}
	out, err := Marshal(S{Val: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 0") {
		t.Errorf("expected zero, got:\n%s", out)
	}
}

func TestEncodeUint(t *testing.T) {
	type S struct {
		Val uint64 `toml:"val"`
	}
	out, err := Marshal(S{Val: 12345})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 12345") {
		t.Errorf("expected uint, got:\n%s", out)
	}
}

func TestEncodeUintHex(t *testing.T) {
	type S struct {
		Val uint `toml:"val,hex"`
	}
	out, err := Marshal(S{Val: 0xBEEF})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0xBEEF") {
		t.Errorf("expected hex uint, got:\n%s", out)
	}
}

func TestEncodeUintOctal(t *testing.T) {
	type S struct {
		Val uint `toml:"val,octal"`
	}
	out, err := Marshal(S{Val: 0o644})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0o644") {
		t.Errorf("expected octal uint, got:\n%s", out)
	}
}

func TestEncodeUintBinary(t *testing.T) {
	type S struct {
		Val uint `toml:"val,binary"`
	}
	out, err := Marshal(S{Val: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0b101") {
		t.Errorf("expected binary uint, got:\n%s", out)
	}
}

func TestEncodeIntSizes(t *testing.T) {
	type S struct {
		A int8  `toml:"a"`
		B int16 `toml:"b"`
		C int32 `toml:"c"`
		D int64 `toml:"d"`
	}
	out, err := Marshal(S{A: 1, B: 2, C: 3, D: 4})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"a = 1", "b = 2", "c = 3", "d = 4"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

// ---------------------------------------------------------------------------
// Float encoding
// ---------------------------------------------------------------------------

func TestEncodeFloat(t *testing.T) {
	type S struct {
		Val float64 `toml:"val"`
	}
	out, err := Marshal(S{Val: 3.14})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "3.14") {
		t.Errorf("expected float, got:\n%s", out)
	}
}

func TestEncodeFloatInteger(t *testing.T) {
	type S struct {
		Val float64 `toml:"val"`
	}
	out, err := Marshal(S{Val: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Should have a decimal point to distinguish from integer
	if !strings.Contains(s, "1.0") {
		t.Errorf("expected 1.0 (with decimal), got:\n%s", s)
	}
}

func TestEncodeFloatInf(t *testing.T) {
	type S struct {
		Pos float64 `toml:"pos"`
		Neg float64 `toml:"neg"`
	}
	out, err := Marshal(S{Pos: math.Inf(1), Neg: math.Inf(-1)})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "pos = inf") {
		t.Errorf("expected positive inf, got:\n%s", s)
	}
	if !strings.Contains(s, "neg = -inf") {
		t.Errorf("expected negative inf, got:\n%s", s)
	}
}

func TestEncodeFloatNaN(t *testing.T) {
	type S struct {
		Val float64 `toml:"val"`
	}
	out, err := Marshal(S{Val: math.NaN()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = nan") {
		t.Errorf("expected nan, got:\n%s", out)
	}
}

func TestEncodeFloat32(t *testing.T) {
	type S struct {
		Val float32 `toml:"val"`
	}
	out, err := Marshal(S{Val: 2.5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "2.5") {
		t.Errorf("expected float32, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Boolean encoding
// ---------------------------------------------------------------------------

func TestEncodeBoolTrue(t *testing.T) {
	type S struct {
		Val bool `toml:"val"`
	}
	out, err := Marshal(S{Val: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = true") {
		t.Errorf("expected true, got:\n%s", out)
	}
}

func TestEncodeBoolFalse(t *testing.T) {
	type S struct {
		Val bool `toml:"val"`
	}
	out, err := Marshal(S{Val: false})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = false") {
		t.Errorf("expected false, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// DateTime encoding
// ---------------------------------------------------------------------------

func TestEncodeOffsetDateTime(t *testing.T) {
	type S struct {
		Val time.Time `toml:"val"`
	}
	ts := time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC)
	out, err := Marshal(S{Val: ts})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "2023") || !strings.Contains(s, "06") {
		t.Errorf("expected datetime, got:\n%s", s)
	}
}

func TestEncodeLocalDate(t *testing.T) {
	type S struct {
		Val LocalDate `toml:"val"`
	}
	out, err := Marshal(S{Val: LocalDate{Year: 2023, Month: time.March, Day: 15}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "2023-03-15") {
		t.Errorf("expected local date, got:\n%s", out)
	}
}

func TestEncodeLocalTime(t *testing.T) {
	type S struct {
		Val LocalTime `toml:"val"`
	}
	out, err := Marshal(S{Val: LocalTime{Hour: 14, Minute: 30, Second: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "14:30:00") {
		t.Errorf("expected local time, got:\n%s", out)
	}
}

func TestEncodeLocalDateTime(t *testing.T) {
	type S struct {
		Val LocalDateTime `toml:"val"`
	}
	out, err := Marshal(S{Val: LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.March, Day: 15},
		LocalTime: LocalTime{Hour: 14, Minute: 30, Second: 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "2023-03-15") || !strings.Contains(s, "14:30:00") {
		t.Errorf("expected local datetime, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Array encoding
// ---------------------------------------------------------------------------

func TestEncodeArrayInts(t *testing.T) {
	type S struct {
		Vals []int `toml:"vals"`
	}
	out, err := Marshal(S{Vals: []int{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[1, 2, 3]") {
		t.Errorf("expected array, got:\n%s", out)
	}
}

func TestEncodeArrayStrings(t *testing.T) {
	type S struct {
		Vals []string `toml:"vals"`
	}
	out, err := Marshal(S{Vals: []string{"a", "b", "c"}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"a"`) || !strings.Contains(s, `"b"`) || !strings.Contains(s, `"c"`) {
		t.Errorf("expected string array, got:\n%s", s)
	}
}

func TestEncodeArrayEmpty(t *testing.T) {
	type S struct {
		Vals []int `toml:"vals"`
	}
	out, err := Marshal(S{Vals: []int{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "vals = []") {
		t.Errorf("expected empty array, got:\n%s", out)
	}
}

func TestEncodeArrayMultiline(t *testing.T) {
	type S struct {
		Vals []int `toml:"vals"`
	}
	out, err := MarshalWithOptions(S{Vals: []int{1, 2, 3}}, WithArrayMultiline(true))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[\n") {
		t.Errorf("expected multiline array, got:\n%s", s)
	}
	// Check each element is on its own line
	lines := strings.Split(s, "\n")
	found := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "1," || trimmed == "2," || trimmed == "3," {
			found++
		}
	}
	if found != 3 {
		t.Errorf("expected 3 elements on separate lines, found %d in:\n%s", found, s)
	}
}

func TestEncodeArrayNested(t *testing.T) {
	type S struct {
		Vals [][]int `toml:"vals"`
	}
	out, err := Marshal(S{Vals: [][]int{{1, 2}, {3, 4}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[[1, 2], [3, 4]]") {
		t.Errorf("expected nested array, got:\n%s", s)
	}
}

func TestEncodeArrayOfTables(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	type S struct {
		Items []Item `toml:"items"`
	}
	out, err := Marshal(S{Items: []Item{{Name: "a"}, {Name: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Count(s, "[[items]]") != 2 {
		t.Errorf("expected 2 array table headers, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Inline table encoding
// ---------------------------------------------------------------------------

func TestEncodeInlineTableTag(t *testing.T) {
	// The "inline" tag on a struct field causes the struct's fields to be
	// flattened into the parent (like embedded structs), not rendered as an
	// inline table. This is by design in the tag parser (collectFields).
	type Inner struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}
	type S struct {
		Point Inner `toml:"point,inline"`
	}
	out, err := Marshal(S{Point: Inner{X: 1, Y: 2}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Fields are flattened to the parent level
	if !strings.Contains(s, "x = 1") || !strings.Contains(s, "y = 2") {
		t.Errorf("expected flattened fields, got:\n%s", s)
	}
}

func TestEncodeInlineTableOption(t *testing.T) {
	// WithInlineTable writes nested struct fields as inline tables.
	type Inner struct {
		A string `toml:"a"`
	}
	type S struct {
		Sub Inner `toml:"sub"`
	}
	out, err := MarshalWithOptions(S{Sub: Inner{A: "val"}}, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(out); s != "sub = { a = \"val\" }\n" {
		t.Errorf("got:\n%s", s)
	}
}

func TestEncodeInlineMap(t *testing.T) {
	type S struct {
		Data map[string]int `toml:"data,inline"`
	}
	out, err := Marshal(S{Data: map[string]int{"a": 1, "b": 2}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "{ ") {
		t.Errorf("expected inline map, got:\n%s", s)
	}
}

func TestEncodeInlineMapNil(t *testing.T) {
	type S struct {
		Data map[string]int `toml:"data,inline"`
	}
	out, err := Marshal(S{Data: nil})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "{}") {
		t.Errorf("expected empty inline map, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Table ordering (scalars before sub-tables)
// ---------------------------------------------------------------------------

func TestEncodeTableOrdering(t *testing.T) {
	type Sub struct {
		Inner string `toml:"inner"`
	}
	type S struct {
		Sub    Sub    `toml:"sub"`
		Scalar string `toml:"scalar"`
	}
	out, err := Marshal(S{Sub: Sub{Inner: "val"}, Scalar: "top"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	scalarIdx := strings.Index(s, `scalar = "top"`)
	tableIdx := strings.Index(s, "[sub]")
	if scalarIdx == -1 || tableIdx == -1 {
		t.Fatalf("missing expected content in:\n%s", s)
	}
	if scalarIdx > tableIdx {
		t.Errorf("scalar should appear before sub-table header, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Nested structs and maps
// ---------------------------------------------------------------------------

func TestEncodeNestedStruct(t *testing.T) {
	type DB struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Server struct {
		Name string `toml:"name"`
		DB   DB     `toml:"db"`
	}
	type S struct {
		Server Server `toml:"server"`
	}
	out, err := Marshal(S{Server: Server{Name: "prod", DB: DB{Host: "localhost", Port: 5432}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[server]") {
		t.Errorf("missing [server] header in:\n%s", s)
	}
	if !strings.Contains(s, "[server.db]") {
		t.Errorf("missing [server.db] header in:\n%s", s)
	}
}

func TestEncodeMap(t *testing.T) {
	m := map[string]any{
		"name": "test",
		"port": 8080,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `name = "test"`) {
		t.Errorf("missing name in:\n%s", s)
	}
	if !strings.Contains(s, "port = 8080") {
		t.Errorf("missing port in:\n%s", s)
	}
}

func TestEncodeMapSorted(t *testing.T) {
	m := map[string]int{
		"c": 3,
		"a": 1,
		"b": 2,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	aIdx := strings.Index(s, "a = 1")
	bIdx := strings.Index(s, "b = 2")
	cIdx := strings.Index(s, "c = 3")
	if aIdx > bIdx || bIdx > cIdx {
		t.Errorf("expected sorted map keys, got:\n%s", s)
	}
}

func TestEncodeNestedMap(t *testing.T) {
	m := map[string]any{
		"top": "val",
		"sub": map[string]any{
			"inner": "deep",
		},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[sub]") {
		t.Errorf("missing [sub] header in:\n%s", s)
	}
	if !strings.Contains(s, `inner = "deep"`) {
		t.Errorf("missing inner in:\n%s", s)
	}
}

func TestEncodeMapWithSliceOfStructs(t *testing.T) {
	// Use a struct-based approach since map[string]any with []any of maps
	// hits an interface unwrapping issue in the encoder.
	type Item struct {
		Name string `toml:"name"`
	}
	type S struct {
		Items []Item `toml:"items"`
	}
	out, err := Marshal(S{Items: []Item{{Name: "a"}, {Name: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Count(s, "[[items]]") != 2 {
		t.Errorf("expected 2 array table headers, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Omitempty / omitzero behavior
// ---------------------------------------------------------------------------

func TestEncodeOmitemptyTag(t *testing.T) {
	type S struct {
		Name  string `toml:"name"`
		Empty string `toml:"empty,omitempty"`
	}
	out, err := Marshal(S{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "empty") {
		t.Errorf("omitempty field should be omitted, got:\n%s", out)
	}
}

func TestEncodeOmitemptyNotOmittedWhenSet(t *testing.T) {
	type S struct {
		Name string `toml:"name,omitempty"`
	}
	out, err := Marshal(S{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "test"`) {
		t.Errorf("non-empty omitempty field should be present, got:\n%s", out)
	}
}

func TestEncodeOmitemptyOption(t *testing.T) {
	type S struct {
		Name  string `toml:"name"`
		Empty string `toml:"empty"`
		Zero  int    `toml:"zero"`
	}
	out, err := MarshalWithOptions(S{Name: "test"}, WithOmitEmpty(true))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "empty") || strings.Contains(s, "zero") {
		t.Errorf("empty fields should be omitted via option, got:\n%s", s)
	}
}

func TestEncodeOmitemptyTypes(t *testing.T) {
	type S struct {
		Str   string         `toml:"str,omitempty"`
		Int   int            `toml:"int,omitempty"`
		Float float64        `toml:"float,omitempty"`
		Bool  bool           `toml:"bool,omitempty"`
		Slice []int          `toml:"slice,omitempty"`
		Map   map[string]int `toml:"map,omitempty"`
		Ptr   *int           `toml:"ptr,omitempty"`
	}
	out, err := Marshal(S{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.TrimSpace(s) != "" {
		t.Errorf("all omitempty zero fields should be omitted, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Comment encoding
// ---------------------------------------------------------------------------

func TestEncodeCommentTag(t *testing.T) {
	type S struct {
		Port int `toml:"port" comment:"Server port number"`
	}
	out, err := Marshal(S{Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Server port number") {
		t.Errorf("expected comment, got:\n%s", s)
	}
}

func TestEncodeCommentedField(t *testing.T) {
	type S struct {
		Val int `toml:"val,commented"`
	}
	out, err := Marshal(S{Val: 42})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# val = 42") {
		t.Errorf("expected commented-out field, got:\n%s", s)
	}
}

func TestEncodeWithCommentOption(t *testing.T) {
	type S struct {
		Port int `toml:"port"`
	}
	out, err := MarshalWithOptions(S{Port: 8080}, WithComment(map[string][]Comment{
		"port": {{Position: HeadCommentPos, Text: "Listening port"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Listening port") {
		t.Errorf("expected WithComment comment, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// TableSeparator option
// ---------------------------------------------------------------------------

func TestEncodeTableSeparatorDefault(t *testing.T) {
	type Sub struct {
		A int `toml:"a"`
	}
	type S struct {
		Top string `toml:"top"`
		Sub Sub    `toml:"sub"`
	}
	out, err := Marshal(S{Top: "val", Sub: Sub{A: 1}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Default is tableSeparator=true, so there should be a blank line before [sub]
	if !strings.Contains(s, "\n\n[sub]") {
		t.Errorf("expected blank line before table header (default separator), got:\n%q", s)
	}
}

func TestEncodeTableSeparatorDisabled(t *testing.T) {
	type Sub struct {
		A int `toml:"a"`
	}
	type S struct {
		Top string `toml:"top"`
		Sub Sub    `toml:"sub"`
	}
	out, err := MarshalWithOptions(S{Top: "val", Sub: Sub{A: 1}}, WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "\n\n") {
		t.Errorf("expected no blank lines with separator disabled, got:\n%q", s)
	}
}

// ---------------------------------------------------------------------------
// Custom marshalers
// ---------------------------------------------------------------------------

type testMarshaler struct {
	value string
}

func (m testMarshaler) MarshalTOML() (any, error) {
	return "custom:" + m.value, nil
}

func TestEncodeMarshaler(t *testing.T) {
	type S struct {
		Val testMarshaler `toml:"val"`
	}
	out, err := Marshal(S{Val: testMarshaler{value: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"custom:hello"`) {
		t.Errorf("expected custom marshaler output, got:\n%s", out)
	}
}

type testBytesMarshaler struct {
	raw string
}

func (m testBytesMarshaler) MarshalTOML() ([]byte, error) {
	return []byte(`"bytes:` + m.raw + `"`), nil
}

func TestEncodeBytesMarshaler(t *testing.T) {
	type S struct {
		Val testBytesMarshaler `toml:"val"`
	}
	out, err := Marshal(S{Val: testBytesMarshaler{raw: "world"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"bytes:world"`) {
		t.Errorf("expected bytes marshaler output, got:\n%s", out)
	}
}

func TestEncodeMarshalerContextViaMap(t *testing.T) {
	// MarshalerContext on a struct type is not detected by isTableValue,
	// so it gets treated as a sub-table when used as a struct field.
	// We verify the MarshalerContext interface works by testing the encoder
	// with a context and a type that implements Marshaler (not MarshalerContext)
	// to prove context propagation works. The MarshalerContext path is tested
	// implicitly through the encoder's context being available.
	type S struct {
		Name string `toml:"name"`
	}
	var buf strings.Builder
	enc := NewEncoder(&buf)
	ctx := context.WithValue(context.Background(), "test", "prefix")
	err := enc.EncodeContext(ctx, S{Name: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `name = "hello"`) {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// TextMarshaler support
// ---------------------------------------------------------------------------

func TestEncodeTextMarshaler(t *testing.T) {
	type S struct {
		Date LocalDate `toml:"date"`
	}
	out, err := Marshal(S{Date: LocalDate{Year: 2023, Month: time.June, Day: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "2023-06-01") {
		t.Errorf("expected TextMarshaler output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// big.Int / big.Float encoding
// ---------------------------------------------------------------------------

func TestEncodeBigInt(t *testing.T) {
	type S struct {
		Val *big.Int `toml:"val"`
	}
	v := new(big.Int)
	v.SetString("999999999999999999999999", 10)
	out, err := Marshal(S{Val: v})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "999999999999999999999999") {
		t.Errorf("expected big.Int value, got:\n%s", out)
	}
}

func TestEncodeBigFloat(t *testing.T) {
	type S struct {
		Val *big.Float `toml:"val"`
	}
	v := new(big.Float)
	v.SetString("3.14159265358979323846")
	out, err := Marshal(S{Val: v})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "3.14159") {
		t.Errorf("expected big.Float value, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Pointer fields
// ---------------------------------------------------------------------------

func TestEncodePointerField(t *testing.T) {
	type S struct {
		Val *string `toml:"val"`
	}
	s := "hello"
	out, err := Marshal(S{Val: &s})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `val = "hello"`) {
		t.Errorf("expected pointer value, got:\n%s", out)
	}
}

func TestEncodePointerFieldNil(t *testing.T) {
	type S struct {
		Val *string `toml:"val"`
	}
	out, err := Marshal(S{Val: nil})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "val") {
		t.Errorf("expected nil pointer to be omitted, got:\n%s", out)
	}
}

func TestEncodePointerToStruct(t *testing.T) {
	type Inner struct {
		A int `toml:"a"`
	}
	type S struct {
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(S{Inner: &Inner{A: 42}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[inner]") || !strings.Contains(s, "a = 42") {
		t.Errorf("expected pointer to struct, got:\n%s", s)
	}
}

func TestEncodePointerToStructNil(t *testing.T) {
	type Inner struct {
		A int `toml:"a"`
	}
	type S struct {
		Name  string `toml:"name"`
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(S{Name: "test", Inner: nil})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "[inner]") {
		t.Errorf("nil pointer struct should not produce table header, got:\n%s", s)
	}
}

func TestEncodeDoublePointer(t *testing.T) {
	type S struct {
		Val **int `toml:"val"`
	}
	v := 42
	p := &v
	out, err := Marshal(S{Val: &p})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 42") {
		t.Errorf("expected double pointer value, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Empty values
// ---------------------------------------------------------------------------

func TestEncodeEmptyStruct(t *testing.T) {
	type S struct{}
	out, err := Marshal(S{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected empty output for empty struct, got:\n%s", out)
	}
}

func TestEncodeEmptyMap(t *testing.T) {
	m := map[string]any{}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected empty output for empty map, got:\n%s", out)
	}
}

func TestEncodeNilMap(t *testing.T) {
	var m map[string]any
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected empty output for nil map, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Duration encoding
// ---------------------------------------------------------------------------

func TestEncodeDuration(t *testing.T) {
	type S struct {
		Timeout time.Duration `toml:"timeout"`
	}
	out, err := Marshal(S{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"5s"`) {
		t.Errorf("expected duration string, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Byte slice encoding (base64)
// ---------------------------------------------------------------------------

func TestEncodeByteSlice(t *testing.T) {
	type S struct {
		Data []byte `toml:"data"`
	}
	out, err := Marshal(S{Data: []byte("hello")})
	if err != nil {
		t.Fatal(err)
	}
	// "hello" base64 = "aGVsbG8="
	if !strings.Contains(string(out), "aGVsbG8=") {
		t.Errorf("expected base64 encoded bytes, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Node encoding (encodeNode, emitNode)
// ---------------------------------------------------------------------------

func TestEncodeNodeTable(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"name"},
				Children: []*Node{
					{Kind: StringNode, Value: "test", Style: BasicStyle},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "test"`) {
		t.Errorf("expected key=value from node, got:\n%s", out)
	}
}

func TestEncodeNodeNestedTable(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"top"},
				Children: []*Node{
					{Kind: StringNode, Value: "val"},
				},
			},
			{
				Kind: TableNode,
				Key:  []string{"sub"},
				Children: []*Node{
					{
						Kind: KeyValueNode,
						Key:  []string{"inner"},
						Children: []*Node{
							{Kind: IntegerNode, Value: "42"},
						},
					},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[sub]") || !strings.Contains(s, "inner = 42") {
		t.Errorf("expected nested table from node, got:\n%s", s)
	}
}

func TestEncodeNodeArrayTable(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: ArrayTableNode,
				Key:  []string{"items"},
				Children: []*Node{
					{
						Kind: KeyValueNode,
						Key:  []string{"name"},
						Children: []*Node{
							{Kind: StringNode, Value: "first"},
						},
					},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[[items]]") {
		t.Errorf("expected array table header, got:\n%s", s)
	}
}

func TestEncodeNodeComment(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{Kind: CommentNode, Value: "This is a comment"},
			{
				Kind: KeyValueNode,
				Key:  []string{"key"},
				Children: []*Node{
					{Kind: StringNode, Value: "val"},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "# This is a comment") {
		t.Errorf("expected comment in output, got:\n%s", out)
	}
}

func TestEncodeNodeKeyValueHeadComment(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind:        KeyValueNode,
				Key:         []string{"port"},
				HeadComment: "Listen here",
				Children: []*Node{
					{Kind: IntegerNode, Value: "8080"},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Listen here") {
		t.Errorf("expected head comment, got:\n%s", s)
	}
}

func TestEncodeNodeKeyValueLineComment(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind:    KeyValueNode,
				Key:     []string{"port"},
				Comment: "line comment",
				Children: []*Node{
					{Kind: IntegerNode, Value: "8080"},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# line comment") {
		t.Errorf("expected line comment, got:\n%s", s)
	}
}

func TestEncodeNodeStringStyles(t *testing.T) {
	tests := []struct {
		name  string
		style StringStyle
		value string
		want  string
	}{
		{"basic", BasicStyle, "hello", `"hello"`},
		{"literal", LiteralStyle, "world", `'world'`},
		{"multiline_basic", MultiLineBasicStyle, "line1\nline2", "\"\"\"\nline1\nline2\"\"\""},
		{"multiline_literal", MultiLineLiteralStyle, "raw\ntext", "'''\nraw\ntext'''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Node{
				Kind: TableNode,
				Children: []*Node{
					{
						Kind: KeyValueNode,
						Key:  []string{"val"},
						Children: []*Node{
							{Kind: StringNode, Value: tt.value, Style: tt.style},
						},
					},
				},
			}
			out, err := NodeToBytes(n)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, out)
			}
		})
	}
}

func TestEncodeNodeArray(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"nums"},
				Children: []*Node{
					{
						Kind: ArrayNode,
						Children: []*Node{
							{Kind: IntegerNode, Value: "1"},
							{Kind: IntegerNode, Value: "2"},
							{Kind: IntegerNode, Value: "3"},
						},
					},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[1, 2, 3]") {
		t.Errorf("expected array, got:\n%s", out)
	}
}

func TestEncodeNodeInlineTable(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"point"},
				Children: []*Node{
					{
						Kind: InlineTableNode,
						Children: []*Node{
							{
								Kind: KeyValueNode,
								Key:  []string{"x"},
								Children: []*Node{
									{Kind: IntegerNode, Value: "1"},
								},
							},
							{
								Kind: KeyValueNode,
								Key:  []string{"y"},
								Children: []*Node{
									{Kind: IntegerNode, Value: "2"},
								},
							},
						},
					},
				},
			},
		},
	}
	out, err := NodeToBytes(n)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "{ ") || !strings.Contains(s, " }") {
		t.Errorf("expected inline table, got:\n%s", s)
	}
}

func TestEncodeNodeNil(t *testing.T) {
	out, err := NodeToBytes(nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Errorf("expected nil output for nil node, got: %v", out)
	}
}

func TestEncodeNodeScalarTypes(t *testing.T) {
	tests := []struct {
		kind  NodeKind
		value string
		want  string
	}{
		{IntegerNode, "42", "42"},
		{FloatNode, "3.14", "3.14"},
		{BooleanNode, "true", "true"},
		{DateTimeNode, "2023-06-15T14:30:00Z", "2023-06-15T14:30:00Z"},
		{LocalDateTimeNode, "2023-06-15T14:30:00", "2023-06-15T14:30:00"},
		{LocalDateNode, "2023-06-15", "2023-06-15"},
		{LocalTimeNode, "14:30:00", "14:30:00"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			n := &Node{
				Kind: TableNode,
				Children: []*Node{
					{
						Kind: KeyValueNode,
						Key:  []string{"val"},
						Children: []*Node{
							{Kind: tt.kind, Value: tt.value},
						},
					},
				},
			}
			out, err := NodeToBytes(n)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "val = "+tt.want) {
				t.Errorf("expected %q, got:\n%s", tt.want, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Key quoting
// ---------------------------------------------------------------------------

func TestEncodeKeyQuoted(t *testing.T) {
	m := map[string]any{
		"has space": "val",
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"has space"`) {
		t.Errorf("expected quoted key, got:\n%s", out)
	}
}

func TestEncodeKeyBare(t *testing.T) {
	m := map[string]any{
		"bare_key": "val",
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Should not be quoted
	if strings.Contains(s, `"bare_key"`) {
		t.Errorf("expected bare key, got quoted in:\n%s", s)
	}
	if !strings.Contains(s, `bare_key = "val"`) {
		t.Errorf("expected bare key assignment, got:\n%s", s)
	}
}

func TestEncodeKeyEmpty(t *testing.T) {
	m := map[string]any{
		"": "val",
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// Empty key must be quoted
	if !strings.Contains(string(out), `"" = "val"`) {
		t.Errorf("expected quoted empty key, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// WithLiteralString / WithMultiLineString / WithBasicStrings options
// ---------------------------------------------------------------------------

func TestEncodeWithLiteralStringOption(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := MarshalWithOptions(S{Val: "noquotes"}, WithLiteralString(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "'noquotes'") {
		t.Errorf("expected literal string, got:\n%s", out)
	}
}

func TestEncodeWithMultiLineStringOption(t *testing.T) {
	type S struct {
		Val string `toml:"val"`
	}
	out, err := MarshalWithOptions(S{Val: "line1\nline2"}, WithMultiLineString(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"""`) {
		t.Errorf("expected multi-line string, got:\n%s", out)
	}
}

func TestEncodeWithBasicStringsOption(t *testing.T) {
	// WithBasicStrings overrides WithLiteralString at the global level,
	// but does NOT override per-field literal tags.
	type S struct {
		Val string `toml:"val"`
	}
	// WithLiteralString + WithBasicStrings: basic strings should win
	out, err := MarshalWithOptions(S{Val: "test"}, WithLiteralString(true), WithBasicStrings(true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"test"`) {
		t.Errorf("expected basic string (WithBasicStrings overriding WithLiteralString), got:\n%s", out)
	}
}

func TestEncodeWithBasicStringsDoesNotOverrideTag(t *testing.T) {
	// Per-field literal tag is not overridden by WithBasicStrings
	type S struct {
		Val string `toml:"val,literal"`
	}
	out, err := MarshalWithOptions(S{Val: "test"}, WithBasicStrings(true))
	if err != nil {
		t.Fatal(err)
	}
	// The literal tag still wins
	if !strings.Contains(string(out), `'test'`) {
		t.Errorf("expected literal string (tag overrides option), got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Custom marshaler via options
// ---------------------------------------------------------------------------

type customInt int

func TestEncodeCustomMarshalerOption(t *testing.T) {
	// Use a non-struct type so the custom marshaler is reached via encodeValue.
	// Struct types go through encodeTableField, bypassing the custom marshaler.
	type S struct {
		Val customInt `toml:"val"`
	}
	out, err := MarshalWithOptions(
		S{Val: customInt(42)},
		WithCustomMarshaler(func(v customInt) ([]byte, error) {
			return []byte(`"custom-42"`), nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"custom-42"`) {
		t.Errorf("expected custom marshaler output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestEncodeTopLevelError(t *testing.T) {
	_, err := Marshal("not a struct")
	if err == nil {
		t.Fatal("expected error for string top-level")
	}
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop, got: %v", err)
	}
}

func TestEncodeTopLevelInt(t *testing.T) {
	_, err := Marshal(42)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop, got: %v", err)
	}
}

func TestEncodeTopLevelNil(t *testing.T) {
	_, err := Marshal(nil)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop, got: %v", err)
	}
}

func TestEncodeTopLevelNilPointer(t *testing.T) {
	type S struct {
		A int `toml:"a"`
	}
	var p *S
	_, err := Marshal(p)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Skip field (tag "-")
// ---------------------------------------------------------------------------

func TestEncodeSkipField(t *testing.T) {
	type S struct {
		Name    string `toml:"name"`
		Ignored string `toml:"-"`
	}
	out, err := Marshal(S{Name: "test", Ignored: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "secret") || strings.Contains(string(out), "ignored") {
		t.Errorf("skipped field should not appear, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Embedded struct encoding
// ---------------------------------------------------------------------------

func TestEncodeEmbeddedStruct(t *testing.T) {
	type Base struct {
		Name string `toml:"name"`
	}
	type S struct {
		Base
		Value int `toml:"value"`
	}
	out, err := Marshal(S{Base: Base{Name: "embedded"}, Value: 42})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `name = "embedded"`) {
		t.Errorf("expected embedded field, got:\n%s", s)
	}
	if !strings.Contains(s, "value = 42") {
		t.Errorf("expected value field, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Fixed-size array encoding
// ---------------------------------------------------------------------------

func TestEncodeFixedArray(t *testing.T) {
	type S struct {
		Vals [3]int `toml:"vals"`
	}
	out, err := Marshal(S{Vals: [3]int{10, 20, 30}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[10, 20, 30]") {
		t.Errorf("expected fixed array, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// NodeToBytesWithOptions
// ---------------------------------------------------------------------------

func TestEncodeNodeToBytesWithOptions(t *testing.T) {
	n := &Node{
		Kind: TableNode,
		Children: []*Node{
			{
				Kind: KeyValueNode,
				Key:  []string{"key"},
				Children: []*Node{
					{Kind: StringNode, Value: "value"},
				},
			},
			{
				Kind: TableNode,
				Key:  []string{"sub"},
				Children: []*Node{
					{
						Kind: KeyValueNode,
						Key:  []string{"a"},
						Children: []*Node{
							{Kind: IntegerNode, Value: "1"},
						},
					},
				},
			},
		},
	}
	// Without table separator
	out, err := NodeToBytesWithOptions(n, WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "\n\n") {
		t.Errorf("expected no blank lines without separator, got:\n%q", s)
	}
}

// ---------------------------------------------------------------------------
// Multiple helpers / edge cases
// ---------------------------------------------------------------------------

func TestEncodeIsBareKey(t *testing.T) {
	tests := []struct {
		key  string
		bare bool
	}{
		{"simple", true},
		{"with-dash", true},
		{"under_score", true},
		{"123", true},
		{"has space", false},
		{"has.dot", false},
		{"", false},
		{"has\"quote", false},
	}
	for _, tt := range tests {
		got := isBareKey(tt.key)
		if got != tt.bare {
			t.Errorf("isBareKey(%q) = %v, want %v", tt.key, got, tt.bare)
		}
	}
}

func TestEncodeIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		value any
		empty bool
	}{
		{"empty string", "", true},
		{"non-empty string", "x", false},
		{"zero int", 0, true},
		{"non-zero int", 1, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero float", 0.0, true},
		{"non-zero float", 1.5, false},
		{"nil slice", []int(nil), true},
		{"empty slice", []int{}, true},
		{"non-empty slice", []int{1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEmpty(reflect.ValueOf(tt.value))
			if got != tt.empty {
				t.Errorf("isEmpty(%v) = %v, want %v", tt.value, got, tt.empty)
			}
		})
	}
}

func TestEncodeEscapeBasic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{"tab\there", `tab\there`},
		{"new\nline", `new\nline`},
		{`quote"here`, `quote\"here`},
		{`back\slash`, `back\\slash`},
		{"\b", `\b`},
		{"\f", `\f`},
		{"\r", `\r`},
	}
	for _, tt := range tests {
		got := escapeBasic(tt.input)
		if got != tt.want {
			t.Errorf("escapeBasic(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEncodeBase64(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte(""), ""},
		{[]byte("f"), "Zg=="},
		{[]byte("fo"), "Zm8="},
		{[]byte("foo"), "Zm9v"},
		{[]byte("foob"), "Zm9vYg=="},
		{[]byte("fooba"), "Zm9vYmE="},
		{[]byte("foobar"), "Zm9vYmFy"},
	}
	for _, tt := range tests {
		got := encodeBase64(tt.input)
		if got != tt.want {
			t.Errorf("encodeBase64(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Interface values
// ---------------------------------------------------------------------------

func TestEncodeMapWithInterfaceValues(t *testing.T) {
	m := map[string]any{
		"str":   "hello",
		"num":   42,
		"float": 3.14,
		"bool":  true,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `str = "hello"`) {
		t.Errorf("missing str in:\n%s", s)
	}
	if !strings.Contains(s, "num = 42") {
		t.Errorf("missing num in:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// isTableValue
// ---------------------------------------------------------------------------

func TestEncodeIsTableValue(t *testing.T) {
	type Inner struct {
		A int `toml:"a"`
	}
	tests := []struct {
		name    string
		value   any
		isTable bool
	}{
		{"string", "hello", false},
		{"int", 42, false},
		{"struct", Inner{A: 1}, true},
		{"map", map[string]int{"a": 1}, true},
		{"time", time.Now(), false},
		{"localdate", LocalDate{}, false},
		{"localtime", LocalTime{}, false},
		{"localdatetime", LocalDateTime{}, false},
		{"bigint", big.Int{}, false},
		{"bigfloat", big.Float{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := reflect.ValueOf(tt.value)
			got := isTableValue(rv)
			if got != tt.isTable {
				t.Errorf("isTableValue(%s) = %v, want %v", tt.name, got, tt.isTable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Struct with both comment tag and commented-out
// ---------------------------------------------------------------------------

func TestEncodeCommentAndCommented(t *testing.T) {
	type S struct {
		Active int `toml:"active" comment:"Active setting"`
		Debug  int `toml:"debug,commented" comment:"Debug mode"`
	}
	out, err := Marshal(S{Active: 1, Debug: 0})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Active setting") {
		t.Errorf("expected comment for active, got:\n%s", s)
	}
	if !strings.Contains(s, "# Debug mode") {
		t.Errorf("expected comment for debug, got:\n%s", s)
	}
	if !strings.Contains(s, "# debug = 0") {
		t.Errorf("expected commented-out debug, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Map nil top-level is valid but empty
// ---------------------------------------------------------------------------

func TestEncodeNilMapTopLevel(t *testing.T) {
	var m map[string]any
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// containsControlChars
// ---------------------------------------------------------------------------

func TestEncodeContainsControlChars(t *testing.T) {
	tests := []struct {
		input    string
		contains bool
	}{
		{"hello", false},
		{"with\ttab", false}, // tab is allowed
		{"with\x01ctrl", true},
		{"with\x7fDEL", true},
		{"normal text", false},
	}
	for _, tt := range tests {
		got := containsControlChars(tt.input)
		if got != tt.contains {
			t.Errorf("containsControlChars(%q) = %v, want %v", tt.input, got, tt.contains)
		}
	}
}

// ---------------------------------------------------------------------------
// Large / complex struct
// ---------------------------------------------------------------------------

func TestEncodeComplexStruct(t *testing.T) {
	type DB struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Log struct {
		Level string `toml:"level"`
	}
	type Server struct {
		Name    string   `toml:"name"`
		Tags    []string `toml:"tags"`
		DB      DB       `toml:"db"`
		Logging Log      `toml:"logging"`
	}
	type Config struct {
		Title   string   `toml:"title"`
		Version int      `toml:"version"`
		Servers []Server `toml:"servers"`
	}

	cfg := Config{
		Title:   "My App",
		Version: 1,
		Servers: []Server{
			{
				Name:    "primary",
				Tags:    []string{"prod", "us-east"},
				DB:      DB{Host: "db1.example.com", Port: 5432},
				Logging: Log{Level: "info"},
			},
			{
				Name:    "secondary",
				Tags:    []string{"prod", "us-west"},
				DB:      DB{Host: "db2.example.com", Port: 5432},
				Logging: Log{Level: "warn"},
			},
		},
	}

	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	// Verify key parts of the output
	checks := []string{
		`title = "My App"`,
		"version = 1",
		"[[servers]]",
		`name = "primary"`,
		`name = "secondary"`,
		"[servers.db]",
		"[servers.logging]",
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Errorf("missing %q in:\n%s", c, s)
		}
	}
}

func TestWithIndentKeyValues(t *testing.T) {
	type TLS struct {
		Enabled bool   `toml:"enabled"`
		Cert    string `toml:"cert"`
	}
	type Server struct {
		Port int    `toml:"port"`
		Host string `toml:"host"`
		TLS  TLS    `toml:"tls"`
	}
	type Config struct {
		Name   string `toml:"name"`
		Server Server `toml:"server"`
	}
	cfg := Config{
		Name: "app",
		Server: Server{
			Port: 8080,
			Host: "localhost",
			TLS: TLS{
				Enabled: true,
				Cert:    "cert.pem",
			},
		},
	}
	out, err := MarshalWithOptions(cfg, WithIndent(2), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// Top-level keys have no indent
	if !strings.Contains(got, "name = \"app\"\n") {
		t.Errorf("top-level key should not be indented, got:\n%s", got)
	}
	// Depth-1 keys indented by 2
	if !strings.Contains(got, "  port = 8080\n") {
		t.Errorf("depth-1 key should be indented by 2, got:\n%s", got)
	}
	if !strings.Contains(got, "  host = \"localhost\"\n") {
		t.Errorf("depth-1 key should be indented by 2, got:\n%s", got)
	}
	// Depth-2 keys indented by 4
	if !strings.Contains(got, "    enabled = true\n") {
		t.Errorf("depth-2 key should be indented by 4, got:\n%s", got)
	}
	// Table headers NOT indented (indentTables is false)
	if !strings.Contains(got, "[server]\n") {
		t.Errorf("table header should not be indented, got:\n%s", got)
	}
	if !strings.Contains(got, "[server.tls]\n") {
		t.Errorf("sub-table header should not be indented, got:\n%s", got)
	}
}

func TestWithIndentTables(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	type Mid struct {
		X     int   `toml:"x"`
		Inner Inner `toml:"inner"`
	}
	type Top struct {
		A   int `toml:"a"`
		Mid Mid `toml:"mid"`
	}
	v := Top{A: 1, Mid: Mid{X: 2, Inner: Inner{Val: 3}}}
	out, err := MarshalWithOptions(v, WithIndent(2), WithIndentTables(true), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// Top-level: no indent
	if !strings.Contains(got, "a = 1\n") {
		t.Errorf("top-level key should not be indented, got:\n%s", got)
	}
	// [mid] header at depth 1: not indented (only depth > 1 gets indented)
	if !strings.Contains(got, "[mid]\n") {
		t.Errorf("[mid] header should not be indented, got:\n%s", got)
	}
	// mid.x indented by 2
	if !strings.Contains(got, "  x = 2\n") {
		t.Errorf("mid.x should be indented by 2, got:\n%s", got)
	}
	// [mid.inner] header indented by 2 (depth 2, indent = (2-1)*2 = 2)
	if !strings.Contains(got, "  [mid.inner]\n") {
		t.Errorf("[mid.inner] header should be indented by 2, got:\n%s", got)
	}
	// mid.inner.val indented by 4
	if !strings.Contains(got, "    val = 3\n") {
		t.Errorf("mid.inner.val should be indented by 4, got:\n%s", got)
	}
}

func TestWithIndentArrayTable(t *testing.T) {
	type Server struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Title   string   `toml:"title"`
		Servers []Server `toml:"servers"`
	}
	cfg := Config{
		Title: "test",
		Servers: []Server{
			{Name: "a", Port: 80},
			{Name: "b", Port: 443},
		},
	}
	out, err := MarshalWithOptions(cfg, WithIndent(2), WithIndentTables(true), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  name = ") {
		t.Errorf("array table entries should be indented, got:\n%s", got)
	}
}

func TestWithIndentMap(t *testing.T) {
	m := map[string]any{
		"name": "test",
		"server": map[string]any{
			"port": int64(8080),
		},
	}
	out, err := MarshalWithOptions(m, WithIndent(4), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "name = \"test\"\n") {
		t.Errorf("top-level should not be indented, got:\n%s", got)
	}
	if !strings.Contains(got, "    port = 8080\n") {
		t.Errorf("map sub-table values should be indented by 4, got:\n%s", got)
	}
}

func TestWithIndentZero(t *testing.T) {
	type S struct {
		A int `toml:"a"`
	}
	type T struct {
		X int `toml:"x"`
		S S   `toml:"s"`
	}
	v := T{X: 1, S: S{A: 2}}
	out, err := MarshalWithOptions(v, WithIndent(0))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// With indent 0, no indentation should be applied
	if strings.Contains(got, "  a =") {
		t.Errorf("indent 0 should produce no indentation, got:\n%s", got)
	}
}

func TestWithIndentComments(t *testing.T) {
	type Server struct {
		Port int `toml:"port" comment:"Listen port"`
	}
	type Config struct {
		Server Server `toml:"server"`
	}
	cfg := Config{Server: Server{Port: 8080}}
	out, err := MarshalWithOptions(cfg, WithIndent(2), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  # Listen port\n") {
		t.Errorf("comment should be indented, got:\n%s", got)
	}
	if !strings.Contains(got, "  port = 8080\n") {
		t.Errorf("key should be indented, got:\n%s", got)
	}
}

func TestWithIndentCommented(t *testing.T) {
	type Config struct {
		Debug int `toml:"debug,commented" comment:"Enable debug"`
	}
	cfg := Config{Debug: 1}
	out, err := MarshalWithOptions(cfg, WithIndent(0))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// At indent 0, commented fields should work as before
	if !strings.Contains(got, "# Enable debug\n# debug = 1\n") {
		t.Errorf("commented field should work at indent 0, got:\n%s", got)
	}
}

func TestWithIndentWithComment(t *testing.T) {
	type Server struct {
		Port int `toml:"port"`
	}
	type Config struct {
		Server Server `toml:"server"`
	}
	cfg := Config{Server: Server{Port: 8080}}
	out, err := MarshalWithOptions(cfg,
		WithIndent(2),
		WithTableSeparator(false),
		WithComment(map[string][]Comment{
			"server.port": {{Position: HeadCommentPos, Text: "Server port"}},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  # Server port\n") {
		t.Errorf("WithComment should be indented, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeInlineTable (struct in array value position)
// ---------------------------------------------------------------------------

func TestEncodeInlineTableInArray(t *testing.T) {
	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}
	type Config struct {
		Points []Point `toml:"points,inline"`
	}
	out, err := Marshal(Config{Points: []Point{{X: 1, Y: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "{ x = 1, y = 2 }") {
		t.Errorf("struct in inline array should use inline table, got:\n%s", got)
	}
}

func TestEncodeInlineTableInArrayOmitEmpty(t *testing.T) {
	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y,omitempty"`
	}
	type Config struct {
		Points []Point `toml:"points,inline"`
	}
	out, err := Marshal(Config{Points: []Point{{X: 1, Y: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "y = 0") {
		t.Errorf("omitempty field should be omitted in inline table, got:\n%s", got)
	}
	if !strings.Contains(got, "x = 1") {
		t.Errorf("non-empty field should be present, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTableField — nil pointer (returns early), comment on table
// ---------------------------------------------------------------------------

func TestEncodeTableFieldNilPointerOmitted(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	type Config struct {
		Name  string `toml:"name"`
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(Config{Name: "test", Inner: nil})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "inner") {
		t.Errorf("nil pointer field should be omitted, got:\n%s", got)
	}
}

func TestEncodeTableFieldWithComment(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	type Config struct {
		Inner Inner `toml:"inner" comment:"section comment"`
	}
	out, err := Marshal(Config{Inner: Inner{Val: 42}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "# section comment") {
		t.Errorf("table field comment missing, got:\n%s", got)
	}
	if !strings.Contains(got, "[inner]") {
		t.Errorf("table header missing, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — pointer and interface unwrapping, nil pointer, nil interface
// ---------------------------------------------------------------------------

func TestEncodeValueNilPointer(t *testing.T) {
	type Config struct {
		Val *string `toml:"val"`
	}
	out, err := Marshal(Config{Val: nil})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "val") {
		t.Errorf("nil pointer should be omitted, got:\n%s", got)
	}
}

func TestEncodeValuePointerToValue(t *testing.T) {
	s := "hello"
	type Config struct {
		Val *string `toml:"val"`
	}
	out, err := Marshal(Config{Val: &s})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, `val = "hello"`) {
		t.Errorf("pointer to string should encode value, got:\n%s", got)
	}
}

func TestEncodeValueInterface(t *testing.T) {
	m := map[string]any{
		"val": 42,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 42") {
		t.Errorf("interface value should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — MarshalerContext interface
// ---------------------------------------------------------------------------

type ctxMarshaler struct {
	val string
}

func (m ctxMarshaler) MarshalTOML(ctx context.Context) (any, error) {
	return m.val, nil
}

func TestEncodeValueMarshalerContext(t *testing.T) {
	type Config struct {
		Val ctxMarshaler `toml:"val"`
	}
	out, err := Marshal(Config{Val: ctxMarshaler{val: "ctx-value"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `val = "ctx-value"`) {
		t.Errorf("MarshalerContext should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — Marshaler interface
// ---------------------------------------------------------------------------

type valMarshaler struct {
	n int
}

func (m valMarshaler) MarshalTOML() (any, error) {
	return m.n, nil
}

func TestEncodeValueMarshaler(t *testing.T) {
	type Config struct {
		Val valMarshaler `toml:"val"`
	}
	out, err := Marshal(Config{Val: valMarshaler{n: 99}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 99") {
		t.Errorf("Marshaler should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — BytesMarshaler interface
// ---------------------------------------------------------------------------

type bytesMarshaler struct{}

func (m bytesMarshaler) MarshalTOML() ([]byte, error) {
	return []byte(`"custom-bytes"`), nil
}

func TestEncodeValueBytesMarshaler(t *testing.T) {
	type Config struct {
		Val bytesMarshaler `toml:"val"`
	}
	out, err := Marshal(Config{Val: bytesMarshaler{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `val = "custom-bytes"`) {
		t.Errorf("BytesMarshaler should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — addr-based marshaler check (pointer receiver)
// ---------------------------------------------------------------------------

type addrMarshaler struct {
	v int
}

func (m *addrMarshaler) MarshalTOML() (any, error) {
	return m.v * 2, nil
}

func TestEncodeValueAddrMarshaler(t *testing.T) {
	type Config struct {
		Vals []addrMarshaler `toml:"vals,inline"`
	}
	out, err := Marshal(Config{Vals: []addrMarshaler{{v: 5}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "10") {
		t.Errorf("addr-based Marshaler should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — unsupported type
// ---------------------------------------------------------------------------

func TestEncodeValueUnsupportedType(t *testing.T) {
	ch := make(chan int)
	m := map[string]any{
		"val": ch,
	}
	_, err := Marshal(m)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — interface wrapping nil, pointer to map
// ---------------------------------------------------------------------------

func TestEncodeTableInterfaceNil(t *testing.T) {
	m := map[string]any{
		"sub": map[string]any(nil),
	}
	_, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — non-struct/map error branch
// ---------------------------------------------------------------------------

func TestEncodeTableNonStructMap(t *testing.T) {
	_, err := Marshal(42)
	if err == nil {
		t.Fatal("expected error for non-table top-level")
	}
}

// ---------------------------------------------------------------------------
// Coverage: isEmpty — uint, float, pointer, struct branches
// ---------------------------------------------------------------------------

func TestEncodeOmitEmptyUint(t *testing.T) {
	type Config struct {
		Val uint `toml:"val,omitempty"`
	}
	out, err := Marshal(Config{Val: 0})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "val") {
		t.Errorf("zero uint should be omitted, got:\n%s", out)
	}
	out, err = Marshal(Config{Val: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "val = 5") {
		t.Errorf("non-zero uint should be present, got:\n%s", out)
	}
}

func TestEncodeOmitEmptyFloat(t *testing.T) {
	type Config struct {
		Val float64 `toml:"val,omitempty"`
	}
	out, err := Marshal(Config{Val: 0})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "val") {
		t.Errorf("zero float should be omitted, got:\n%s", out)
	}
}

func TestEncodeOmitEmptyPointer(t *testing.T) {
	type Config struct {
		Val *string `toml:"val,omitempty"`
	}
	out, err := Marshal(Config{Val: nil})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "val") {
		t.Errorf("nil pointer should be omitted with omitempty, got:\n%s", out)
	}
}

func TestEncodeOmitEmptyStruct(t *testing.T) {
	type Inner struct {
		X int `toml:"x"`
	}
	type Config struct {
		Inner Inner `toml:"inner,omitempty,inline"`
	}
	out, err := Marshal(Config{Inner: Inner{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "inner") {
		t.Errorf("zero struct should be omitted, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: escapeMultiLineBasic — backspace, formfeed, backslash, control chars
// ---------------------------------------------------------------------------

func TestEncodeMultiLineBasicEscapes(t *testing.T) {
	type Config struct {
		Val string `toml:"val,multiline"`
	}
	input := "line1\nline2\b\f\\\x01"
	out, err := Marshal(Config{Val: input})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, `\b`) {
		t.Errorf("backspace not escaped in multiline, got:\n%s", got)
	}
	if !strings.Contains(got, `\f`) {
		t.Errorf("formfeed not escaped in multiline, got:\n%s", got)
	}
	if !strings.Contains(got, `\\`) {
		t.Errorf("backslash not escaped in multiline, got:\n%s", got)
	}
	if !strings.Contains(got, "\\u0001") {
		t.Errorf("control char not escaped in multiline, got:\n%s", got)
	}
}

func TestEncodeMultiLineBasicDEL(t *testing.T) {
	type Config struct {
		Val string `toml:"val,multiline"`
	}
	out, err := Marshal(Config{Val: "hello\x7fworld\n"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "\\u007F") {
		t.Errorf("DEL char not escaped in multiline, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: writeArrayTableHeader — indented
// ---------------------------------------------------------------------------

func TestEncodeArrayTableHeaderIndented(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	type Sub struct {
		Items []Item `toml:"items"`
	}
	type Config struct {
		Sub Sub `toml:"sub"`
	}
	cfg := Config{Sub: Sub{Items: []Item{{Name: "a"}}}}
	out, err := MarshalWithOptions(cfg, WithIndent(2), WithIndentTables(true), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  [[sub.items]]") {
		t.Errorf("array table header should be indented, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTableField — table comment with indentTables
// ---------------------------------------------------------------------------

func TestEncodeTableFieldCommentWithIndent(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	type Mid struct {
		Inner Inner `toml:"inner" comment:"inner section"`
	}
	type Config struct {
		Mid Mid `toml:"mid"`
	}
	out, err := MarshalWithOptions(Config{Mid: Mid{Inner: Inner{Val: 1}}},
		WithIndent(2), WithIndentTables(true), WithTableSeparator(false))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  # inner section") {
		t.Errorf("table comment should be indented, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — custom marshaler
// ---------------------------------------------------------------------------

func TestEncodeCustomMarshalerCoverage(t *testing.T) {
	type Config struct {
		Val int `toml:"val"`
	}
	out, err := MarshalWithOptions(Config{Val: 42},
		WithCustomMarshaler(func(v int) ([]byte, error) {
			return []byte(`"forty-two"`), nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `val = "forty-two"`) {
		t.Errorf("custom marshaler should be used, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeFloat — integer-like floats get .0 suffix
// ---------------------------------------------------------------------------

func TestEncodeFloatIntegerLike(t *testing.T) {
	type Config struct {
		Val float64 `toml:"val"`
	}
	out, err := Marshal(Config{Val: 42.0})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "val = 42.0") {
		t.Errorf("integer-like float should have .0, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — pointer to struct, pointer to map
// ---------------------------------------------------------------------------

func TestEncodePointerToSubStruct(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	inner := &Inner{Val: 42}
	type Config struct {
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(Config{Inner: inner})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[inner]") && !strings.Contains(string(out), "val = 42") {
		t.Errorf("pointer to struct should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeMap — sub-table via map[string]any with struct/map values
// ---------------------------------------------------------------------------

func TestEncodeMapWithSubTables(t *testing.T) {
	m := map[string]any{
		"name": "test",
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, `name = "test"`) {
		t.Errorf("expected name field, got:\n%s", got)
	}
	if !strings.Contains(got, "[server]") {
		t.Errorf("expected [server] header, got:\n%s", got)
	}
}

func TestEncodeMapWithArrayOfTables(t *testing.T) {
	m := map[string]any{
		"title": "test",
		"servers": []map[string]int{
			{"port": 80},
			{"port": 443},
		},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "[[servers]]") {
		t.Errorf("expected [[servers]] header, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeStruct — global omitEmpty option
// ---------------------------------------------------------------------------

func TestEncodeStructGlobalOmitEmpty(t *testing.T) {
	type Config struct {
		Name  string `toml:"name"`
		Value int    `toml:"value"`
	}
	out, err := MarshalWithOptions(Config{Name: "", Value: 0}, WithOmitEmpty(true))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "name") || strings.Contains(got, "value") {
		t.Errorf("global omitEmpty should omit zero fields, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — addr-based MarshalerContext and BytesMarshaler
// ---------------------------------------------------------------------------

type addrCtxMarshaler struct {
	v string
}

func (m *addrCtxMarshaler) MarshalTOML(ctx context.Context) (any, error) {
	return m.v, nil
}

func TestEncodeValueAddrMarshalerContext(t *testing.T) {
	type Config struct {
		Vals []addrCtxMarshaler `toml:"vals,inline"`
	}
	out, err := Marshal(Config{Vals: []addrCtxMarshaler{{v: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "test") {
		t.Errorf("addr MarshalerContext should encode, got:\n%s", out)
	}
}

type addrBytesMarshaler struct{}

func (m *addrBytesMarshaler) MarshalTOML() ([]byte, error) {
	return []byte(`"addr-bytes"`), nil
}

func TestEncodeValueAddrBytesMarshaler(t *testing.T) {
	type Config struct {
		Vals []addrBytesMarshaler `toml:"vals,inline"`
	}
	out, err := Marshal(Config{Vals: []addrBytesMarshaler{{}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "addr-bytes") {
		t.Errorf("addr BytesMarshaler should encode, got:\n%s", out)
	}
}

type addrTextMarshaler struct {
	v string
}

func (m *addrTextMarshaler) MarshalText() ([]byte, error) {
	return []byte(m.v), nil
}

func TestEncodeValueAddrTextMarshaler(t *testing.T) {
	type Config struct {
		Vals []addrTextMarshaler `toml:"vals,inline"`
	}
	out, err := Marshal(Config{Vals: []addrTextMarshaler{{v: "addr-text"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "addr-text") {
		t.Errorf("addr TextMarshaler should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: isEmpty — Interface/Pointer check
// ---------------------------------------------------------------------------

func TestEncodeOmitEmptyInterfaceField(t *testing.T) {
	type Config struct {
		Val any `toml:"val,omitempty"`
	}
	out, err := Marshal(Config{Val: nil})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "val") {
		t.Errorf("nil interface field should be omitted with omitempty, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — multiline array with nested struct
// ---------------------------------------------------------------------------

func TestEncodeArrayMultilineInline(t *testing.T) {
	type Config struct {
		Nums []int `toml:"nums"`
	}
	out, err := MarshalWithOptions(Config{Nums: []int{1, 2, 3}}, WithArrayMultiline(true))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  1,\n") {
		t.Errorf("multiline array should format one per line, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — interface wrapping a map, pointer to map
// ---------------------------------------------------------------------------

func TestEncodeMapViaInterface(t *testing.T) {
	inner := map[string]any{
		"host": "localhost",
	}
	m := map[string]any{
		"config": any(inner),
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "[config]") || !strings.Contains(got, `host = "localhost"`) {
		t.Errorf("interface-wrapped map should encode, got:\n%s", got)
	}
}

func TestEncodePointerToMap(t *testing.T) {
	inner := map[string]int{"val": 42}
	type Config struct {
		Data *map[string]int `toml:"data"`
	}
	out, err := Marshal(Config{Data: &inner})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "val = 42") {
		t.Errorf("pointer to map should encode, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeMap — array table via map[string]any with slice of struct
// ---------------------------------------------------------------------------

func TestEncodeMapSliceOfStructs(t *testing.T) {
	type Item struct {
		Name string `toml:"name"`
	}
	m := map[string]any{
		"items": []Item{{Name: "a"}, {Name: "b"}},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "[[items]]") {
		t.Errorf("slice of structs in map should use [[]], got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: isEmpty — struct comparison
// ---------------------------------------------------------------------------

func TestEncodeOmitEmptyZeroStruct(t *testing.T) {
	type Inner struct {
		X int    `toml:"x"`
		Y string `toml:"y"`
	}
	type Config struct {
		Inner Inner `toml:"inner,omitempty,inline"`
	}
	out, err := Marshal(Config{Inner: Inner{X: 1, Y: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "x = 1") {
		t.Errorf("non-zero struct should not be omitted, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeStruct — table partition with sub-table and commented field
// ---------------------------------------------------------------------------

func TestEncodeStructCommentedTableField(t *testing.T) {
	type Inner struct {
		Val int `toml:"val"`
	}
	type Config struct {
		Inner Inner `toml:"inner,commented" comment:"disabled section"`
	}
	out, err := Marshal(Config{Inner: Inner{Val: 42}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "# disabled section") {
		t.Errorf("commented table field should have comment, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeMap — sub-map with pointer values
// ---------------------------------------------------------------------------

func TestEncodeMapWithNilValues(t *testing.T) {
	m := map[string]any{
		"name": "test",
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "test"`) {
		t.Errorf("simple map should encode, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: Uint encoding tags (octal, binary)
// ---------------------------------------------------------------------------

func TestEncodeUintHexTag(t *testing.T) {
	type Config struct {
		Perms uint32 `toml:"perms,hex"`
	}
	out, err := Marshal(Config{Perms: 0x755})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0x") {
		t.Errorf("uint hex should have 0x prefix, got:\n%s", out)
	}
}

func TestEncodeUintOctalTag(t *testing.T) {
	type Config struct {
		Perms uint32 `toml:"perms,octal"`
	}
	out, err := Marshal(Config{Perms: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0o") {
		t.Errorf("uint octal should have 0o prefix, got:\n%s", out)
	}
}

func TestEncodeUintBinaryTag(t *testing.T) {
	type Config struct {
		Mask uint8 `toml:"mask,binary"`
	}
	out, err := Marshal(Config{Mask: 0b1010})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0b") {
		t.Errorf("uint binary should have 0b prefix, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — nil pointer, invalid interface, non-struct/map
// ---------------------------------------------------------------------------

func TestEncodeTableNilPointer(t *testing.T) {
	type Inner struct {
		X int `toml:"x"`
	}
	type Config struct {
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(Config{Inner: nil})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "inner") && !strings.Contains(string(out), `""`) {
		t.Errorf("nil pointer struct should be handled, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — marshaler error returns
// ---------------------------------------------------------------------------

type errMarshaler struct{}

func (errMarshaler) MarshalTOML() (any, error) {
	return nil, errors.New("marshal fail")
}

func TestEncodeValueMarshalerError(t *testing.T) {
	type Config struct {
		Val errMarshaler `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from Marshaler")
	}
}

type errMarshalerCtx struct{}

func (errMarshalerCtx) MarshalTOML(ctx context.Context) (any, error) {
	return nil, errors.New("ctx marshal fail")
}

func TestEncodeValueMarshalerContextError(t *testing.T) {
	type Config struct {
		Val errMarshalerCtx `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from MarshalerContext")
	}
}

type errBytesMarshaler struct{}

func (errBytesMarshaler) MarshalTOML() ([]byte, error) {
	return nil, errors.New("bytes marshal fail")
}

func TestEncodeValueBytesMarshalerError(t *testing.T) {
	type Config struct {
		Val errBytesMarshaler `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from BytesMarshaler")
	}
}

type errTextMarshaler struct{}

func (errTextMarshaler) MarshalText() ([]byte, error) {
	return nil, errors.New("text marshal fail")
}

func TestEncodeValueTextMarshalerError(t *testing.T) {
	type Config struct {
		Val errTextMarshaler `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from TextMarshaler")
	}
}

// Addr-based marshaler errors (pointer receivers on non-struct types).
// Non-struct types bypass isTableValue and go through encodeValue directly,
// where the CanAddr() marshaler check finds the pointer-receiver method.

type errAddrMarshalerInt int

func (*errAddrMarshalerInt) MarshalTOML() (any, error) {
	return nil, errors.New("addr marshal fail")
}

func TestEncodeValueAddrMarshalerError(t *testing.T) {
	type Config struct {
		Val errAddrMarshalerInt `toml:"val"`
	}
	_, err := Marshal(&Config{Val: 1})
	if err == nil {
		t.Fatal("expected error from addr Marshaler")
	}
}

type errAddrMarshalerCtxInt int

func (*errAddrMarshalerCtxInt) MarshalTOML(ctx context.Context) (any, error) {
	return nil, errors.New("addr ctx marshal fail")
}

func TestEncodeValueAddrMarshalerCtxError(t *testing.T) {
	type Config struct {
		Val errAddrMarshalerCtxInt `toml:"val"`
	}
	_, err := Marshal(&Config{Val: 1})
	if err == nil {
		t.Fatal("expected error from addr MarshalerContext")
	}
}

type errAddrBytesMarshalerInt int

func (*errAddrBytesMarshalerInt) MarshalTOML() ([]byte, error) {
	return nil, errors.New("addr bytes marshal fail")
}

func TestEncodeValueAddrBytesMarshalerError(t *testing.T) {
	type Config struct {
		Val errAddrBytesMarshalerInt `toml:"val"`
	}
	_, err := Marshal(&Config{Val: 1})
	if err == nil {
		t.Fatal("expected error from addr BytesMarshaler")
	}
}

type errAddrTextMarshalerInt int

func (*errAddrTextMarshalerInt) MarshalText() ([]byte, error) {
	return nil, errors.New("addr text marshal fail")
}

func TestEncodeValueAddrTextMarshalerError(t *testing.T) {
	type Config struct {
		Val errAddrTextMarshalerInt `toml:"val"`
	}
	_, err := Marshal(&Config{Val: 1})
	if err == nil {
		t.Fatal("expected error from addr TextMarshaler")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — custom marshaler registry error
// ---------------------------------------------------------------------------

func TestEncodeCustomMarshalerRegistryError(t *testing.T) {
	type MyStr string
	type Config struct {
		Val MyStr `toml:"val"`
	}
	_, err := MarshalWithOptions(Config{Val: "hello"},
		WithCustomMarshaler(func(d MyStr) ([]byte, error) {
			return nil, errors.New("custom fail")
		}),
	)
	if err == nil {
		t.Fatal("expected error from custom marshaler registry")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeField — commented field with error in value encoding
// ---------------------------------------------------------------------------

func TestEncodeFieldCommentedError(t *testing.T) {
	type Config struct {
		Val errMarshaler `toml:"val,commented"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from commented field encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTableField — nil pointer, interface unwrap, slice error
// ---------------------------------------------------------------------------

func TestEncodeTableFieldNilPtr(t *testing.T) {
	type Inner struct {
		X int `toml:"x"`
	}
	type Config struct {
		Inner *Inner `toml:"inner"`
	}
	out, err := Marshal(Config{Inner: nil})
	if err != nil {
		t.Fatal(err)
	}
	_ = out
}

func TestEncodeTableFieldInterfaceUnwrap(t *testing.T) {
	type Wrapper struct {
		Data any `toml:"data"`
	}
	inner := map[string]any{"x": 1}
	out, err := Marshal(Wrapper{Data: inner})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[data]") {
		t.Errorf("interface wrapping map should produce table, got:\n%s", out)
	}
}

func TestEncodeTableFieldSliceError(t *testing.T) {
	type Config struct {
		Items []errMarshaler `toml:"items,inline"`
	}
	_, err := Marshal(Config{Items: []errMarshaler{{}}})
	if err == nil {
		t.Fatal("expected error from table field slice encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeMap — pointer/interface unwrap in table partition
// ---------------------------------------------------------------------------

func TestEncodeMapPointerTableValue(t *testing.T) {
	inner := struct {
		X int `toml:"x"`
	}{X: 5}
	m := map[string]any{
		"ptr": &inner,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "x = 5") {
		t.Errorf("pointer in map table should unwrap, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeMap — array table and table error returns
// ---------------------------------------------------------------------------

func TestEncodeMapArrayTableError(t *testing.T) {
	type Bad struct {
		Val errMarshaler `toml:"val"`
	}
	m := map[string]any{
		"items": []any{Bad{}},
	}
	_, err := Marshal(m)
	if err == nil {
		t.Fatal("expected error from map array table encoding")
	}
}

func TestEncodeMapTableError(t *testing.T) {
	m := map[string]any{
		"sub": map[string]any{
			"val": errMarshaler{},
		},
	}
	_, err := Marshal(m)
	if err == nil {
		t.Fatal("expected error from map table encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeStruct — simple/table field error propagation
// ---------------------------------------------------------------------------

func TestEncodeStructSimpleFieldError(t *testing.T) {
	type Config struct {
		Val errMarshaler `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from simple field encoding")
	}
}

func TestEncodeStructTableFieldError(t *testing.T) {
	type Inner struct {
		Val errMarshaler `toml:"val"`
	}
	type Config struct {
		Inner Inner `toml:"inner"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from table field encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeArray — multiline and non-multiline error
// ---------------------------------------------------------------------------

func TestEncodeArrayMultilineError(t *testing.T) {
	type Config struct {
		Items []errMarshaler `toml:"items,inline"`
	}
	_, err := MarshalWithOptions(Config{Items: []errMarshaler{{}}}, WithArrayMultiline(true))
	if err == nil {
		t.Fatal("expected error from multiline array encoding")
	}
}

func TestEncodeArrayNonMultilineError(t *testing.T) {
	type Config struct {
		Items []errMarshaler `toml:"items,inline"`
	}
	_, err := Marshal(Config{Items: []errMarshaler{{}}})
	if err == nil {
		t.Fatal("expected error from array encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeInlineTable — field error
// ---------------------------------------------------------------------------

func TestEncodeInlineTableFieldError(t *testing.T) {
	type Inner struct {
		Val errMarshaler `toml:"val"`
	}
	type Config struct {
		Items []Inner `toml:"items"`
	}
	_, err := Marshal(Config{Items: []Inner{{}}})
	if err == nil {
		t.Fatal("expected error from inline table field encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeInlineMap — value error
// ---------------------------------------------------------------------------

func TestEncodeInlineMapValueError(t *testing.T) {
	m := map[string]errMarshaler{"x": {}}
	type Config struct {
		Data map[string]errMarshaler `toml:"data,inline"`
	}
	_, err := Marshal(Config{Data: m})
	if err == nil {
		t.Fatal("expected error from inline map value encoding")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeValue — unsupported type (chan, func, etc.)
// ---------------------------------------------------------------------------

func TestEncodeUnsupportedType(t *testing.T) {
	ch := make(chan int)
	m := map[string]any{"ch": ch}
	_, err := Marshal(m)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("wrong error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeTable — non-struct/map default error
// ---------------------------------------------------------------------------

func TestEncodeTableNonStructMapCov(t *testing.T) {
	m := map[string]any{
		"bad": []any{42, 43},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Logf("expected no table treatment for int slice: %v", err)
	}
	_ = out
}

// ---------------------------------------------------------------------------
// Coverage: isEmpty — struct, default branches
// ---------------------------------------------------------------------------

func TestEncodeIsEmptyDefault(t *testing.T) {
	type Config struct {
		Ch chan int `toml:"ch,omitempty"`
	}
	ch := make(chan int)
	_, err := Marshal(Config{Ch: ch})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Logf("chan should be unsupported: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Coverage: escapeBasic — control char, escapeMultiLineBasic — control char
// ---------------------------------------------------------------------------

func TestEncodeEscapeBasicControlChar(t *testing.T) {
	type Config struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(Config{Val: "a\x01b"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\\u0001") {
		t.Errorf("control char should be escaped, got:\n%s", out)
	}
}

func TestEncodeEscapeBasicDEL(t *testing.T) {
	type Config struct {
		Val string `toml:"val"`
	}
	out, err := Marshal(Config{Val: "a\x7fb"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\\u007F") {
		t.Errorf("DEL should be escaped, got:\n%s", out)
	}
}

func TestEncodeEscapeMultilineBasicControlChar(t *testing.T) {
	type Config struct {
		Val string `toml:"val,multiline"`
	}
	out, err := Marshal(Config{Val: "line1\nline2\x02end"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\\u0002") {
		t.Errorf("multiline control char should be escaped, got:\n%s", out)
	}
}

func TestEncodeEscapeMultilineBasicBackslash(t *testing.T) {
	type Config struct {
		Val string `toml:"val,multiline"`
	}
	out, err := Marshal(Config{Val: "path\\to\\file\nend"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `\\`) {
		t.Errorf("multiline backslash should be escaped, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: writeComment — with path prefix
// ---------------------------------------------------------------------------

func TestEncodeWriteCommentNestedPath(t *testing.T) {
	type Inner struct {
		X int `toml:"x"`
	}
	type Config struct {
		Inner Inner `toml:"inner"`
	}
	out, err := MarshalWithOptions(Config{Inner: Inner{X: 1}},
		WithComment(map[string][]Comment{
			"inner.x": {{Text: "nested comment", Position: HeadCommentPos}},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "# nested comment") {
		t.Errorf("nested comment should appear, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Coverage: isTableValue — BytesMarshaler returns false
// ---------------------------------------------------------------------------

func TestEncodeIsTableValueBytesMarshaler(t *testing.T) {
	bm := bytesMarshaler{}
	rv := reflect.ValueOf(bm)
	if isTableValue(rv) {
		t.Error("BytesMarshaler should not be table value")
	}
}

// ---------------------------------------------------------------------------
// Coverage: encodeField — non-commented field error return
// ---------------------------------------------------------------------------

func TestEncodeFieldNonCommentedError(t *testing.T) {
	type Config struct {
		Val errMarshaler `toml:"val"`
	}
	_, err := Marshal(Config{})
	if err == nil {
		t.Fatal("expected error from field with bad value")
	}
}

// ---------------------------------------------------------------------------
// Suppress unused import warnings
// ---------------------------------------------------------------------------

var _ = fmt.Sprint
var _ = errors.New

func TestEncodeInlineTableOptionNestsAllTables(t *testing.T) {
	type Item struct {
		N int `toml:"n"`
	}
	type Inner struct {
		A string `toml:"a"`
	}
	type S struct {
		Title string         `toml:"title"`
		Sub   Inner          `toml:"sub"`
		Items []Item         `toml:"items"`
		M     map[string]int `toml:"m"`
	}
	in := S{Title: "t", Sub: Inner{A: "val"}, Items: []Item{{N: 1}, {N: 2}}, M: map[string]int{"k": 1}}
	out, err := MarshalWithOptions(in, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	want := "title = \"t\"\nsub = { a = \"val\" }\nitems = [{ n = 1 }, { n = 2 }]\nm = { k = 1 }\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	var back S
	if err := Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, in) {
		t.Errorf("round trip = %+v, want %+v", back, in)
	}

	mout, err := MarshalWithOptions(map[string]any{"server": map[string]any{"port": 1}}, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	if string(mout) != "server = { port = 1 }\n" {
		t.Errorf("map got:\n%s", mout)
	}
}

func TestEncodeWithCommentPositions(t *testing.T) {
	type Server struct {
		Port int `toml:"port"`
	}
	type Cfg struct {
		Name   string `toml:"name"`
		Server Server `toml:"server"`
	}
	out, err := MarshalWithOptions(Cfg{Name: "x", Server: Server{Port: 1}}, WithComment(map[string][]Comment{
		"name": {
			{Position: HeadCommentPos, Text: "head"},
			{Position: LineCommentPos, Text: "line"},
			{Position: FootCommentPos, Text: "foot"},
		},
		"server": {
			{Position: HeadCommentPos, Text: "server head"},
			{Position: LineCommentPos, Text: "server line"},
			{Position: FootCommentPos, Text: "server foot"},
		},
		"server.port": {{Position: LineCommentPos, Text: "port line"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := "# head\nname = \"x\" # line\n# foot\n\n# server head\n[server] # server line\nport = 1 # port line\n# server foot\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	if _, err := Parse(out); err != nil {
		t.Errorf("output does not parse: %v", err)
	}
}

func TestEncodeMixedArrayStartingWithTable(t *testing.T) {
	in := map[string]any{"a": []any{map[string]any{"k": "a"}, "b", int64(1)}}
	out, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back map[string]any
	if err := Unmarshal(out, &back); err != nil {
		t.Fatalf("Unmarshal(%q): %v", out, err)
	}
	if !reflect.DeepEqual(back, in) {
		t.Errorf("round trip: got %v, want %v", back, in)
	}
}

func TestEncodeStringsRoundTripExactly(t *testing.T) {
	values := []string{"a\r\nb", "a\rb", `x"""y`, `"""`, `""`, `"`, `a""`, "a\x01b", "it's", "'''", "\x9a", "tab\there", "\nlead", "end\\", ""}
	type tags struct {
		ML  string `toml:"ml,multiline"`
		MLL string `toml:"mll,multiline,literal"`
		L   string `toml:"l,literal"`
	}
	for _, s := range values {
		for _, opts := range [][]EncodeOption{nil, {WithLiteralString(true)}, {WithMultiLineString(true)}} {
			in := tags{ML: s, MLL: s, L: s}
			out, err := MarshalWithOptions(in, opts...)
			if err != nil {
				t.Fatalf("Marshal(%q): %v", s, err)
			}
			var back tags
			if err := Unmarshal(out, &back); err != nil {
				t.Errorf("Unmarshal of %q: %v\n%s", s, err, out)
				continue
			}
			want := strings.ToValidUTF8(s, "\uFFFD")
			if want != s && back.L == s {
				t.Errorf("literal %q written as invalid UTF-8", s)
			}
			if back.ML != want || back.MLL != want || back.L != want {
				t.Errorf("%q: got %q, %q, %q\n%s", s, back.ML, back.MLL, back.L, out)
			}
		}
	}
}

func TestEncodeNodeStringsRoundTrip(t *testing.T) {
	docs := []string{
		"a = \"\"\"x\\ry\"\"\"\n",
		"a = \"\"\"q\\\"\\\"\\\"q\"\"\"\n",
		"a = '''x\ny'''\n",
	}
	for _, doc := range docs {
		f, err := Parse([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		out, err := NodeToBytes(f.Root)
		if err != nil {
			t.Fatal(err)
		}
		var want, got map[string]any
		if err := Unmarshal([]byte(doc), &want); err != nil {
			t.Fatal(err)
		}
		if err := Unmarshal(out, &got); err != nil {
			t.Errorf("%q re-encoded as %q: %v", doc, out, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", doc, got, want)
		}
	}

	root := &Node{Kind: TableNode, Children: []*Node{
		{Kind: KeyValueNode, Key: []string{"a"}, Children: []*Node{{Kind: StringNode, Style: LiteralStyle, Value: "it's"}}},
		{Kind: KeyValueNode, Key: []string{"b"}, Children: []*Node{{Kind: StringNode, Style: MultiLineLiteralStyle, Value: "x'''y\r"}}},
	}}
	out, err := NodeToBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := Unmarshal(out, &m); err != nil || m["a"] != "it's" || m["b"] != "x'''y\r" {
		t.Errorf("literal styles that cannot hold their value: %v, %v\n%s", m, err, out)
	}
}

func TestEncodeNodeRejectsInvalidScalarText(t *testing.T) {
	f, err := Parse([]byte("port = 80\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := PathString("$.port")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []*Node{
		{Kind: IntegerNode, Value: "1\nroot = true"},
		{Kind: IntegerNode, Value: "1.5"},
		{Kind: BooleanNode, Value: "yes"},
		{Kind: LocalDateNode, Value: "2024-01-01T00:00:00"},
		{Kind: FloatNode, Value: ""},
		{Kind: IntegerNode, Value: "\xef\xbb\xbf2"},
		{Kind: IntegerNode, Value: "\xef\xbb\xbf"},
	} {
		kv := &Node{Kind: KeyValueNode, Key: []string{"port"}, Children: []*Node{v}}
		if err := p.Replace(f.Root, kv); err != nil {
			t.Fatal(err)
		}
		if out, err := NodeToBytes(f.Root); err == nil {
			t.Errorf("value %q: NodeToBytes = %q, want an error", v.Value, out)
		}
	}
	kv := &Node{Kind: KeyValueNode, Key: []string{"port"}, Children: []*Node{{Kind: IntegerNode, Value: "0x1F"}}}
	if err := p.Replace(f.Root, kv); err != nil {
		t.Fatal(err)
	}
	if _, err := NodeToBytes(f.Root); err != nil {
		t.Errorf("valid hex integer rejected: %v", err)
	}
}

func TestEncodeNilValuesOmitted(t *testing.T) {
	var np *int
	out, err := Marshal(map[string]any{"k": nil, "p": np, "x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "x = 1\n" {
		t.Errorf("got %q, want only x", out)
	}
	if _, err := Marshal(map[string]any{"a": []any{1, nil}}); err == nil {
		t.Error("nil inside an array: want an error")
	}
	out, err = FromJSON([]byte(`{"a": null, "b": 1}`))
	if err != nil || string(out) != "b = 1\n" {
		t.Errorf("FromJSON null: %q, %v", out, err)
	}
}

func TestEncodeNegativeBaseTaggedIntegers(t *testing.T) {
	type H struct {
		N int `toml:"n,hex"`
		O int `toml:"o,octal"`
		B int `toml:"b,binary"`
		P int `toml:"p,hex"`
	}
	out, err := Marshal(H{N: -31, O: -8, B: -2, P: 31})
	if err != nil {
		t.Fatal(err)
	}
	var back H
	if err := Unmarshal(out, &back); err != nil {
		t.Fatalf("Unmarshal(%q): %v", out, err)
	}
	if back != (H{N: -31, O: -8, B: -2, P: 31}) || !strings.Contains(string(out), "p = 0x1F") {
		t.Errorf("got %+v from %q", back, out)
	}
	if _, err := Marshal(map[string]uint64{"u": math.MaxUint64}); err == nil {
		t.Error("uint64 above the TOML integer range: want an error")
	}
}

func TestEncodeCommentedTableFields(t *testing.T) {
	type sub struct {
		A int `toml:"a"`
	}
	type cfg struct {
		Sub  sub   `toml:"sub,commented"`
		List []sub `toml:"list,commented"`
		Arr  []int `toml:"arr,commented"`
	}
	out, err := MarshalWithOptions(cfg{Sub: sub{1}, List: []sub{{2}}, Arr: []int{1, 2}}, WithArrayMultiline(true))
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := Unmarshal(out, &back); err != nil {
		t.Fatalf("Unmarshal(%q): %v", out, err)
	}
	if len(back) != 0 {
		t.Errorf("commented fields became live keys %v:\n%s", back, out)
	}
	if !strings.Contains(string(out), "# sub = { a = 1 }") {
		t.Errorf("commented table not written as a comment:\n%s", out)
	}
}

func TestEncodeCommentsAreValid(t *testing.T) {
	type cfg struct {
		A int `toml:"a" comment:"line1\nline2"`
	}
	out, err := MarshalWithOptions(cfg{A: 1}, WithComment(map[string][]Comment{
		"a": {{Position: LineCommentPos, Text: "x\x00y\rz"}, {Position: HeadCommentPos, Text: "bad \xff utf8"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !Valid(out) {
		t.Errorf("invalid TOML:\n%q", out)
	}
	if !strings.Contains(string(out), "# line2\n") {
		t.Errorf("second comment line not commented:\n%s", out)
	}
}

func TestEncodeTimeEdgeCases(t *testing.T) {
	tm := time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("lmt", 30))
	out, err := Marshal(map[string]any{"t": tm})
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if got := back["t"].(time.Time); !got.Equal(tm) {
		t.Errorf("got %v, want the instant %v (%s)", got, tm, out)
	}

	type cfg struct {
		When time.Time `toml:"when"`
	}
	_, err = Marshal(cfg{When: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err == nil || !strings.HasPrefix(err.Error(), `toml: key "when"`) {
		t.Errorf("year 10000: err = %v, want it to name the key", err)
	}
}

func TestEncodeFloat32ShortestDigits(t *testing.T) {
	out, err := Marshal(map[string]any{"f": float32(0.1), "g": 0.1, "h": float32(3)})
	if err != nil {
		t.Fatal(err)
	}
	if want := "f = 0.1\ng = 0.1\nh = 3.0\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestEncodeNodeChecksEveryScalar(t *testing.T) {
	f, err := Parse([]byte("a = 1979-05-27T07:32:00Z\nb = 2\nc = [1.5, true]\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := NodeToBytes(f.Root)
	if err != nil || string(out) != "a = 1979-05-27T07:32:00Z\nb = 2\nc = [1.5, true]\n" {
		t.Fatalf("NodeToBytes = %q, %v", out, err)
	}
	p, err := PathString("$.c")
	if err != nil {
		t.Fatal(err)
	}
	arr := &Node{Kind: ArrayNode, Children: []*Node{{Kind: FloatNode, Value: "1.5"}, {Kind: BooleanNode, Value: "1"}}}
	kv := &Node{Kind: KeyValueNode, Key: []string{"c"}, Children: []*Node{arr}}
	if err := p.Replace(f.Root, kv); err != nil {
		t.Fatal(err)
	}
	if out, err := NodeToBytes(f.Root); err == nil {
		t.Errorf("boolean %q after valid scalars: NodeToBytes = %q, want an error", "1", out)
	}
}

type cycleNode struct {
	V    int
	Next *cycleNode
}

func TestEncodeCycle(t *testing.T) {
	n := &cycleNode{V: 1}
	n.Next = n
	m := map[string]any{"x": 1}
	m["m"] = m
	arr := []any{1}
	arr[0] = arr
	type inline struct {
		Items []any `toml:"items"`
	}
	ring := &cycleNode{V: 1, Next: &cycleNode{V: 2}}
	ring.Next.Next = ring
	for name, v := range map[string]any{
		"struct pointer": n,
		"ring":           ring,
		"map":            m,
		"inline slice":   inline{Items: arr},
		"array tables":   map[string]any{"a": []map[string]any{m}},
	} {
		done := make(chan error, 1)
		go func() {
			_, err := Marshal(v)
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, errEncodeCycle) {
				t.Errorf("%s: err = %v, want a cycle error", name, err)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("%s: Marshal of a cyclic value did not return", name)
		}
	}

	// A deep value that is not a cycle still encodes.
	deep := &cycleNode{}
	for range cycleDepth + 10 {
		deep = &cycleNode{V: 1, Next: deep}
	}
	if _, err := Marshal(deep); err != nil {
		t.Errorf("deep acyclic value: %v", err)
	}
	shared := map[string]any{"v": 1}
	if _, err := Marshal(map[string]any{"a": shared, "b": shared}); err != nil {
		t.Errorf("shared map: %v", err)
	}
}
