package toml

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unmarshal basic usage
// ---------------------------------------------------------------------------

func TestUnmarshalBasicStruct(t *testing.T) {
	input := `
host = "example.com"
port = 443
debug = false
`
	type Config struct {
		Host  string `toml:"host"`
		Port  int    `toml:"port"`
		Debug bool   `toml:"debug"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Host != "example.com" {
		t.Errorf("Host = %q", cfg.Host)
	}
	if cfg.Port != 443 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.Debug {
		t.Errorf("Debug = true, want false")
	}
}

func TestUnmarshalIntoMap(t *testing.T) {
	input := `alpha = "one"
beta = 2
`
	var m map[string]any
	if err := Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["alpha"] != "one" {
		t.Errorf("alpha = %v", m["alpha"])
	}
	if m["beta"] != int64(2) {
		t.Errorf("beta = %v (%T)", m["beta"], m["beta"])
	}
}

func TestUnmarshalEmptyInput(t *testing.T) {
	var m map[string]any
	if err := Unmarshal([]byte(""), &m); err != nil {
		t.Fatalf("Unmarshal empty: %v", err)
	}
}

func TestUnmarshalNilTarget(t *testing.T) {
	err := Unmarshal([]byte(`key = "val"`), nil)
	if !errors.Is(err, ErrNilPointer) {
		t.Fatalf("expected ErrNilPointer, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UnmarshalWithOptions
// ---------------------------------------------------------------------------

func TestUnmarshalWithOptionsStrict(t *testing.T) {
	input := `known = 1
unknown = 2
`
	type Config struct {
		Known int `toml:"known"`
	}
	var cfg Config
	err := UnmarshalWithOptions([]byte(input), &cfg, WithStrict())
	if err == nil {
		t.Fatal("expected strict error")
	}
	var uf *UnknownFieldError
	if !errors.As(err, &uf) {
		t.Fatalf("expected UnknownFieldError, got %T: %v", err, err)
	}
}

func TestUnmarshalWithOptionsDefaults(t *testing.T) {
	input := `name = "svc"
`
	type Config struct {
		Name    string `toml:"name"`
		Timeout string `toml:"timeout,default=30s"`
	}
	var cfg Config
	if err := UnmarshalWithOptions([]byte(input), &cfg, WithDefaults()); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Timeout != "30s" {
		t.Errorf("Timeout = %q, want %q", cfg.Timeout, "30s")
	}
}

func TestUnmarshalWithOptionsMaxDepth(t *testing.T) {
	input := `
[a]
[a.b]
[a.b.c]
key = 1
`
	type D struct {
		Key int `toml:"key"`
	}
	type C struct {
		D D `toml:"c"`
	}
	type B struct {
		C C `toml:"b"`
	}
	type A struct {
		B B `toml:"a"`
	}
	var cfg A
	err := UnmarshalWithOptions([]byte(input), &cfg, WithMaxDepth(2))
	if err == nil {
		t.Fatal("expected depth error")
	}
}

func TestUnmarshalWithOptionsMaxKeys(t *testing.T) {
	input := `
x = 1
y = 2
z = 3
`
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxKeys(2))
	if err == nil {
		t.Fatal("expected max keys error")
	}
}

func TestUnmarshalWithOptionsMaxDocumentSize(t *testing.T) {
	input := `key = "a longer value"
`
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxDocumentSize(5))
	if !errors.Is(err, ErrDocumentSize) {
		t.Fatalf("expected ErrDocumentSize, got %v", err)
	}
}

func TestUnmarshalWithOptionsMaxDocumentSizeAllowed(t *testing.T) {
	input := `k = 1
`
	var m map[string]any
	// Size large enough to pass.
	if err := UnmarshalWithOptions([]byte(input), &m, WithMaxDocumentSize(1000)); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["k"] != int64(1) {
		t.Errorf("k = %v", m["k"])
	}
}

func TestUnmarshalWithOptionsOrderedMap(t *testing.T) {
	input := `z = 3
a = 1
m = 2
`
	var result any
	if err := UnmarshalWithOptions([]byte(input), &result, WithOrderedMap()); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ms, ok := result.(MapSlice)
	if !ok {
		t.Fatalf("expected MapSlice, got %T", result)
	}
	if len(ms) != 3 {
		t.Fatalf("len = %d", len(ms))
	}
	if ms[0].Key != "z" {
		t.Errorf("first key = %v, want z", ms[0].Key)
	}
}

func TestUnmarshalWithOptionsMultiple(t *testing.T) {
	input := `name = "svc"
`
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port,default=9090"`
	}
	var cfg Config
	err := UnmarshalWithOptions([]byte(input), &cfg, WithDefaults(), WithMaxDocumentSize(10000))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

// ---------------------------------------------------------------------------
// UnmarshalTo[T]
// ---------------------------------------------------------------------------

func TestUnmarshalToGenericStruct(t *testing.T) {
	input := `title = "hello"
count = 5
`
	type Doc struct {
		Title string `toml:"title"`
		Count int    `toml:"count"`
	}
	doc, err := UnmarshalTo[Doc]([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTo: %v", err)
	}
	if doc.Title != "hello" {
		t.Errorf("Title = %q", doc.Title)
	}
	if doc.Count != 5 {
		t.Errorf("Count = %d", doc.Count)
	}
}

func TestUnmarshalToGenericMap(t *testing.T) {
	input := `key = "value"
`
	m, err := UnmarshalTo[map[string]any]([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTo: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v", m["key"])
	}
}

func TestUnmarshalToWithOptions(t *testing.T) {
	input := `name = "opts"
`
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port,default=3000"`
	}
	cfg, err := UnmarshalTo[Config]([]byte(input), WithDefaults())
	if err != nil {
		t.Fatalf("UnmarshalTo: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d", cfg.Port)
	}
}

func TestUnmarshalToInvalidInput(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
	}
	_, err := UnmarshalTo[Config]([]byte(`name = `))
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// DecodeFile
// ---------------------------------------------------------------------------

func TestUnmarshalDecodeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := []byte(`host = "localhost"
port = 9090
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	type Config struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	var cfg Config
	if err := DecodeFile(path, &cfg); err != nil {
		t.Fatalf("DecodeFile: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d", cfg.Port)
	}
}

func TestUnmarshalDecodeFileNotFound(t *testing.T) {
	var m map[string]any
	err := DecodeFile("/nonexistent/path/no_such_file.toml", &m)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestUnmarshalDecodeFileWithOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opts.toml")
	content := []byte(`name = "test"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port,default=4000"`
	}
	var cfg Config
	if err := DecodeFile(path, &cfg, WithDefaults()); err != nil {
		t.Fatalf("DecodeFile: %v", err)
	}
	if cfg.Port != 4000 {
		t.Errorf("Port = %d, want 4000", cfg.Port)
	}
}

func TestUnmarshalDecodeFileMaxDocumentSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.toml")
	content := []byte(`key = "some value here"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var m map[string]any
	err := DecodeFile(path, &m, WithMaxDocumentSize(5))
	if !errors.Is(err, ErrDocumentSize) {
		t.Fatalf("expected ErrDocumentSize, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Decoder
// ---------------------------------------------------------------------------

func TestUnmarshalNewDecoder(t *testing.T) {
	input := `key = "decoder"
`
	type Config struct {
		Key string `toml:"key"`
	}
	var cfg Config
	dec := NewDecoder(strings.NewReader(input))
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Key != "decoder" {
		t.Errorf("Key = %q", cfg.Key)
	}
}

func TestUnmarshalDecoderSecondDecodeEOF(t *testing.T) {
	input := `key = "once"
`
	dec := NewDecoder(strings.NewReader(input))
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("first Decode: %v", err)
	}
	// Second call must return io.EOF.
	var m2 map[string]any
	err := dec.Decode(&m2)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF on second Decode, got %v", err)
	}
}

func TestUnmarshalDecoderDecodeContext(t *testing.T) {
	input := `val = 42
`
	type Config struct {
		Val int `toml:"val"`
	}
	dec := NewDecoder(strings.NewReader(input))
	var cfg Config
	ctx := context.Background()
	if err := dec.DecodeContext(ctx, &cfg); err != nil {
		t.Fatalf("DecodeContext: %v", err)
	}
	if cfg.Val != 42 {
		t.Errorf("Val = %d", cfg.Val)
	}
}

func TestUnmarshalDecoderDecodeContextCancelled(t *testing.T) {
	// Even with cancelled context, data is already available on the reader,
	// so this tests that SetContext/DecodeContext accept cancelled contexts.
	// Whether the decode fails depends on implementation details.
	input := `val = 1
`
	dec := NewDecoder(strings.NewReader(input))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var m map[string]any
	// We don't assert on the error because the implementation may or may not
	// check context cancellation. We just ensure it doesn't panic.
	_ = dec.DecodeContext(ctx, &m)
}

func TestUnmarshalDecoderSetContext(t *testing.T) {
	input := `val = 100
`
	type Config struct {
		Val int `toml:"val"`
	}
	dec := NewDecoder(strings.NewReader(input))
	ctx := context.Background()
	dec.SetContext(ctx)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("Decode after SetContext: %v", err)
	}
	if cfg.Val != 100 {
		t.Errorf("Val = %d", cfg.Val)
	}
}

func TestUnmarshalDecoderWithOptions(t *testing.T) {
	input := `name = "test"
extra = "unexpected"
`
	type Config struct {
		Name string `toml:"name"`
	}
	dec := NewDecoder(strings.NewReader(input), WithStrict())
	var cfg Config
	err := dec.Decode(&cfg)
	if err == nil {
		t.Fatal("expected strict error")
	}
	var uf *UnknownFieldError
	if !errors.As(err, &uf) {
		t.Fatalf("expected UnknownFieldError, got %T: %v", err, err)
	}
}

func TestUnmarshalDecoderDecodeContextSecondCallEOF(t *testing.T) {
	input := `x = 1
`
	dec := NewDecoder(strings.NewReader(input))
	var m map[string]any
	if err := dec.DecodeContext(context.Background(), &m); err != nil {
		t.Fatalf("first DecodeContext: %v", err)
	}
	var m2 map[string]any
	err := dec.DecodeContext(context.Background(), &m2)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// WithMaxDocumentSize via Decoder
// ---------------------------------------------------------------------------

func TestUnmarshalDecoderMaxDocumentSize(t *testing.T) {
	input := `key = "this is a fairly long string value"
`
	dec := NewDecoder(strings.NewReader(input), WithMaxDocumentSize(5))
	var m map[string]any
	err := dec.Decode(&m)
	if !errors.Is(err, ErrDocumentSize) {
		t.Fatalf("expected ErrDocumentSize, got %v", err)
	}
}

func TestUnmarshalDecoderMaxDocumentSizeAllowed(t *testing.T) {
	input := `k = 1
`
	dec := NewDecoder(strings.NewReader(input), WithMaxDocumentSize(10000))
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m["k"] != int64(1) {
		t.Errorf("k = %v", m["k"])
	}
}

// ---------------------------------------------------------------------------
// WithMaxNodes via Unmarshal and Decoder paths
// ---------------------------------------------------------------------------

func TestUnmarshalMaxNodesViaUnmarshal(t *testing.T) {
	input := `
a = 1
b = 2
c = 3
d = 4
`
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxNodes(2))
	if err == nil {
		t.Fatal("expected max nodes error")
	}
}

func TestUnmarshalMaxNodesViaDecoder(t *testing.T) {
	input := `
a = 1
b = 2
c = 3
d = 4
`
	dec := NewDecoder(strings.NewReader(input), WithMaxNodes(2))
	var m map[string]any
	err := dec.Decode(&m)
	if err == nil {
		t.Fatal("expected max nodes error via Decoder")
	}
}

// ---------------------------------------------------------------------------
// Error propagation
// ---------------------------------------------------------------------------

func TestUnmarshalScannerError(t *testing.T) {
	// Unterminated string should cause a scanner error.
	input := `key = "unterminated`
	var m map[string]any
	err := Unmarshal([]byte(input), &m)
	if err == nil {
		t.Fatal("expected scanner error")
	}
}

func TestUnmarshalParserError(t *testing.T) {
	// Duplicate key should cause a parser error.
	input := `key = 1
key = 2
`
	var m map[string]any
	err := Unmarshal([]byte(input), &m)
	if err == nil {
		t.Fatal("expected parser error")
	}
}

func TestUnmarshalDecoderError(t *testing.T) {
	// Type mismatch: TOML has a string but target expects int.
	input := `val = "not a number"
`
	type Config struct {
		Val int `toml:"val"`
	}
	var cfg Config
	err := Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("expected decoder type error")
	}
}

