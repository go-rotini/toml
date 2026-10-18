package toml

import (
	"encoding/json"
	"reflect"
	"time"
)

// EncodeOption configures the behavior of [Marshal], [MarshalWithOptions],
// and [Encoder].
type EncodeOption func(*encoderOptions)

// Comment attaches a comment to a TOML node identified by key path when
// encoding with [WithComment].
type Comment struct {
	Position CommentPosition
	Text     string
}

// CommentPosition specifies where a [Comment] appears relative to its node.
type CommentPosition int

const (
	HeadCommentPos CommentPosition = iota // before the node
	LineCommentPos                        // on the same line, after the value
	FootCommentPos                        // after the node
)

type encoderOptions struct {
	indent           int
	inlineTable      bool
	arrayMultiline   bool
	literalString    bool
	multiLineString  bool
	basicStrings     bool
	omitEmpty        bool
	indentTables     bool
	tableSeparator   bool
	comments         map[string][]Comment
	customMarshalers map[reflect.Type]any
}

func defaultEncodeOptions() *encoderOptions {
	return &encoderOptions{
		tableSeparator: true,
	}
}

// WithIndent indents the key/value pairs inside a table by n spaces per
// nesting level (default 0, no indentation). [WithIndentTables] applies the
// same width to nested table headers.
func WithIndent(n int) EncodeOption {
	return func(o *encoderOptions) { o.indent = n }
}

// WithInlineTable encodes all nested tables as inline tables ({...}) instead
// of [table] sections, and arrays of tables as arrays of inline tables.
func WithInlineTable(b bool) EncodeOption {
	return func(o *encoderOptions) { o.inlineTable = b }
}

// WithArrayMultiline encodes all arrays in multiline format (one element per line).
func WithArrayMultiline(b bool) EncodeOption {
	return func(o *encoderOptions) { o.arrayMultiline = b }
}

// WithLiteralString prefers literal strings (single-quoted) when the value
// contains no characters that would require a basic string.
func WithLiteralString(b bool) EncodeOption {
	return func(o *encoderOptions) { o.literalString = b }
}

// WithMultiLineString encodes strings containing newlines as multi-line
// basic strings (triple-quoted).
func WithMultiLineString(b bool) EncodeOption {
	return func(o *encoderOptions) { o.multiLineString = b }
}

// WithBasicStrings overrides [WithLiteralString] so strings are written as
// basic (double-quoted) strings. A field's literal tag still applies.
func WithBasicStrings(b bool) EncodeOption {
	return func(o *encoderOptions) { o.basicStrings = b }
}

// WithOmitEmpty omits struct fields and map entries whose values are zero/empty,
// equivalent to adding ",omitempty" to every field tag.
func WithOmitEmpty(b bool) EncodeOption {
	return func(o *encoderOptions) { o.omitEmpty = b }
}

// WithComment attaches comments to nodes by dot-path key (e.g. "server.port").
// Each key maps to a slice of [Comment] values specifying position and text.
// A key may name a key/value pair or a table. Head comments go on the lines
// above the pair or table header, a line comment after the value or header,
// and foot comments on the lines after the pair or after the table's
// contents. For an array of tables, head and line comments go on the first
// [[header]] and foot comments after the last entry.
func WithComment(comments map[string][]Comment) EncodeOption {
	return func(o *encoderOptions) { o.comments = comments }
}

// WithIndentTables indents nested table headers, and the comments above them,
// by their depth using the [WithIndent] width. It has no effect when that
// width is 0.
func WithIndentTables(b bool) EncodeOption {
	return func(o *encoderOptions) { o.indentTables = b }
}

// WithTableSeparator controls whether a blank line is emitted between
// top-level tables for readability (default true).
func WithTableSeparator(b bool) EncodeOption {
	return func(o *encoderOptions) { o.tableSeparator = b }
}

// WithCustomMarshaler registers a function that encodes values of type T to
// TOML bytes, overriding the default encoding for that type.
func WithCustomMarshaler[T any](fn func(T) ([]byte, error)) EncodeOption {
	return func(o *encoderOptions) {
		if o.customMarshalers == nil {
			o.customMarshalers = make(map[reflect.Type]any)
		}
		o.customMarshalers[reflect.TypeFor[T]()] = fn
	}
}

// DecodeOption configures the behavior of [Unmarshal], [UnmarshalWithOptions],
// and [Decoder].
type DecodeOption func(*decoderOptions)

// StructValidator validates a struct after all of its fields have been
// decoded. Struct receives a pointer to the struct. Register one with
// [WithValidator].
type StructValidator interface {
	Struct(v any) error
}

