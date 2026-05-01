package toml

import (
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

		if !reflect.DeepEqual(m1, m2) {
			t.Fatalf("round-trip mismatch\noriginal:     %#v\nre-marshaled: %#v\nencoded TOML:\n%s", m1, m2, encoded)
		}
	})
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
