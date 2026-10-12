package toml

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Marshal() basic usage
// ---------------------------------------------------------------------------

func TestMarshalBasicStruct(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	out, err := Marshal(Config{Name: "app", Port: 3000})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `name = "app"`) || !strings.Contains(s, "port = 3000") {
		t.Errorf("unexpected output:\n%s", s)
	}
}

func TestMarshalMap(t *testing.T) {
	m := map[string]any{
		"key": "value",
		"num": 100,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `key = "value"`) || !strings.Contains(s, "num = 100") {
		t.Errorf("unexpected output:\n%s", s)
	}
}

func TestMarshalPointerToStruct(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
	}
	cfg := &Config{Name: "ptr"}
	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "ptr"`) {
		t.Errorf("unexpected output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// MarshalWithOptions() with various options
// ---------------------------------------------------------------------------

func TestMarshalWithOptionsOmitEmpty(t *testing.T) {
	type Config struct {
		Name  string `toml:"name"`
		Debug bool   `toml:"debug"`
	}
	out, err := MarshalWithOptions(Config{Name: "app"}, WithOmitEmpty(true))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "debug") {
		t.Errorf("expected debug omitted, got:\n%s", s)
	}
}

func TestMarshalWithOptionsInlineTable(t *testing.T) {
	// WithInlineTable writes nested struct fields as inline tables.
	type Inner struct {
		A int `toml:"a"`
	}
	type Config struct {
		Inner Inner `toml:"inner"`
	}
	out, err := MarshalWithOptions(Config{Inner: Inner{A: 1}}, WithInlineTable(true))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(out); s != "inner = { a = 1 }\n" {
		t.Errorf("got:\n%s", s)
	}
}

func TestMarshalWithOptionsArrayMultiline(t *testing.T) {
	type Config struct {
		Items []string `toml:"items"`
	}
	out, err := MarshalWithOptions(
		Config{Items: []string{"a", "b"}},
		WithArrayMultiline(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[\n") {
		t.Errorf("expected multiline array, got:\n%s", s)
	}
}

func TestMarshalWithOptionsLiteralString(t *testing.T) {
	type Config struct {
		Path string `toml:"path"`
	}
	out, err := MarshalWithOptions(
		Config{Path: `C:\Users\test`},
		WithLiteralString(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `'C:\Users\test'`) {
		t.Errorf("expected literal string, got:\n%s", s)
	}
}

func TestMarshalWithOptionsMultiLineString(t *testing.T) {
	type Config struct {
		Text string `toml:"text"`
	}
	out, err := MarshalWithOptions(
		Config{Text: "hello\nworld"},
		WithMultiLineString(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"""`) {
		t.Errorf("expected multi-line string, got:\n%s", s)
	}
}

