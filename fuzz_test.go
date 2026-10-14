package toml

import (
	"math"
	"reflect"
	"testing"
)

// FuzzUnmarshal feeds random bytes to Unmarshal. It must never panic;
// parse/decode errors are expected and acceptable.
func FuzzUnmarshal(f *testing.F) {
	f.Add([]byte(`key = "value"`))
	f.Add([]byte(`num = 42`))
	f.Add([]byte(`pi = 3.14`))
	f.Add([]byte(`enabled = true`))
	f.Add([]byte(`arr = [1, 2, 3]`))
	f.Add([]byte(`[table]` + "\n" + `key = "val"`))
	f.Add([]byte(`[[array]]` + "\n" + `name = "first"`))
	f.Add([]byte(``))
	f.Add([]byte(`# just a comment`))
	f.Add([]byte("\xEF\xBB\xBF" + `key = "bom"`)) // BOM prefix

	f.Fuzz(func(t *testing.T, data []byte) {
		var m map[string]any
		_ = Unmarshal(data, &m)
	})
}

// FuzzScanner feeds random bytes to the scanner. It must never panic;
// scan errors are expected and acceptable.
func FuzzScanner(f *testing.F) {
	f.Add([]byte(`key = "value"`))
	f.Add([]byte(`[table]`))
	f.Add([]byte(`[[array]]`))
	f.Add([]byte(`"quoted" = 'literal'`))
	f.Add([]byte(`num = 0xFF`))
	f.Add([]byte(`dt = 1979-05-27T07:32:00Z`))
	f.Add([]byte(`ml = """\nmulti\n"""`))
	f.Add([]byte(``))
	f.Add([]byte("\x00\x01\x02\x03"))

	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := newScanner(data)
		if err != nil {
			return
		}
		_, _ = s.scan()
	})
}

// FuzzRoundTrip verifies that valid TOML survives a Marshal/Unmarshal
// round trip: Unmarshal -> Marshal -> Unmarshal must yield an equivalent map.
func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte(`key = "value"` + "\n" + `num = 42`))
	f.Add([]byte(`[server]` + "\n" + `host = "localhost"` + "\n" + `port = 8080`))
	f.Add([]byte(`enabled = true` + "\n" + `pi = 3.14`))
	f.Add([]byte(`colors = ["red", "green", "blue"]`))
	f.Add([]byte(`inline = { a = 1, b = 2 }`))
	f.Add([]byte(`name = ""`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var m1 map[string]any
		if err := Unmarshal(data, &m1); err != nil {
			t.Skip("invalid TOML")
		}

		encoded, err := Marshal(m1)
		if err != nil {
			// Some decoded values may not be marshalable; that is fine.
			t.Skip("marshal error")
		}

		var m2 map[string]any
		if err := Unmarshal(encoded, &m2); err != nil {
			t.Fatalf("re-Unmarshal failed: %v\noriginal input:\n%s\nre-marshaled:\n%s", err, data, encoded)
		}

		if !roundTripEqual(m1, m2) {
			t.Fatalf("round-trip mismatch\noriginal:     %#v\nre-marshaled: %#v\nencoded TOML:\n%s", m1, m2, encoded)
		}
	})
}

// roundTripEqual is reflect.DeepEqual, except that two NaN floats are equal: NaN never
// equals itself, but `nan` decoding to NaN on both sides is a correct round trip.
func roundTripEqual(a, b any) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && (av == bv || (math.IsNaN(av) && math.IsNaN(bv)))
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			w, ok := bv[k]
			if !ok || !roundTripEqual(v, w) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !roundTripEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// FuzzValid feeds random bytes to Valid. It must never panic.
func FuzzValid(f *testing.F) {
	f.Add([]byte(`key = "value"`))
	f.Add([]byte(`[table]`))
	f.Add([]byte(`bad toml !!!`))
	f.Add([]byte(``))
	f.Add([]byte("\x00"))
	f.Add([]byte(`a = 1` + "\n" + `b = 2`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = Valid(data)
	})
}

// FuzzSpans parses random documents and checks that every node's source span
// is in bounds and its text parses back to the same key or value.
func FuzzSpans(f *testing.F) {
	f.Add([]byte(spanDoc))
	f.Add([]byte("a.\"b c\".d = [1, {x = 'y'}]\r\n[[t]]\r\ns = \"\"\"\r\nx\"\"\"\r\n"))
	f.Add([]byte("\uFEFFk = 1 # c\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Parse(data)
		if err != nil {
			return
		}
		checkSpans(t, data, file.Root)
	})
}

