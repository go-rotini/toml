package toml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTOML11UnicodeBareKeysRejected(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "toml11", "unicode_bare_keys.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := Unmarshal(data, &m); err == nil {
		t.Fatal("expected error for Unicode bare keys, got nil")
	}
}

func TestTOML11EscapeSequences(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "toml11", "escape_sequences.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	checks := map[string]string{
		"escape_e":   "\x1b",
		"hex_tab":    "\t",
		"hex_newline": "\n",
		"hex_max":    "\x7f",
		"combined":   "hello\x1b\tworld",
	}
	for k, want := range checks {
		got, ok := m[k].(string)
		if !ok {
			t.Errorf("key %q not found or not string", k)
			continue
		}
		if got != want {
			t.Errorf("m[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestTOML11InlineTableNewlines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "toml11", "inline_table_newlines.toml"))
	if err != nil {
		t.Fatal(err)
	}
	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}
	type Inner struct {
		A int `toml:"a"`
		B int `toml:"b"`
	}
	type Nested struct {
		Name  string `toml:"name"`
		Inner Inner  `toml:"inner"`
	}
	type Config struct {
		Point         Point  `toml:"point"`
		Nested        Nested `toml:"nested"`
		TrailingComma Point  `toml:"trailing_comma"`
	}
	var cfg Config
	if err := Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Point.X != 1 || cfg.Point.Y != 2 {
		t.Errorf("point = %+v, want {X:1 Y:2}", cfg.Point)
	}
	if cfg.Nested.Name != "test" {
		t.Errorf("nested.name = %q, want %q", cfg.Nested.Name, "test")
	}
	if cfg.Nested.Inner.A != 1 || cfg.Nested.Inner.B != 2 {
		t.Errorf("nested.inner = %+v, want {A:1 B:2}", cfg.Nested.Inner)
	}
	if cfg.TrailingComma.X != 1 || cfg.TrailingComma.Y != 2 {
		t.Errorf("trailing_comma = %+v, want {X:1 Y:2}", cfg.TrailingComma)
	}
}

func TestTOML11LeadingZeros(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "toml11", "leading_zeros.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	floatChecks := map[string]float64{
		"exp_leading_zero":          1e06,
		"exp_negative_leading_zero": 1e-03,
		"exp_positive_leading_zero": 1e+09,
	}
	for k, want := range floatChecks {
		got, ok := m[k].(float64)
		if !ok {
			t.Errorf("key %q not found or not float64", k)
			continue
		}
		if got != want {
			t.Errorf("m[%q] = %v, want %v", k, got, want)
		}
	}

	intChecks := map[string]int64{
		"hex_leading": 0x00ff,
		"oct_leading": 0o0777,
		"bin_leading": 0b00001111,
	}
	for k, want := range intChecks {
		got, ok := m[k].(int64)
		if !ok {
			t.Errorf("key %q not found or not int64", k)
			continue
		}
		if got != want {
			t.Errorf("m[%q] = %v, want %v", k, got, want)
		}
	}
}

func TestTOML11DecimalLeadingZeroRejected(t *testing.T) {
	inputs := []string{
		"val = 0123",
		"val = 00",
		"val = 01",
		"val = -0123",
		"val = +0123",
	}
	for _, input := range inputs {
		var m map[string]any
		err := Unmarshal([]byte(input), &m)
		if err == nil {
			t.Errorf("expected error for %q, got nil", input)
		}
	}
}
