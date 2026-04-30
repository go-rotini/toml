package toml

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	input := `
title = "TOML Example"

[owner]
name = "Tom Preston-Werner"
dob = 1979-05-27T07:32:00Z

[database]
enabled = true
ports = [8000, 8001, 8002]
temp_targets = { cpu = 79.5, case = 72.0 }
`

	type Database struct {
		Enabled     bool       `toml:"enabled"`
		Ports       []int      `toml:"ports"`
		TempTargets map[string]float64 `toml:"temp_targets"`
	}
	type Owner struct {
		Name string    `toml:"name"`
		DOB  time.Time `toml:"dob"`
	}
	type Config struct {
		Title    string   `toml:"title"`
		Owner    Owner    `toml:"owner"`
		Database Database `toml:"database"`
	}

	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Title != "TOML Example" {
		t.Errorf("Title = %q, want %q", cfg.Title, "TOML Example")
	}
	if cfg.Owner.Name != "Tom Preston-Werner" {
		t.Errorf("Owner.Name = %q, want %q", cfg.Owner.Name, "Tom Preston-Werner")
	}
	if !cfg.Database.Enabled {
		t.Error("Database.Enabled = false, want true")
	}
	if len(cfg.Database.Ports) != 3 {
		t.Errorf("len(Ports) = %d, want 3", len(cfg.Database.Ports))
	}
	if cfg.Database.TempTargets["cpu"] != 79.5 {
		t.Errorf("TempTargets[cpu] = %v, want 79.5", cfg.Database.TempTargets["cpu"])
	}

	out, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(out, []byte("title")) {
		t.Errorf("Marshal output missing 'title':\n%s", out)
	}
}