type decoderOptions struct {
	strict             bool
	useOrderedMap      bool
	unmarshalJSON      func(*decoder, *node, json.Unmarshaler) error  // (*decoder).unmarshalJSON, set by WithJSONUnmarshaler, so a decoder without it doesn't link encoding/json
	applyDefault       func(*decoder, fieldInfo, reflect.Value) error // set by WithDefaults; nil leaves default tags unread and unlinked
	maxDepth           int
	maxKeys            int
	maxDocumentSize    int
	maxNodes           int
	localTimeZone      *time.Location
	validator          StructValidator
	customUnmarshalers map[reflect.Type]any
}

func defaultDecodeOptions() *decoderOptions {
	return &decoderOptions{
		maxDepth:      100,
		localTimeZone: time.Local,
	}
}

// WithDefaults enables applying default values from struct tags when a TOML
// key is absent from the input. Default values are specified with the
// "default=<value>" tag option (e.g. `toml:"port,default=8080"`).
// Only scalar types are supported: string, bool, int/uint variants, float
// variants, and time.Duration. A default that does not parse, or one on any
// other type, returns a [DefaultError]. Without this option, default tags are
// ignored.
func WithDefaults() DecodeOption {
	return func(o *decoderOptions) { o.applyDefault = (*decoder).applyDefault }
}

// WithStrict causes decoding to return an [UnknownFieldError] if a TOML key
// does not correspond to any field in the target struct.
func WithStrict() DecodeOption {
	return func(o *decoderOptions) { o.strict = true }
}

// WithOrderedMap causes decoding into any (interface{}) to produce [MapSlice]
// values for tables instead of map[string]any, preserving key order.
func WithOrderedMap() DecodeOption {
	return func(o *decoderOptions) { o.useOrderedMap = true }
}

// WithJSONUnmarshaler causes the decoder to use a type's UnmarshalJSON method,
// passing it the TOML value encoded as JSON (a table as an object), when the
// type has no TOML-specific unmarshaler: an [Unmarshaler],
// [UnmarshalerContext], [BytesUnmarshaler], [encoding.TextUnmarshaler] for a
// scalar, or [WithCustomUnmarshaler] function. [time.Time], [math/big.Int],
// [math/big.Float] and the local date and time types keep their TOML decoding.
func WithJSONUnmarshaler() DecodeOption {
	return func(o *decoderOptions) { o.unmarshalJSON = (*decoder).unmarshalJSON }
}

// WithMaxDepth limits how deeply tables, inline tables and arrays may nest,
// counting the root table as 1 (default 100; zero means no limit). Deeper
// input is rejected with a [SyntaxError] while it is parsed. Whatever the
// limit, nesting deeper than 10000 levels is always rejected, as it is by
// [Parse] and [Valid].
func WithMaxDepth(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxDepth = n }
}

// WithMaxKeys limits the total number of key/value pairs decoded into structs
// and maps; exceeding it returns a [SyntaxError]. Zero means no limit.
func WithMaxKeys(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxKeys = n }
}

// WithMaxDocumentSize rejects input longer than n bytes with
// [ErrDocumentSize] before parsing begins. Zero means no limit.
func WithMaxDocumentSize(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxDocumentSize = n }
}

// WithMaxNodes limits the total number of AST nodes the parser may create;
// exceeding it returns a [SyntaxError]. Zero means no limit.
func WithMaxNodes(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxNodes = n }
}

// WithValidator registers a [StructValidator] that is called after each struct
// is fully decoded. An error it returns is wrapped in a [ValidationError].
func WithValidator(v StructValidator) DecodeOption {
	return func(o *decoderOptions) { o.validator = v }
}

// WithCustomUnmarshaler registers a function that decodes TOML bytes into a
// value of type T, overriding the default decoding for that type. It receives
// the same bytes as a [BytesUnmarshaler].
func WithCustomUnmarshaler[T any](fn func(*T, []byte) error) DecodeOption {
	return func(o *decoderOptions) {
		if o.customUnmarshalers == nil {
			o.customUnmarshalers = make(map[reflect.Type]any)
		}
		o.customUnmarshalers[reflect.TypeFor[T]()] = fn
	}
}

// WithLocalTimeZone sets the [time.Location] used when decoding local date
// and local date-time values into [time.Time]. Default is [time.Local].
func WithLocalTimeZone(loc *time.Location) DecodeOption {
	return func(o *decoderOptions) { o.localTimeZone = loc }
}