// FuzzNodeRoundTrip parses a document, writes the tree back with NodeToBytes,
// and checks the output decodes to the same values as the input.
func FuzzNodeRoundTrip(f *testing.F) {
	f.Add([]byte(benchConfig))
	f.Add([]byte(spanDoc))
	f.Add([]byte("a = \"\"\"x\\ry\"\"\"\nb = '''l\nm'''\n[[t]]\nk.v = [1, {x = 'y'}]\n[[t]]\n[t.s]\n"))
	f.Add([]byte("[[arr]]\na = [1]\n[[arr]]\n[[arr.a]]\nx = 1\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var want map[string]any
		if err := UnmarshalWithOptions(data, &want, WithMaxDepth(0)); err != nil {
			return
		}
		file, err := Parse(data)
		if err != nil {
			t.Fatalf("Unmarshal accepted the input but Parse failed: %v", err)
		}
		out, err := NodeToBytes(file.Root)
		if err != nil {
			t.Fatalf("NodeToBytes: %v", err)
		}
		var got map[string]any
		if err := UnmarshalWithOptions(out, &got, WithMaxDepth(0)); err != nil {
			t.Fatalf("output does not decode: %v\ninput:  %q\noutput: %q", err, data, out)
		}
		if !roundTripEqual(want, got) {
			t.Fatalf("round-trip mismatch\ninput:  %q\noutput: %q", data, out)
		}
	})
}

type fuzzStruct struct {
	S   string            `toml:"s"`
	ML  string            `toml:"ml,multiline"`
	MLL string            `toml:"mll,multiline,literal"`
	L   string            `toml:"l,literal"`
	I   int64             `toml:"i"`
	H   int64             `toml:"h,hex"`
	O   int32             `toml:"o,octal"`
	B   int16             `toml:"b,binary"`
	U   uint32            `toml:"u"`
	F   float64           `toml:"f"`
	F32 float32           `toml:"f32"`
	T   bool              `toml:"t"`
	A   []string          `toml:"a"`
	M   map[string]string `toml:"m"`
	N   struct {
		X string `toml:"x"`
	} `toml:"n"`
	R []struct {
		Y int64 `toml:"y"`
	} `toml:"r"`
}

// FuzzStructRoundTrip checks that arbitrary field values, written with every
// string and integer style, survive Marshal and Unmarshal.
func FuzzStructRoundTrip(f *testing.F) {
	f.Add("plain", "key", int64(1), 1.5, true)
	f.Add("a\r\nb\"\"\"c", "a.b", int64(-31), math.Inf(-1), false)
	f.Add("it's '''\x01", "", int64(math.MinInt64), 1e300, true)
	f.Add("\x9a\\", "\"", int64(math.MaxInt64), math.NaN(), false)

	f.Fuzz(func(t *testing.T, s, key string, i int64, fl float64, b bool) {
		in := fuzzStruct{
			S: s, ML: s, MLL: s, L: s,
			I: i, H: i, O: int32(i), B: int16(i), U: uint32(i),
			F: fl, F32: float32(fl), T: b,
			A: []string{s, key},
			M: map[string]string{key: s},
		}
		in.N.X = s
		in.R = append(in.R, struct {
			Y int64 `toml:"y"`
		}{i})
		out, err := Marshal(in)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var got fuzzStruct
		if err := Unmarshal(out, &got); err != nil {
			t.Fatalf("Unmarshal: %v\n%s", err, out)
		}
		want := in
		valid := string([]rune(s))
		want.S, want.ML, want.MLL, want.L, want.N.X = valid, valid, valid, valid, valid
		want.A = []string{valid, string([]rune(key))}
		want.M = map[string]string{string([]rune(key)): valid}
		if math.IsNaN(fl) {
			if !math.IsNaN(got.F) || !math.IsNaN(float64(got.F32)) {
				t.Fatalf("NaN lost: %v, %v", got.F, got.F32)
			}
			got.F, got.F32, want.F, want.F32 = 0, 0, 0, 0
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round-trip mismatch\n got %+v\nwant %+v\n%s", got, want, out)
		}
	})
}
