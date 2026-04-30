package toml

import (
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

// WithIndent sets the number of spaces per indentation level (default 0).
// TOML convention uses no indentation, but some users prefer indented sub-tables.
func WithIndent(n int) EncodeOption {
	return func(o *encoderOptions) { o.indent = n }
}

// WithInlineTable encodes all nested tables as inline tables ({...}).
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

// WithBasicStrings forces all string values to use basic (double-quoted)
// strings instead of the encoder's default choice. This overrides the
// per-field literal tag.
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
func WithComment(comments map[string][]Comment) EncodeOption {
	return func(o *encoderOptions) { o.comments = comments }
}

// WithIndentTables indents key/value pairs within sub-tables relative to
// their table header.
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

// StructValidator validates a struct after all fields have been decoded.
// Implement this interface to integrate with validation libraries.
type StructValidator interface {
	Struct(v any) error
}

type decoderOptions struct {
	strict             bool
	useOrderedMap      bool
	useJSONUnmarshaler bool
	applyDefaults      bool
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
// variants, and time.Duration. Without this option, default tags are ignored.
func WithDefaults() DecodeOption {
	return func(o *decoderOptions) { o.applyDefaults = true }
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

// WithJSONUnmarshaler causes the decoder to try a type's UnmarshalJSON method
// if no TOML-specific unmarshaler is found.
func WithJSONUnmarshaler() DecodeOption {
	return func(o *decoderOptions) { o.useJSONUnmarshaler = true }
}

// WithMaxDepth limits the nesting depth of the decoded value (default 100).
// Deeply nested documents are rejected with a [SyntaxError].
func WithMaxDepth(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxDepth = n }
}

// WithMaxKeys limits the total number of keys the parser may encounter.
// Zero means no limit.
func WithMaxKeys(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxKeys = n }
}

// WithMaxDocumentSize rejects input that exceeds n bytes before parsing begins.
func WithMaxDocumentSize(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxDocumentSize = n }
}

// WithMaxNodes limits the total number of AST nodes the parser may create.
// Zero means no limit.
func WithMaxNodes(n int) DecodeOption {
	return func(o *decoderOptions) { o.maxNodes = n }
}

// WithValidator registers a [StructValidator] that is called after each struct
// is fully decoded.
func WithValidator(v StructValidator) DecodeOption {
	return func(o *decoderOptions) { o.validator = v }
}

// WithCustomUnmarshaler registers a function that decodes TOML bytes into a
// value of type T, overriding the default decoding for that type.
func WithCustomUnmarshaler[T any](fn func(*T, []byte) error) DecodeOption {
	return func(o *decoderOptions) {
		if o.customUnmarshalers == nil {
			o.customUnmarshalers = make(map[reflect.Type]any)
		}
		o.customUnmarshalers[reflect.TypeFor[T]()] = fn
	}
}

// WithLocalTimeZone sets the [time.Location] used when decoding local date-time
// values into [time.Time]. Default is [time.Local].
func WithLocalTimeZone(loc *time.Location) DecodeOption {
	return func(o *decoderOptions) { o.localTimeZone = loc }
}
