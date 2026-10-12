package toml

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// Position identifies a location within TOML source text.
type Position struct {
	Line   int
	Column int
	Offset int
}

// String returns the position formatted as "line:column".
func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// SyntaxError is returned when the TOML input is malformed, when a key
// tagged required is missing, and when a decode limit such as [WithMaxDepth]
// is exceeded. Use [errors.Is](err, [ErrSyntax]) to test for it.
type SyntaxError struct {
	Message string
	Pos     Position
	Token   string
}

// Error implements the error interface.
func (e *SyntaxError) Error() string {
	if e.Pos.Line > 0 {
		return fmt.Sprintf("toml: line %d, column %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
	}
	return fmt.Sprintf("toml: %s", e.Message)
}

// Is reports whether target is a *SyntaxError, so [errors.Is] matches any
// SyntaxError against its sentinel.
func (e *SyntaxError) Is(target error) bool {
	_, ok := target.(*SyntaxError)
	return ok
}

// TypeError is returned when one or more TOML values cannot be assigned to
// the target Go types. Errors contains a message for each failed conversion.
type TypeError struct {
	Errors []string
}

// Error implements the error interface.
func (e *TypeError) Error() string {
	return fmt.Sprintf("toml: unmarshal errors:\n  %s", strings.Join(e.Errors, "\n  "))
}

// Is reports whether target is a *TypeError, so [errors.Is] matches any
// TypeError against its sentinel.
func (e *TypeError) Is(target error) bool {
	_, ok := target.(*TypeError)
	return ok
}

// UnknownFieldError is returned when decoding with [WithStrict] and a TOML key
// has no corresponding struct field.
type UnknownFieldError struct {
	Field string
	Pos   Position
}

// Error implements the error interface.
func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("toml: line %d: unknown field %q", e.Pos.Line, e.Field)
}

// Is reports whether target is an *UnknownFieldError, so [errors.Is] matches any
// UnknownFieldError against its sentinel.
func (e *UnknownFieldError) Is(target error) bool {
	_, ok := target.(*UnknownFieldError)
	return ok
}

// DuplicateKeyError is returned when a TOML key is defined more than once.
// TOML requires unique keys within a table; this error is always active.
type DuplicateKeyError struct {
	Key string
	Pos Position
}

// Error implements the error interface.
func (e *DuplicateKeyError) Error() string {
	return fmt.Sprintf("toml: line %d: duplicate key %q", e.Pos.Line, e.Key)
}

// Is reports whether target is a *DuplicateKeyError, so [errors.Is] matches any
// DuplicateKeyError against its sentinel.
func (e *DuplicateKeyError) Is(target error) bool {
	_, ok := target.(*DuplicateKeyError)
	return ok
}

// ValidationError wraps an error returned by a [StructValidator] with the
// [Position] of the TOML node that was decoded into the struct.
type ValidationError struct {
	Err error
	Pos Position
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("toml: line %d: validation: %s", e.Pos.Line, e.Err.Error())
}

// Unwrap returns the validator's error.
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// Is reports whether target is a *ValidationError, so [errors.Is] matches any
// ValidationError against its sentinel.
func (e *ValidationError) Is(target error) bool {
	_, ok := target.(*ValidationError)
	return ok
}

// DefaultError is returned when a default value from a struct tag cannot be
// applied.
type DefaultError struct {
	Field   string
	Message string
	Pos     Position
}

// Error implements the error interface.
func (e *DefaultError) Error() string {
	return fmt.Sprintf("toml: field %q: %s", e.Field, e.Message)
}

// Is reports whether target is a *DefaultError, so [errors.Is] matches any
// DefaultError against its sentinel.
func (e *DefaultError) Is(target error) bool {
	_, ok := target.(*DefaultError)
	return ok
}

// OverflowError is returned when a TOML integer overflows the target Go type.
type OverflowError struct {
	Value string
	Type  string
	Pos   Position
}

// Error implements the error interface.
func (e *OverflowError) Error() string {
	return fmt.Sprintf("toml: line %d: value %s overflows %s", e.Pos.Line, e.Value, e.Type)
}

// Is reports whether target is an *OverflowError, so [errors.Is] matches any
// OverflowError against its sentinel.
func (e *OverflowError) Is(target error) bool {
	_, ok := target.(*OverflowError)
	return ok
}

// InlineTableError is returned when an inline table is modified after
// its initial definition.
type InlineTableError struct {
	Table string
	Pos   Position
}

// Error implements the error interface.
func (e *InlineTableError) Error() string {
	return fmt.Sprintf("toml: line %d: cannot extend inline table %q", e.Pos.Line, e.Table)
}

// Is reports whether target is an *InlineTableError, so [errors.Is] matches any
// InlineTableError against its sentinel.
func (e *InlineTableError) Is(target error) bool {
	_, ok := target.(*InlineTableError)
	return ok
}

// ConflictError is returned when a key conflicts with an existing definition,
// such as defining a key as both a table and an array of tables.
type ConflictError struct {
	Key     string
	Message string
	Pos     Position
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	return fmt.Sprintf("toml: line %d: key %q: %s", e.Pos.Line, e.Key, e.Message)
}

// Is reports whether target is a *ConflictError, so [errors.Is] matches any
// ConflictError against its sentinel.
func (e *ConflictError) Is(target error) bool {
	_, ok := target.(*ConflictError)
	return ok
}

