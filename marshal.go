package toml

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// Marshal encodes v as TOML. v must be a struct or map, or a pointer to one;
// otherwise Marshal returns [ErrMarshalTop].
func Marshal(v any) ([]byte, error) {
	return MarshalWithOptions(v)
}

// MarshalWithOptions encodes v as TOML with the given options.
func MarshalWithOptions(v any, opts ...EncodeOption) ([]byte, error) {
	o := defaultEncodeOptions()
	for _, opt := range opts {
		opt(o)
	}
	enc := newEncoder(o)
	return enc.encode(v)
}

// Encoder writes TOML values to an output stream.
type Encoder struct {
	w    io.Writer
	opts *encoderOptions
	ctx  context.Context
}

// NewEncoder creates a new [Encoder] that writes to w.
func NewEncoder(w io.Writer, opts ...EncodeOption) *Encoder {
	o := defaultEncodeOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Encoder{w: w, opts: o, ctx: context.Background()}
}

// Encode encodes v as TOML and writes it to the underlying writer.
func (enc *Encoder) Encode(v any) error {
	return enc.EncodeContext(enc.ctx, v)
}

// EncodeContext encodes v as TOML and writes it to the underlying writer,
// passing ctx to [MarshalerContext] implementations.
func (enc *Encoder) EncodeContext(ctx context.Context, v any) error {
	e := newEncoder(enc.opts)
	e.ctx = ctx
	data, err := e.encode(v)
	if err != nil {
		return err
	}
	_, err = enc.w.Write(data)
	return err
}

// Close calls Flush on the underlying writer if it has a Flush() error
// method; otherwise it does nothing. Each Encode call writes its whole
// document, so the Encoder itself buffers nothing.
func (enc *Encoder) Close() error {
	if f, ok := enc.w.(interface{ Flush() error }); ok {
		if err := f.Flush(); err != nil {
			return fmt.Errorf("toml: flush: %w", err)
		}
	}
	return nil
}

// SetContext sets the context for subsequent Encode calls.
func (enc *Encoder) SetContext(ctx context.Context) {
	enc.ctx = ctx
}

// RawValue is the source text of a single TOML value, such as a string or
// array literal, decoded on demand with [RawValue.Unmarshal]. The encoder and
// decoder give it no special treatment.
type RawValue []byte

// Unmarshal decodes r as the value of a key named "_raw_" into v, so v is a
// pointer to a map or to a struct with a field tagged `toml:"_raw_"`.
func (r RawValue) Unmarshal(v any) error {
	return Unmarshal(wrapAsValue(r), v)
}

func wrapAsValue(raw []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("_raw_ = ")
	buf.Write(raw)
	return buf.Bytes()
}
