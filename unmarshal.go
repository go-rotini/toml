package toml

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Unmarshal decodes TOML data into v, which must be a non-nil pointer
// (otherwise Unmarshal returns [ErrNilPointer]) to a struct, a map with string
// or [encoding.TextUnmarshaler] keys, or an interface value, which receives a
// map[string]any ([MapSlice] with [WithOrderedMap]).
func Unmarshal(data []byte, v any) error {
	return UnmarshalWithOptions(data, v)
}

// UnmarshalWithOptions decodes TOML data into v with the given options.
func UnmarshalWithOptions(data []byte, v any, opts ...DecodeOption) error {
	o := defaultDecodeOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.maxDocumentSize > 0 && len(data) > o.maxDocumentSize {
		return ErrDocumentSize
	}

	s, err := newScanner(data)
	if err != nil {
		return err
	}
	tokens, err := s.scan()
	if err != nil {
		return err
	}

	p := newParser(tokens)
	p.maxNodes = o.maxNodes
	root, err := p.parse()
	if err != nil {
		return err
	}

	d := newDecoder(o)
	return d.decode(root, v)
}

// UnmarshalTo decodes TOML data into a new value of type T.
func UnmarshalTo[T any](data []byte, opts ...DecodeOption) (T, error) {
	var v T
	err := UnmarshalWithOptions(data, &v, opts...)
	return v, err
}

// DecodeFile reads and decodes the TOML file at path into v.
func DecodeFile(path string, v any, opts ...DecodeOption) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("toml: %w", err)
	}
	return UnmarshalWithOptions(data, v, opts...)
}

// Decoder reads TOML values from an input stream. Since TOML does not
// support multi-document streams, the first call to [Decoder.Decode] reads
// the entire input; subsequent calls return [io.EOF].
type Decoder struct {
	r    io.Reader
	opts *decoderOptions
	ctx  context.Context
	done bool
}

// NewDecoder creates a new [Decoder] that reads from r.
func NewDecoder(r io.Reader, opts ...DecodeOption) *Decoder {
	o := defaultDecodeOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Decoder{r: r, opts: o, ctx: context.Background()}
}

// Decode reads the TOML input and decodes it into v.
func (dec *Decoder) Decode(v any) error {
	return dec.DecodeContext(dec.ctx, v)
}

// DecodeContext reads the TOML input and decodes it into v, passing ctx to
// [UnmarshalerContext] implementations.
func (dec *Decoder) DecodeContext(ctx context.Context, v any) error {
	if dec.done {
		return io.EOF
	}
	dec.done = true

	data, err := io.ReadAll(dec.r)
	if err != nil {
		return fmt.Errorf("toml: %w", err)
	}

	if dec.opts.maxDocumentSize > 0 && len(data) > dec.opts.maxDocumentSize {
		return ErrDocumentSize
	}

	sc, err := newScanner(data)
	if err != nil {
		return err
	}
	tokens, err := sc.scan()
	if err != nil {
		return err
	}

	p := newParser(tokens)
	p.maxNodes = dec.opts.maxNodes
	root, err := p.parse()
	if err != nil {
		return err
	}

	d := newDecoder(dec.opts)
	d.ctx = ctx
	return d.decode(root, v)
}

// SetContext sets the context for subsequent Decode calls.
func (dec *Decoder) SetContext(ctx context.Context) {
	dec.ctx = ctx
}
