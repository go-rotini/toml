package toml

import (
	"reflect"
	"testing"
	"time"
)

func TestDefaultEncodeOptions(t *testing.T) {
	opts := defaultEncodeOptions()
	if opts.tableSeparator != true {
		t.Error("tableSeparator should default to true")
	}
	if opts.indent != 0 {
		t.Error("indent should default to 0")
	}
	if opts.inlineTable {
		t.Error("inlineTable should default to false")
	}
	if opts.arrayMultiline {
		t.Error("arrayMultiline should default to false")
	}
	if opts.literalString {
		t.Error("literalString should default to false")
	}
	if opts.multiLineString {
		t.Error("multiLineString should default to false")
	}
	if opts.basicStrings {
		t.Error("basicStrings should default to false")
	}
	if opts.omitEmpty {
		t.Error("omitEmpty should default to false")
	}
	if opts.indentTables {
		t.Error("indentTables should default to false")
	}
	if opts.comments != nil {
		t.Error("comments should default to nil")
	}
	if opts.customMarshalers != nil {
		t.Error("customMarshalers should default to nil")
	}
}

func TestDefaultDecodeOptions(t *testing.T) {
	opts := defaultDecodeOptions()
	if opts.maxDepth != 100 {
		t.Errorf("maxDepth = %d, want 100", opts.maxDepth)
	}
	if opts.localTimeZone != time.Local {
		t.Errorf("localTimeZone = %v, want time.Local", opts.localTimeZone)
	}
	if opts.strict {
		t.Error("strict should default to false")
	}
	if opts.useOrderedMap {
		t.Error("useOrderedMap should default to false")
	}
	if opts.unmarshalJSON != nil {
		t.Error("useJSONUnmarshaler should default to false")
	}
	if opts.applyDefault != nil {
		t.Error("applyDefaults should default to false")
	}
	if opts.maxKeys != 0 {
		t.Errorf("maxKeys = %d, want 0", opts.maxKeys)
	}
	if opts.maxDocumentSize != 0 {
		t.Errorf("maxDocumentSize = %d, want 0", opts.maxDocumentSize)
	}
	if opts.maxNodes != 0 {
		t.Errorf("maxNodes = %d, want 0", opts.maxNodes)
	}
	if opts.validator != nil {
		t.Error("validator should default to nil")
	}
	if opts.customUnmarshalers != nil {
		t.Error("customUnmarshalers should default to nil")
	}
}

func TestWithIndent(t *testing.T) {
	opts := defaultEncodeOptions()
	WithIndent(4)(opts)
	if opts.indent != 4 {
		t.Errorf("indent = %d, want 4", opts.indent)
	}
}

func TestWithInlineTable(t *testing.T) {
	opts := defaultEncodeOptions()
	WithInlineTable(true)(opts)
	if !opts.inlineTable {
		t.Error("inlineTable should be true")
	}
}

func TestWithArrayMultiline(t *testing.T) {
	opts := defaultEncodeOptions()
	WithArrayMultiline(true)(opts)
	if !opts.arrayMultiline {
		t.Error("arrayMultiline should be true")
	}
}

func TestWithLiteralString(t *testing.T) {
	opts := defaultEncodeOptions()
	WithLiteralString(true)(opts)
	if !opts.literalString {
		t.Error("literalString should be true")
	}
}

func TestWithMultiLineString(t *testing.T) {
	opts := defaultEncodeOptions()
	WithMultiLineString(true)(opts)
	if !opts.multiLineString {
		t.Error("multiLineString should be true")
	}
}

func TestWithBasicStrings(t *testing.T) {
	opts := defaultEncodeOptions()
	WithBasicStrings(true)(opts)
	if !opts.basicStrings {
		t.Error("basicStrings should be true")
	}
}

func TestWithOmitEmpty(t *testing.T) {
	opts := defaultEncodeOptions()
	WithOmitEmpty(true)(opts)
	if !opts.omitEmpty {
		t.Error("omitEmpty should be true")
	}
}