func TestUnmarshalTo(t *testing.T) {
	input := `name = "test"
value = 42
`
	type Simple struct {
		Name  string `toml:"name"`
		Value int    `toml:"value"`
	}

	result, err := UnmarshalTo[Simple]([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTo: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("Name = %q, want %q", result.Name, "test")
	}
	if result.Value != 42 {
		t.Errorf("Value = %d, want 42", result.Value)
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`key = "value"`, true},
		{`key = 42`, true},
		{`key = true`, true},
		{`[table]`, true},
		{`key = `, false},
		{`key = "unterminated`, false},
	}
	for _, tt := range tests {
		got := Valid([]byte(tt.input))
		if got != tt.want {
			t.Errorf("Valid(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLocalDateTypes(t *testing.T) {
	input := `
date = 1979-05-27
time = 07:32:00
datetime = 1979-05-27T07:32:00
`
	type Config struct {
		Date     LocalDate     `toml:"date"`
		Time     LocalTime     `toml:"time"`
		DateTime LocalDateTime `toml:"datetime"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Date.Year != 1979 || cfg.Date.Month != time.May || cfg.Date.Day != 27 {
		t.Errorf("Date = %v, want 1979-05-27", cfg.Date)
	}
	if cfg.Time.Hour != 7 || cfg.Time.Minute != 32 {
		t.Errorf("Time = %v, want 07:32:00", cfg.Time)
	}
	if cfg.DateTime.Year != 1979 || cfg.DateTime.Hour != 7 {
		t.Errorf("DateTime = %v, want 1979-05-27T07:32:00", cfg.DateTime)
	}
}

func TestArrayOfTables(t *testing.T) {
	input := `
[[products]]
name = "Hammer"
sku = 738594937

[[products]]
name = "Nail"
sku = 284758393
`
	type Product struct {
		Name string `toml:"name"`
		SKU  int    `toml:"sku"`
	}
	type Config struct {
		Products []Product `toml:"products"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(cfg.Products) != 2 {
		t.Fatalf("len(Products) = %d, want 2", len(cfg.Products))
	}
	if cfg.Products[0].Name != "Hammer" {
		t.Errorf("Products[0].Name = %q, want %q", cfg.Products[0].Name, "Hammer")
	}
	if cfg.Products[1].Name != "Nail" {
		t.Errorf("Products[1].Name = %q, want %q", cfg.Products[1].Name, "Nail")
	}
}

func TestIntegerFormats(t *testing.T) {
	input := `
dec = 42
hex = 0xDEAD
oct = 0o755
bin = 0b11010110
`
	type Config struct {
		Dec int `toml:"dec"`
		Hex int `toml:"hex"`
		Oct int `toml:"oct"`
		Bin int `toml:"bin"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Dec != 42 {
		t.Errorf("Dec = %d, want 42", cfg.Dec)
	}
	if cfg.Hex != 0xDEAD {
		t.Errorf("Hex = %d, want %d", cfg.Hex, 0xDEAD)
	}
	if cfg.Oct != 0o755 {
		t.Errorf("Oct = %d, want %d", cfg.Oct, 0o755)
	}
	if cfg.Bin != 0b11010110 {
		t.Errorf("Bin = %d, want %d", cfg.Bin, 0b11010110)
	}
}

func TestStringStyles(t *testing.T) {
	input := "basic = \"hello\\nworld\"\nliteral = 'hello\\nworld'\n"

	type Config struct {
		Basic   string `toml:"basic"`
		Literal string `toml:"literal"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Basic != "hello\nworld" {
		t.Errorf("Basic = %q, want %q", cfg.Basic, "hello\nworld")
	}
	if cfg.Literal != "hello\\nworld" {
		t.Errorf("Literal = %q, want %q", cfg.Literal, "hello\\nworld")
	}
}

func TestDuplicateKeyError(t *testing.T) {
	input := `
key = "value1"
key = "value2"
`
	var m map[string]any
	err := Unmarshal([]byte(input), &m)
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestStrictMode(t *testing.T) {
	input := `
known = "yes"
unknown = "no"
`
	type Config struct {
		Known string `toml:"known"`
	}
	var cfg Config
	err := UnmarshalWithOptions([]byte(input), &cfg, WithStrict())
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestEncoder(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(Config{Name: "test", Port: 8080}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `name = "test"`) {
		t.Errorf("output missing name:\n%s", output)
	}
	if !strings.Contains(output, "port = 8080") {
		t.Errorf("output missing port:\n%s", output)
	}
}

func TestDecoder(t *testing.T) {
	input := `name = "test"
port = 8080
`
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	var cfg Config
	dec := NewDecoder(strings.NewReader(input))
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestParse(t *testing.T) {
	input := `
[server]
host = "localhost"
port = 8080
`
	file, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if file.Root == nil {
		t.Fatal("Root is nil")
	}
}

func TestPathString(t *testing.T) {
	p, err := PathString("$.server.host")
	if err != nil {
		t.Fatalf("PathString: %v", err)
	}
	if p.String() != "$.server.host" {
		t.Errorf("String() = %q, want %q", p.String(), "$.server.host")
	}
}

func TestFormatError(t *testing.T) {
	data := []byte("key = value\nbad line\n")
	err := &SyntaxError{
		Message: "unexpected token",
		Pos:     Position{Line: 2, Column: 1},
	}
	formatted := FormatError(data, err)
	if !strings.Contains(formatted, "bad line") {
		t.Errorf("FormatError missing source line:\n%s", formatted)
	}
	if !strings.Contains(formatted, "^") {
		t.Errorf("FormatError missing pointer:\n%s", formatted)
	}
}

func TestCommentTag(t *testing.T) {
	type Config struct {
		Port int `toml:"port" comment:"Server port"`
	}
	out, err := Marshal(Config{Port: 8080})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "# Server port") {
		t.Errorf("output missing comment:\n%s", out)
	}
}

func TestHexOctalBinaryEncode(t *testing.T) {
	type Config struct {
		Hex int `toml:"hex,hex"`
		Oct int `toml:"oct,octal"`
		Bin int `toml:"bin,binary"`
	}
	out, err := Marshal(Config{Hex: 0xDEAD, Oct: 0o755, Bin: 0b1010})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "0xDEAD") {
		t.Errorf("output missing hex:\n%s", s)
	}
	if !strings.Contains(s, "0o755") {
		t.Errorf("output missing octal:\n%s", s)
	}
	if !strings.Contains(s, "0b1010") {
		t.Errorf("output missing binary:\n%s", s)
	}
}

func TestMapUnmarshal(t *testing.T) {
	input := `
key1 = "value1"
key2 = 42
key3 = true
`
	var m map[string]any
	if err := Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["key1"] != "value1" {
		t.Errorf("key1 = %v, want %q", m["key1"], "value1")
	}
	if m["key2"] != int64(42) {
		t.Errorf("key2 = %v (%T), want int64(42)", m["key2"], m["key2"])
	}
	if m["key3"] != true {
		t.Errorf("key3 = %v, want true", m["key3"])
	}
}

func TestUnicodeEscapes(t *testing.T) {
	input := `key = "hello A \U00000042 \xC3"` + "\n"
	type Config struct {
		Key string `toml:"key"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !strings.Contains(cfg.Key, "A") || !strings.Contains(cfg.Key, "B") {
		t.Errorf("Key = %q, expected Unicode characters A and B", cfg.Key)
	}
}

func TestEscapeSequence(t *testing.T) {
	input := `key = "hello\tworld\n\e"` + "\n"
	type Config struct {
		Key string `toml:"key"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Key != "hello\tworld\n\x1b" {
		t.Errorf("Key = %q, want %q", cfg.Key, "hello\tworld\n\x1b")
	}
}

func TestMultiLineStrings(t *testing.T) {
	input := "key = \"\"\"\nhello\nworld\n\"\"\"\n"
	type Config struct {
		Key string `toml:"key"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Key != "hello\nworld\n" {
		t.Errorf("Key = %q, want %q", cfg.Key, "hello\nworld\n")
	}
}

func TestRequiredTag(t *testing.T) {
	input := `name = "test"` + "\n"
	type Config struct {
		Name     string `toml:"name"`
		Required string `toml:"required,required"`
	}
	var cfg Config
	err := Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestDefaultTag(t *testing.T) {
	input := `name = "test"` + "\n"
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port,default=8080"`
	}
	var cfg Config
	if err := UnmarshalWithOptions([]byte(input), &cfg, WithDefaults()); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestToJSON(t *testing.T) {
	input := `name = "test"
value = 42
`
	jsonBytes, err := ToJSON([]byte(input))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	s := string(jsonBytes)
	if !strings.Contains(s, `"name"`) || !strings.Contains(s, `"test"`) {
		t.Errorf("ToJSON output unexpected:\n%s", s)
	}
}