func TestMarshalWithOptionsComment(t *testing.T) {
	type Config struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	out, err := MarshalWithOptions(
		Config{Host: "localhost", Port: 8080},
		WithComment(map[string][]Comment{
			"host": {{Position: HeadCommentPos, Text: "Server hostname"}},
			"port": {{Position: HeadCommentPos, Text: "Server port"}},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Server hostname") {
		t.Errorf("missing host comment in:\n%s", s)
	}
	if !strings.Contains(s, "# Server port") {
		t.Errorf("missing port comment in:\n%s", s)
	}
}

func TestMarshalWithOptionsTableSeparator(t *testing.T) {
	type Sub struct {
		A int `toml:"a"`
	}
	type Config struct {
		Name string `toml:"name"`
		Sub  Sub    `toml:"sub"`
	}
	// With separator (default)
	out1, _ := MarshalWithOptions(Config{Name: "test", Sub: Sub{A: 1}}, WithTableSeparator(true))
	if !strings.Contains(string(out1), "\n\n") {
		t.Errorf("expected blank line with separator enabled:\n%q", out1)
	}
	// Without separator
	out2, _ := MarshalWithOptions(Config{Name: "test", Sub: Sub{A: 1}}, WithTableSeparator(false))
	if strings.Contains(string(out2), "\n\n") {
		t.Errorf("expected no blank line with separator disabled:\n%q", out2)
	}
}

func TestMarshalWithMultipleOptions(t *testing.T) {
	type Config struct {
		Items []string `toml:"items"`
		Name  string   `toml:"name"`
		Debug bool     `toml:"debug"`
	}
	out, err := MarshalWithOptions(
		Config{Items: []string{"a", "b"}, Name: "test"},
		WithArrayMultiline(true),
		WithOmitEmpty(true),
		WithTableSeparator(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// debug should be omitted
	if strings.Contains(s, "debug") {
		t.Errorf("debug should be omitted:\n%s", s)
	}
	// array should be multiline
	if !strings.Contains(s, "[\n") {
		t.Errorf("expected multiline array:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// Encoder type: Encode(), EncodeContext(), Close(), SetContext()
// ---------------------------------------------------------------------------

func TestMarshalEncoderEncode(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(Config{Name: "test"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `name = "test"`) {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

func TestMarshalEncoderEncodeContext(t *testing.T) {
	type Config struct {
		Port int `toml:"port"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	ctx := context.WithValue(context.Background(), "key", "value")
	if err := enc.EncodeContext(ctx, Config{Port: 9090}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "port = 9090") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

func TestMarshalEncoderSetContext(t *testing.T) {
	type Config struct {
		Port int `toml:"port"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	ctx := context.WithValue(context.Background(), "mykey", "myval")
	enc.SetContext(ctx)
	if err := enc.Encode(Config{Port: 1234}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "port = 1234") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

func TestMarshalEncoderClose(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	// Close on a bytes.Buffer (no Flush method) should be a no-op
	if err := enc.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}

type flushableWriter struct {
	bytes.Buffer
	flushed bool
}

func (w *flushableWriter) Flush() error {
	w.flushed = true
	return nil
}

func TestMarshalEncoderCloseWithFlush(t *testing.T) {
	w := &flushableWriter{}
	enc := NewEncoder(w)
	if err := enc.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !w.flushed {
		t.Error("expected Flush() to be called on Close()")
	}
}

func TestMarshalEncoderWithOptions(t *testing.T) {
	type Config struct {
		Items []int `toml:"items"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf, WithArrayMultiline(true))
	if err := enc.Encode(Config{Items: []int{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[\n") {
		t.Errorf("expected multiline array:\n%s", buf.String())
	}
}

func TestMarshalEncoderMultipleEncodes(t *testing.T) {
	// Each Encode call writes completely, so multiple calls produce
	// concatenated output
	type Config struct {
		Name string `toml:"name"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	_ = enc.Encode(Config{Name: "first"})
	_ = enc.Encode(Config{Name: "second"})
	s := buf.String()
	if strings.Count(s, "name") != 2 {
		t.Errorf("expected two name entries, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// RawValue type
// ---------------------------------------------------------------------------

func TestMarshalRawValueUnmarshal(t *testing.T) {
	raw := RawValue([]byte(`"hello world"`))
	var s struct {
		Raw string `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&s); err != nil {
		t.Fatal(err)
	}
	if s.Raw != "hello world" {
		t.Errorf("RawValue.Unmarshal = %q, want %q", s.Raw, "hello world")
	}
}

func TestMarshalRawValueUnmarshalInt(t *testing.T) {
	raw := RawValue([]byte("42"))
	var s struct {
		Raw int `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&s); err != nil {
		t.Fatal(err)
	}
	if s.Raw != 42 {
		t.Errorf("RawValue.Unmarshal = %d, want 42", s.Raw)
	}
}

func TestMarshalRawValueUnmarshalArray(t *testing.T) {
	raw := RawValue([]byte("[1, 2, 3]"))
	var s struct {
		Raw []int `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&s); err != nil {
		t.Fatal(err)
	}
	if len(s.Raw) != 3 || s.Raw[0] != 1 || s.Raw[2] != 3 {
		t.Errorf("RawValue.Unmarshal = %v, want [1,2,3]", s.Raw)
	}
}

func TestMarshalRawValueUnmarshalBool(t *testing.T) {
	raw := RawValue([]byte("true"))
	var s struct {
		Raw bool `toml:"_raw_"`
	}
	if err := raw.Unmarshal(&s); err != nil {
		t.Fatal(err)
	}
	if !s.Raw {
		t.Error("RawValue.Unmarshal = false, want true")
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestMarshalNonStructMap(t *testing.T) {
	_, err := Marshal("hello")
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop for string, got: %v", err)
	}
}

func TestMarshalSlice(t *testing.T) {
	_, err := Marshal([]int{1, 2, 3})
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop for slice, got: %v", err)
	}
}

func TestMarshalNil(t *testing.T) {
	_, err := Marshal(nil)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop for nil, got: %v", err)
	}
}

func TestMarshalBool(t *testing.T) {
	_, err := Marshal(true)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop for bool, got: %v", err)
	}
}

func TestMarshalNilPointerToStruct(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
	}
	var p *Config
	_, err := Marshal(p)
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop for nil pointer, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Round-trip: Marshal then Unmarshal
// ---------------------------------------------------------------------------

func TestMarshalRoundTripSimple(t *testing.T) {
	type Config struct {
		Name    string  `toml:"name"`
		Port    int     `toml:"port"`
		Debug   bool    `toml:"debug"`
		Rate    float64 `toml:"rate"`
		Version int64   `toml:"version"`
	}
	original := Config{
		Name:    "myapp",
		Port:    8080,
		Debug:   true,
		Rate:    99.5,
		Version: 3,
	}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

func TestMarshalRoundTripNested(t *testing.T) {
	type DB struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Title string `toml:"title"`
		DB    DB     `toml:"db"`
	}
	original := Config{
		Title: "Test",
		DB:    DB{Host: "localhost", Port: 5432},
	}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title mismatch: %q != %q", decoded.Title, original.Title)
	}
	if decoded.DB.Host != original.DB.Host || decoded.DB.Port != original.DB.Port {
		t.Errorf("DB mismatch: %+v != %+v", decoded.DB, original.DB)
	}
}

func TestMarshalRoundTripArrayOfTables(t *testing.T) {
	type Item struct {
		Name  string `toml:"name"`
		Count int    `toml:"count"`
	}
	type Config struct {
		Items []Item `toml:"items"`
	}
	original := Config{
		Items: []Item{
			{Name: "a", Count: 1},
			{Name: "b", Count: 2},
		},
	}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded.Items) != 2 {
		t.Fatalf("Items length = %d, want 2", len(decoded.Items))
	}
	if decoded.Items[0] != original.Items[0] || decoded.Items[1] != original.Items[1] {
		t.Errorf("Items mismatch:\n  original: %+v\n  decoded:  %+v", original.Items, decoded.Items)
	}
}

func TestMarshalRoundTripSlice(t *testing.T) {
	type Config struct {
		Tags []string `toml:"tags"`
		Nums []int    `toml:"nums"`
	}
	original := Config{
		Tags: []string{"go", "toml", "test"},
		Nums: []int{1, 2, 3, 4, 5},
	}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Fatalf("Tags length mismatch: %d != %d", len(decoded.Tags), len(original.Tags))
	}
	for i, tag := range original.Tags {
		if decoded.Tags[i] != tag {
			t.Errorf("Tags[%d] = %q, want %q", i, decoded.Tags[i], tag)
		}
	}
}

func TestMarshalRoundTripMap(t *testing.T) {
	original := map[string]any{
		"name": "test",
		"port": int64(8080),
	}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded map[string]any
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded["name"] != original["name"] {
		t.Errorf("name = %v, want %v", decoded["name"], original["name"])
	}
	if decoded["port"] != original["port"] {
		t.Errorf("port = %v (%T), want %v (%T)", decoded["port"], decoded["port"], original["port"], original["port"])
	}
}

func TestMarshalRoundTripBooleans(t *testing.T) {
	type Config struct {
		A bool `toml:"a"`
		B bool `toml:"b"`
	}
	original := Config{A: true, B: false}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != original {
		t.Errorf("mismatch: %+v != %+v", decoded, original)
	}
}

func TestMarshalRoundTripFloat(t *testing.T) {
	type Config struct {
		Pi  float64 `toml:"pi"`
		Big float64 `toml:"big"`
	}
	original := Config{Pi: 3.14159, Big: 1e10}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if math.Abs(decoded.Pi-original.Pi) > 1e-10 {
		t.Errorf("Pi mismatch: %v != %v", decoded.Pi, original.Pi)
	}
}

func TestMarshalRoundTripDateTime(t *testing.T) {
	type Config struct {
		Created time.Time `toml:"created"`
	}
	original := Config{Created: time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC)}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.Created.Equal(original.Created) {
		t.Errorf("Created mismatch: %v != %v", decoded.Created, original.Created)
	}
}

func TestMarshalRoundTripLocalDate(t *testing.T) {
	type Config struct {
		Birthday LocalDate `toml:"birthday"`
	}
	original := Config{Birthday: LocalDate{Year: 1990, Month: time.January, Day: 15}}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Birthday != original.Birthday {
		t.Errorf("Birthday mismatch: %v != %v", decoded.Birthday, original.Birthday)
	}
}

func TestMarshalRoundTripLocalTime(t *testing.T) {
	type Config struct {
		Alarm LocalTime `toml:"alarm"`
	}
	original := Config{Alarm: LocalTime{Hour: 7, Minute: 30, Second: 0}}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Alarm != original.Alarm {
		t.Errorf("Alarm mismatch: %v != %v", decoded.Alarm, original.Alarm)
	}
}

func TestMarshalRoundTripLocalDateTime(t *testing.T) {
	type Config struct {
		Event LocalDateTime `toml:"event"`
	}
	original := Config{Event: LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.December, Day: 25},
		LocalTime: LocalTime{Hour: 12, Minute: 0, Second: 0},
	}}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Event != original.Event {
		t.Errorf("Event mismatch: %v != %v", decoded.Event, original.Event)
	}
}

func TestMarshalRoundTripDuration(t *testing.T) {
	type Config struct {
		Timeout time.Duration `toml:"timeout"`
	}
	original := Config{Timeout: 30 * time.Second}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Timeout != original.Timeout {
		t.Errorf("Timeout mismatch: %v != %v", decoded.Timeout, original.Timeout)
	}
}

// ---------------------------------------------------------------------------
// wrapAsValue helper
// ---------------------------------------------------------------------------

func TestMarshalWrapAsValue(t *testing.T) {
	raw := wrapAsValue([]byte(`"test"`))
	expected := `_raw_ = "test"`
	if string(raw) != expected {
		t.Errorf("wrapAsValue = %q, want %q", raw, expected)
	}
}

// ---------------------------------------------------------------------------
// Encoder error propagation
// ---------------------------------------------------------------------------

func TestMarshalEncoderError(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.Encode("not a struct")
	if err == nil {
		t.Fatal("expected error for non-struct")
	}
	if err != ErrMarshalTop {
		t.Errorf("expected ErrMarshalTop, got: %v", err)
	}
}

func TestMarshalEncoderContextError(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.EncodeContext(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for int top-level")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestMarshalDeepNesting(t *testing.T) {
	type L3 struct {
		Val string `toml:"val"`
	}
	type L2 struct {
		L3 L3 `toml:"l3"`
	}
	type L1 struct {
		L2 L2 `toml:"l2"`
	}
	type Config struct {
		L1 L1 `toml:"l1"`
	}
	out, err := Marshal(Config{L1: L1{L2: L2{L3: L3{Val: "deep"}}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "[l1.l2.l3]") {
		t.Errorf("expected deeply nested table path, got:\n%s", s)
	}
	if !strings.Contains(s, `val = "deep"`) {
		t.Errorf("expected val = deep, got:\n%s", s)
	}
}

func TestMarshalMixedScalarsAndTables(t *testing.T) {
	type Inner struct {
		B int `toml:"b"`
	}
	type Config struct {
		A     int    `toml:"a"`
		Inner Inner  `toml:"inner"`
		C     string `toml:"c"`
	}
	out, err := Marshal(Config{A: 1, Inner: Inner{B: 2}, C: "three"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Scalars (a, c) should appear before the table header [inner]
	aIdx := strings.Index(s, "a = 1")
	cIdx := strings.Index(s, `c = "three"`)
	tIdx := strings.Index(s, "[inner]")
	if aIdx == -1 || cIdx == -1 || tIdx == -1 {
		t.Fatalf("missing content in:\n%s", s)
	}
	if aIdx > tIdx || cIdx > tIdx {
		t.Errorf("scalars should appear before table header:\n%s", s)
	}
}

func TestMarshalEmptyArrayNotOmitted(t *testing.T) {
	type Config struct {
		Items []int `toml:"items"`
	}
	out, err := Marshal(Config{Items: []int{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "items = []") {
		t.Errorf("expected empty array to be present:\n%s", out)
	}
}

func TestMarshalUintVariants(t *testing.T) {
	type Config struct {
		A uint8  `toml:"a"`
		B uint16 `toml:"b"`
		C uint32 `toml:"c"`
		D uint64 `toml:"d"`
	}
	out, err := Marshal(Config{A: 10, B: 20, C: 30, D: 40})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"a = 10", "b = 20", "c = 30", "d = 40"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestMarshalSpecialCharsInKey(t *testing.T) {
	m := map[string]any{
		"normal":        1,
		"with space":    2,
		"with.dot":      3,
		"with\"quote":   4,
		"with\nnewline": 5,
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Keys with special chars should be quoted
	if !strings.Contains(s, `"with space"`) {
		t.Errorf("expected quoted key for space, got:\n%s", s)
	}
}

func TestMarshalEncoderWithContextPropagation(t *testing.T) {
	// Test that EncodeContext propagates the context to the internal encoder.
	// MarshalerContext on a struct type is not detected by isTableValue,
	// so we test basic context propagation through EncodeContext.
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	ctx := context.WithValue(context.Background(), "test", "ctx-value")
	err := enc.EncodeContext(ctx, Config{Name: "hello", Port: 42})
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, `name = "hello"`) || !strings.Contains(s, "port = 42") {
		t.Errorf("unexpected output:\n%s", s)
	}
}