func TestWithComment(t *testing.T) {
	comments := map[string][]Comment{
		"server.port": {
			{Position: HeadCommentPos, Text: "Server port"},
			{Position: LineCommentPos, Text: "default 8080"},
		},
	}
	opts := defaultEncodeOptions()
	WithComment(comments)(opts)
	if len(opts.comments) != 1 {
		t.Fatalf("comments length = %d, want 1", len(opts.comments))
	}
	got := opts.comments["server.port"]
	if len(got) != 2 {
		t.Fatalf("server.port comments = %d, want 2", len(got))
	}
	if got[0].Position != HeadCommentPos || got[0].Text != "Server port" {
		t.Errorf("first comment = %+v", got[0])
	}
	if got[1].Position != LineCommentPos || got[1].Text != "default 8080" {
		t.Errorf("second comment = %+v", got[1])
	}
}

func TestWithIndentTablesOption(t *testing.T) {
	opts := defaultEncodeOptions()
	WithIndentTables(true)(opts)
	if !opts.indentTables {
		t.Error("indentTables should be true")
	}
}

func TestWithTableSeparator(t *testing.T) {
	opts := defaultEncodeOptions()
	if !opts.tableSeparator {
		t.Fatal("tableSeparator should default to true")
	}
	WithTableSeparator(false)(opts)
	if opts.tableSeparator {
		t.Error("tableSeparator should be false after WithTableSeparator(false)")
	}
}

func TestWithCustomMarshaler(t *testing.T) {
	opts := defaultEncodeOptions()
	fn := func(v int) ([]byte, error) { return []byte("42"), nil }
	WithCustomMarshaler(fn)(opts)
	if len(opts.customMarshalers) != 1 {
		t.Fatalf("customMarshalers length = %d, want 1", len(opts.customMarshalers))
	}
	if _, ok := opts.customMarshalers[reflect.TypeFor[int]()]; !ok {
		t.Error("customMarshalers should contain int type")
	}
}

func TestWithCustomMarshalerMultiple(t *testing.T) {
	opts := defaultEncodeOptions()
	WithCustomMarshaler(func(v int) ([]byte, error) { return nil, nil })(opts)
	WithCustomMarshaler(func(v string) ([]byte, error) { return nil, nil })(opts)
	if len(opts.customMarshalers) != 2 {
		t.Fatalf("customMarshalers length = %d, want 2", len(opts.customMarshalers))
	}
}

func TestWithDefaults(t *testing.T) {
	opts := defaultDecodeOptions()
	WithDefaults()(opts)
	if !(opts.applyDefault != nil) {
		t.Error("applyDefaults should be true")
	}
}

func TestWithStrict(t *testing.T) {
	opts := defaultDecodeOptions()
	WithStrict()(opts)
	if !opts.strict {
		t.Error("strict should be true")
	}
}

func TestWithOrderedMap(t *testing.T) {
	opts := defaultDecodeOptions()
	WithOrderedMap()(opts)
	if !opts.useOrderedMap {
		t.Error("useOrderedMap should be true")
	}
}

func TestWithJSONUnmarshaler(t *testing.T) {
	opts := defaultDecodeOptions()
	WithJSONUnmarshaler()(opts)
	if opts.unmarshalJSON == nil {
		t.Error("useJSONUnmarshaler should be true")
	}
}

func TestWithMaxDepth(t *testing.T) {
	opts := defaultDecodeOptions()
	WithMaxDepth(50)(opts)
	if opts.maxDepth != 50 {
		t.Errorf("maxDepth = %d, want 50", opts.maxDepth)
	}
}

func TestWithMaxKeys(t *testing.T) {
	opts := defaultDecodeOptions()
	WithMaxKeys(1000)(opts)
	if opts.maxKeys != 1000 {
		t.Errorf("maxKeys = %d, want 1000", opts.maxKeys)
	}
}

func TestWithMaxDocumentSize(t *testing.T) {
	opts := defaultDecodeOptions()
	WithMaxDocumentSize(1 << 20)(opts)
	if opts.maxDocumentSize != 1<<20 {
		t.Errorf("maxDocumentSize = %d, want %d", opts.maxDocumentSize, 1<<20)
	}
}