// Sentinel errors for use with [errors.Is].
var (
	// ErrSyntax indicates malformed TOML input.
	ErrSyntax = &SyntaxError{}
	// ErrType indicates one or more TOML values could not be assigned to the target Go types.
	ErrType = &TypeError{}
	// ErrUnknownField indicates a TOML key has no corresponding struct field (with [WithStrict]).
	ErrUnknownField = &UnknownFieldError{}
	// ErrDuplicateKey indicates a TOML key was defined more than once within a table.
	ErrDuplicateKey = &DuplicateKeyError{}
	// ErrValidation indicates a [StructValidator] rejected a decoded struct.
	ErrValidation = &ValidationError{}
	// ErrDefault indicates a default struct tag value could not be applied.
	ErrDefault = &DefaultError{}
	// ErrOverflow indicates a TOML integer overflows the target Go type.
	ErrOverflow = &OverflowError{}
	// ErrInlineTable indicates an inline table was modified after its initial definition.
	ErrInlineTable = &InlineTableError{}
	// ErrConflict indicates a key conflicts with an existing definition.
	ErrConflict = &ConflictError{}

	// ErrPathSyntax indicates an invalid [Path] expression.
	ErrPathSyntax = errors.New("toml: invalid path syntax")
	// ErrPathNotFound indicates no node matched a [Path] query.
	ErrPathNotFound = errors.New("toml: path not found")
	// ErrPointerSyntax indicates an invalid RFC 6901 JSON Pointer passed to [PathPointer].
	ErrPointerSyntax = errors.New("toml: invalid JSON Pointer syntax")
	// ErrNilPointer indicates a nil pointer was passed where a non-nil pointer is required.
	ErrNilPointer = errors.New("toml: non-nil pointer required")
	// ErrDocumentSize indicates the input exceeds the configured [WithMaxDocumentSize] limit.
	ErrDocumentSize = errors.New("toml: document size exceeds limit")
	// ErrMarshalTop indicates the top-level value passed to [Marshal] is not a struct or map.
	ErrMarshalTop = errors.New("toml: top-level value must be a table (struct or map)")
)

var (
	errNonPointerTarget       = errors.New("toml: non-pointer target")
	errUnsupportedKeyType     = errors.New("toml: cannot set key type")
	errInvalidBase64Char      = errors.New("invalid base64 character")
	errUnsupportedEncodeType  = errors.New("toml: cannot encode as table")
	errUnsupportedFieldType   = errors.New("toml: unsupported type")
	errLeadingZeros           = errors.New("leading zeros are not allowed in decimal integers")
	errInvalidLocalTime       = errors.New("toml: invalid local time")
	errInvalidLocalDateTime   = errors.New("toml: invalid local date-time")
	errInvalidBoolDefault     = errors.New("invalid bool default")
	errInvalidDurationDefault = errors.New("invalid duration default")
	errInvalidIntDefault      = errors.New("invalid int default")
	errInvalidUintDefault     = errors.New("invalid uint default")
	errInvalidFloatDefault    = errors.New("invalid float default")
	errUnsupportedDefault     = errors.New("default tag is not supported for type")
)

// FormatError returns a human-readable string for errors that carry a
// [Position] ([SyntaxError], [DuplicateKeyError], [OverflowError],
// [InlineTableError], [ConflictError], [UnknownFieldError] or
// [ValidationError], wrapped or not). The output
// includes the offending source line and a column pointer. For other error
// types it returns err.Error(). Set color to true to include ANSI color
// escape sequences.
func FormatError(data []byte, err error, color ...bool) string {
	pos := extractPosition(err)
	if pos.Line == 0 {
		return err.Error()
	}

	lines := bytes.Split(data, []byte("\n"))
	lineIdx := pos.Line - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return err.Error()
	}

	useColor := len(color) > 0 && color[0]

	var buf bytes.Buffer
	if useColor {
		fmt.Fprintf(&buf, "\x1b[1;31m%s\x1b[0m\n", err.Error())
		fmt.Fprintf(&buf, "  %s\n", string(lines[lineIdx]))
		fmt.Fprintf(&buf, "  %s\x1b[1;31m^\x1b[0m\n", repeatByte(' ', pos.Column-1))
	} else {
		fmt.Fprintf(&buf, "%s\n", err.Error())
		fmt.Fprintf(&buf, "  %s\n", string(lines[lineIdx]))
		fmt.Fprintf(&buf, "  %s^\n", repeatByte(' ', pos.Column-1))
	}
	return buf.String()
}

func extractPosition(err error) Position {
	var synErr *SyntaxError
	var valErr *ValidationError
	var dupErr *DuplicateKeyError
	var ovfErr *OverflowError
	var inlErr *InlineTableError
	var conErr *ConflictError
	var unkErr *UnknownFieldError
	switch {
	case errors.As(err, &synErr):
		return synErr.Pos
	case errors.As(err, &valErr):
		return valErr.Pos
	case errors.As(err, &dupErr):
		return dupErr.Pos
	case errors.As(err, &ovfErr):
		return ovfErr.Pos
	case errors.As(err, &inlErr):
		return inlErr.Pos
	case errors.As(err, &conErr):
		return conErr.Pos
	case errors.As(err, &unkErr):
		return unkErr.Pos
	default:
		return Position{}
	}
}

func repeatByte(b byte, n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = b
	}
	return string(buf)
}