func TestUnmarshalDecoderScannerError(t *testing.T) {
	input := `key = "unterminated`
	dec := NewDecoder(strings.NewReader(input))
	var m map[string]any
	err := dec.Decode(&m)
	if err == nil {
		t.Fatal("expected scanner error via Decoder")
	}
}

func TestUnmarshalDecoderParserError(t *testing.T) {
	input := `key = 1
key = 2
`
	dec := NewDecoder(strings.NewReader(input))
	var m map[string]any
	err := dec.Decode(&m)
	if err == nil {
		t.Fatal("expected parser error via Decoder")
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestUnmarshalWithOptionsMaxDocumentSizeZeroMeansNoLimit(t *testing.T) {
	// maxDocumentSize=0 means no limit (default).
	input := `key = "value"
`
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(input), &m, WithMaxDocumentSize(0)); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

func TestUnmarshalWithOptionsMaxNodesZeroMeansNoLimit(t *testing.T) {
	input := `
a = 1
b = 2
c = 3
`
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(input), &m, WithMaxNodes(0)); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

func TestUnmarshalDecoderEmptyReader(t *testing.T) {
	dec := NewDecoder(strings.NewReader(""))
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("Decode empty: %v", err)
	}
}

func TestUnmarshalDecoderMultipleOptions(t *testing.T) {
	input := `name = "svc"
`
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port,default=7070"`
	}
	dec := NewDecoder(strings.NewReader(input), WithDefaults(), WithMaxDocumentSize(10000))
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Port != 7070 {
		t.Errorf("Port = %d, want 7070", cfg.Port)
	}
}

func TestUnmarshalDecodeFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.toml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var m map[string]any
	if err := DecodeFile(path, &m); err != nil {
		t.Fatalf("DecodeFile empty: %v", err)
	}
}
