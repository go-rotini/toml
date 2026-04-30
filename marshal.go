package toml

import (
	"bytes"
	"context"
	"io"
)

// Marshal encodes v as TOML. The top-level value must be a struct or map.
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

// EncodeContext encodes v as TOML with the given context.
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

// Close flushes any buffered data. For TOML, this is a no-op since each
// Encode call writes completely.
func (enc *Encoder) Close() error {
	if f, ok := enc.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// SetContext sets the context for subsequent Encode calls.
func (enc *Encoder) SetContext(ctx context.Context) {
	enc.ctx = ctx
}

// RawValue is raw TOML that has not been decoded. It can be used to
// delay decoding or to pass through a TOML value without interpreting it.
type RawValue []byte

// Unmarshal decodes the raw TOML value into v.
func (r RawValue) Unmarshal(v any) error {
	return Unmarshal(wrapAsValue(r), v)
}

func wrapAsValue(raw []byte) []byte {
	// Wrap as a key-value pair so the decoder can handle it
	var buf bytes.Buffer
	buf.WriteString("_raw_ = ")
	buf.Write(raw)
	return buf.Bytes()
}