func TestWithMaxNodes(t *testing.T) {
	opts := defaultDecodeOptions()
	WithMaxNodes(500)(opts)
	if opts.maxNodes != 500 {
		t.Errorf("maxNodes = %d, want 500", opts.maxNodes)
	}
}

type optionsTestValidator struct{}

func (v *optionsTestValidator) Struct(val any) error { return nil }

func TestWithValidatorOption(t *testing.T) {
	v := &optionsTestValidator{}
	opts := defaultDecodeOptions()
	WithValidator(v)(opts)
	if opts.validator != v {
		t.Error("validator not set")
	}
}

func TestWithCustomUnmarshaler(t *testing.T) {
	opts := defaultDecodeOptions()
	fn := func(v *int, data []byte) error { return nil }
	WithCustomUnmarshaler(fn)(opts)
	if len(opts.customUnmarshalers) != 1 {
		t.Fatalf("customUnmarshalers length = %d, want 1", len(opts.customUnmarshalers))
	}
	if _, ok := opts.customUnmarshalers[reflect.TypeFor[int]()]; !ok {
		t.Error("customUnmarshalers should contain int type")
	}
}

func TestWithCustomUnmarshalerMultiple(t *testing.T) {
	opts := defaultDecodeOptions()
	WithCustomUnmarshaler(func(v *int, data []byte) error { return nil })(opts)
	WithCustomUnmarshaler(func(v *string, data []byte) error { return nil })(opts)
	if len(opts.customUnmarshalers) != 2 {
		t.Fatalf("customUnmarshalers length = %d, want 2", len(opts.customUnmarshalers))
	}
}

func TestWithLocalTimeZone(t *testing.T) {
	loc := time.FixedZone("EST", -5*60*60)
	opts := defaultDecodeOptions()
	WithLocalTimeZone(loc)(opts)
	if opts.localTimeZone != loc {
		t.Errorf("localTimeZone = %v, want EST", opts.localTimeZone)
	}
}

func TestCommentPositionValues(t *testing.T) {
	if HeadCommentPos != 0 {
		t.Errorf("HeadCommentPos = %d, want 0", HeadCommentPos)
	}
	if LineCommentPos != 1 {
		t.Errorf("LineCommentPos = %d, want 1", LineCommentPos)
	}
	if FootCommentPos != 2 {
		t.Errorf("FootCommentPos = %d, want 2", FootCommentPos)
	}
}

func TestEncodeOptionComposition(t *testing.T) {
	opts := defaultEncodeOptions()
	options := []EncodeOption{
		WithIndent(2),
		WithInlineTable(true),
		WithLiteralString(true),
		WithTableSeparator(false),
	}
	for _, o := range options {
		o(opts)
	}
	if opts.indent != 2 {
		t.Errorf("indent = %d, want 2", opts.indent)
	}
	if !opts.inlineTable {
		t.Error("inlineTable should be true")
	}
	if !opts.literalString {
		t.Error("literalString should be true")
	}
	if opts.tableSeparator {
		t.Error("tableSeparator should be false")
	}
}

func TestDecodeOptionComposition(t *testing.T) {
	opts := defaultDecodeOptions()
	options := []DecodeOption{
		WithStrict(),
		WithDefaults(),
		WithMaxDepth(50),
		WithMaxKeys(500),
		WithOrderedMap(),
	}
	for _, o := range options {
		o(opts)
	}
	if !opts.strict {
		t.Error("strict should be true")
	}
	if !(opts.applyDefault != nil) {
		t.Error("applyDefaults should be true")
	}
	if opts.maxDepth != 50 {
		t.Errorf("maxDepth = %d, want 50", opts.maxDepth)
	}
	if opts.maxKeys != 500 {
		t.Errorf("maxKeys = %d, want 500", opts.maxKeys)
	}
	if !opts.useOrderedMap {
		t.Error("useOrderedMap should be true")
	}
}
