// Package toml implements TOML 1.1.0 encoding and decoding.
//
// The API follows the conventions of [encoding/json]: use [Marshal] and
// [Unmarshal] for one-shot conversions, [Encoder] and [Decoder] for streaming,
// and struct field tags to control mapping between TOML keys and Go fields.
//
// For low-level AST access, [Parse] returns a [File] containing a [Node] tree
// that can be inspected, mutated with [Path] queries, and re-serialized with
// [NodeToBytes]. Each parsed node also carries its [Span] in the source, so a
// value can be replaced in place without disturbing comments or layout.
//
// # Struct Tags
//
// Struct fields may be annotated with "toml" tags:
//
//	type Config struct {
//	    Name    string `toml:"name"`
//	    Count   int    `toml:"count,omitempty"`
//	    Ignored string `toml:"-"`
//	}
//
// The tag format is "keyname,opts" where opts is a comma-separated list of:
//   - omitempty: omit the field if it is a zero value or an empty slice or map
//   - omitzero: same as omitempty
//   - inline: on a struct field, merge its fields into the parent table as
//     an embedded struct's are; on a map field, encode it as an inline table
//   - multiline: encode strings as multi-line basic strings (multi-line
//     literal strings when combined with literal)
//   - literal: encode strings as literal (single-quoted) strings
//   - commented: encode the field as a commented-out line
//   - hex: encode integers in hexadecimal (0x) format
//   - octal: encode integers in octal (0o) format
//   - binary: encode integers in binary (0b) format
//   - required: return an error during decoding if the key is absent
//   - default=<value>: set field to <value> during decoding if the key is absent
//     (requires [WithDefaults]; scalar types only)
//
// A tag of "-" excludes the field from encoding and decoding. A field with
// no "toml" tag uses its "json" tag, and a field with neither is keyed by its
// lower-cased name. Decoding falls back to a case-insensitive match when no
// field name matches a key exactly. A "comment" tag writes its text as a
// comment line above the field when encoding.
//
// # Custom Marshalers
//
// Types can implement [Marshaler], [BytesMarshaler], [MarshalerContext],
// [Unmarshaler], [BytesUnmarshaler], or [UnmarshalerContext] for custom
// serialization logic.
//
// # Error Handling
//
// Decoding errors are returned as typed values that support [errors.Is]:
//
//	if errors.Is(err, toml.ErrSyntax) { ... }
//	if errors.Is(err, toml.ErrDuplicateKey) { ... }
//
// Use [FormatError] to produce a human-readable error with a source pointer.
package toml
