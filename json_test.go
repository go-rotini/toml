package toml

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ToJSON
// ---------------------------------------------------------------------------

func TestJSONToJSONBasic(t *testing.T) {
	input := `name = "Alice"
age = 30
active = true
`
	got, err := ToJSON([]byte(input))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `"name"`) || !strings.Contains(s, `"Alice"`) {
		t.Errorf("missing name: %s", s)
	}
	if !strings.Contains(s, `"age"`) || !strings.Contains(s, `30`) {
		t.Errorf("missing age: %s", s)
	}
	if !strings.Contains(s, `"active"`) || !strings.Contains(s, `true`) {
		t.Errorf("missing active: %s", s)
	}
}

func TestJSONToJSONNestedTables(t *testing.T) {
	input := `
[server]
host = "localhost"
port = 8080

[server.tls]
enabled = true
`
	got, err := ToJSON([]byte(input))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	// Parse the JSON to verify structure.
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	srv, ok := m["server"].(map[string]any)
	if !ok {
		t.Fatalf("server not a map: %T", m["server"])
	}
	if srv["host"] != "localhost" {
		t.Errorf("host = %v", srv["host"])
	}
	tls, ok := srv["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls not a map: %T", srv["tls"])
	}
	if tls["enabled"] != true {
		t.Errorf("enabled = %v", tls["enabled"])
	}
}

func TestJSONToJSONArrays(t *testing.T) {
	input := `ports = [80, 443, 8080]
`
	got, err := ToJSON([]byte(input))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	arr, ok := m["ports"].([]any)
	if !ok {
		t.Fatalf("ports not array: %T", m["ports"])
	}
	if len(arr) != 3 {
		t.Errorf("len(ports) = %d, want 3", len(arr))
	}
}

func TestJSONToJSONAllValueTypes(t *testing.T) {
	input := `
str = "hello"
integer = 42
float = 3.14
flag = true
`
	got, err := ToJSON([]byte(input))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	s := string(got)
	for _, want := range []string{`"hello"`, `42`, `3.14`, `true`} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %s: %s", want, s)
		}
	}
}

func TestJSONToJSONInvalidTOML(t *testing.T) {
	_, err := ToJSON([]byte(`key = `))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

// ---------------------------------------------------------------------------
// FromJSON
// ---------------------------------------------------------------------------

func TestJSONFromJSONBasic(t *testing.T) {
	input := `{"name":"Bob","count":7}`
	got, err := FromJSON([]byte(input))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `name = "Bob"`) {
		t.Errorf("missing name: %s", s)
	}
	if !strings.Contains(s, "count = 7") {
		t.Errorf("missing count: %s", s)
	}
}

func TestJSONFromJSONNestedObjects(t *testing.T) {
	input := `{"server":{"host":"localhost","port":8080}}`
	got, err := FromJSON([]byte(input))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "host") || !strings.Contains(s, "localhost") {
		t.Errorf("missing host: %s", s)
	}
	if !strings.Contains(s, "port") || !strings.Contains(s, "8080") {
		t.Errorf("missing port: %s", s)
	}
}

func TestJSONFromJSONArrays(t *testing.T) {
	input := `{"tags":["a","b","c"]}`
	got, err := FromJSON([]byte(input))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "tags") {
		t.Errorf("missing tags: %s", s)
	}
}

func TestJSONFromJSONNumberHandling(t *testing.T) {
	// json.Number integers should become int64, floats should become float64.
	input := `{"int_val":42,"float_val":3.14,"sci_val":1e2}`
	got, err := FromJSON([]byte(input))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	// Verify by round-tripping back through Unmarshal.
	var m map[string]any
	if err := Unmarshal(got, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["int_val"] != int64(42) {
		t.Errorf("int_val = %v (%T)", m["int_val"], m["int_val"])
	}
	if m["float_val"] != 3.14 {
		t.Errorf("float_val = %v (%T)", m["float_val"], m["float_val"])
	}
	// sci_val has 'e' so it should be a float.
	if _, ok := m["sci_val"].(float64); !ok {
		t.Errorf("sci_val = %v (%T), want float64", m["sci_val"], m["sci_val"])
	}
}

func TestJSONFromJSONInvalidJSON(t *testing.T) {
	_, err := FromJSON([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONFromJSONBoolAndNull(t *testing.T) {
	input := `{"flag":true,"empty":null}`
	got, err := FromJSON([]byte(input))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "flag = true") {
		t.Errorf("missing flag: %s", s)
	}
}

// ---------------------------------------------------------------------------
// convertJSONValues
// ---------------------------------------------------------------------------

func TestJSONConvertValuesMap(t *testing.T) {
	m := map[string]any{
		"a": json.Number("10"),
		"b": json.Number("2.5"),
	}
	result := convertJSONValues(m)
	rm := result.(map[string]any)
	if rm["a"] != int64(10) {
		t.Errorf("a = %v (%T)", rm["a"], rm["a"])
	}
	if rm["b"] != 2.5 {
		t.Errorf("b = %v (%T)", rm["b"], rm["b"])
	}
}

func TestJSONConvertValuesSlice(t *testing.T) {
	s := []any{json.Number("1"), json.Number("2.0"), "hello"}
	result := convertJSONValues(s)
	rs := result.([]any)
	if rs[0] != int64(1) {
		t.Errorf("[0] = %v (%T)", rs[0], rs[0])
	}
	if rs[1] != 2.0 {
		t.Errorf("[1] = %v (%T)", rs[1], rs[1])
	}
	if rs[2] != "hello" {
		t.Errorf("[2] = %v (%T)", rs[2], rs[2])
	}
}

func TestJSONConvertValuesJsonNumberInt(t *testing.T) {
	v := convertJSONValues(json.Number("42"))
	if v != int64(42) {
		t.Errorf("got %v (%T), want int64(42)", v, v)
	}
}

func TestJSONConvertValuesJsonNumberFloat(t *testing.T) {
	v := convertJSONValues(json.Number("3.14"))
	if v != 3.14 {
		t.Errorf("got %v (%T), want 3.14", v, v)
	}
}

func TestJSONConvertValuesJsonNumberExponent(t *testing.T) {
	v := convertJSONValues(json.Number("1E3"))
	if v != float64(1000) {
		t.Errorf("got %v (%T), want 1000.0", v, v)
	}
}

func TestJSONConvertValuesJsonNumberStringFallback(t *testing.T) {
	// A number that can't parse as int or float falls back to string.
	v := convertJSONValues(json.Number("not_a_number"))
	if _, ok := v.(string); !ok {
		t.Errorf("got %v (%T), want string", v, v)
	}
}

func TestJSONConvertValuesPassthrough(t *testing.T) {
	// Non-map, non-slice, non-json.Number values pass through unchanged.
	if v := convertJSONValues(true); v != true {
		t.Errorf("bool: %v", v)
	}
	if v := convertJSONValues("str"); v != "str" {
		t.Errorf("string: %v", v)
	}
	if v := convertJSONValues(nil); v != nil {
		t.Errorf("nil: %v", v)
	}
}

// ---------------------------------------------------------------------------
// marshalJSONValue
// ---------------------------------------------------------------------------

func TestJSONMarshalValueNil(t *testing.T) {
	if got := marshalJSONValue(nil); got != `""` {
		t.Errorf("nil: got %q", got)
	}
}

func TestJSONMarshalValueString(t *testing.T) {
	if got := marshalJSONValue("hello"); got != `"hello"` {
		t.Errorf("string: got %q", got)
	}
}

func TestJSONMarshalValueStringWithQuotes(t *testing.T) {
	got := marshalJSONValue(`say "hi"`)
	if !strings.Contains(got, `\"hi\"`) {
		t.Errorf("expected escaped quotes: got %q", got)
	}
}

func TestJSONMarshalValueBoolTrue(t *testing.T) {
	if got := marshalJSONValue(true); got != "true" {
		t.Errorf("true: got %q", got)
	}
}

func TestJSONMarshalValueBoolFalse(t *testing.T) {
	if got := marshalJSONValue(false); got != "false" {
		t.Errorf("false: got %q", got)
	}
}

func TestJSONMarshalValueInt64(t *testing.T) {
	if got := marshalJSONValue(int64(99)); got != "99" {
		t.Errorf("int64: got %q", got)
	}
}

func TestJSONMarshalValueInt64Negative(t *testing.T) {
	if got := marshalJSONValue(int64(-42)); got != "-42" {
		t.Errorf("int64 neg: got %q", got)
	}
}

func TestJSONMarshalValueFloat64(t *testing.T) {
	got := marshalJSONValue(float64(2.718))
	if got != "2.718" {
		t.Errorf("float64: got %q", got)
	}
}

func TestJSONMarshalValueFloat64Inf(t *testing.T) {
	if got := marshalJSONValue(math.Inf(1)); got != `"inf"` {
		t.Errorf("+inf: got %q", got)
	}
	if got := marshalJSONValue(math.Inf(-1)); got != `"-inf"` {
		t.Errorf("-inf: got %q", got)
	}
}

func TestJSONMarshalValueFloat64NaN(t *testing.T) {
	if got := marshalJSONValue(math.NaN()); got != `"nan"` {
		t.Errorf("nan: got %q", got)
	}
}

func TestJSONMarshalValueTime(t *testing.T) {
	tm := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	got := marshalJSONValue(tm)
	if !strings.Contains(got, "2024") || !strings.Contains(got, "06-15") {
		t.Errorf("time: got %q", got)
	}
}

func TestJSONMarshalValueLocalDate(t *testing.T) {
	d := LocalDate{Year: 2024, Month: time.March, Day: 5}
	got := marshalJSONValue(d)
	if got != `"2024-03-05"` {
		t.Errorf("LocalDate: got %q", got)
	}
}

func TestJSONMarshalValueLocalTime(t *testing.T) {
	lt := LocalTime{Hour: 14, Minute: 30, Second: 0}
	got := marshalJSONValue(lt)
	if got != `"14:30:00"` {
		t.Errorf("LocalTime: got %q", got)
	}
}

func TestJSONMarshalValueLocalDateTime(t *testing.T) {
	dt := LocalDateTime{
		LocalDate: LocalDate{Year: 2024, Month: time.January, Day: 1},
		LocalTime: LocalTime{Hour: 12, Minute: 0, Second: 0},
	}
	got := marshalJSONValue(dt)
	if got != `"2024-01-01T12:00:00"` {
		t.Errorf("LocalDateTime: got %q", got)
	}
}

func TestJSONMarshalValueFallback(t *testing.T) {
	// A struct that json.Marshal can handle.
	type inner struct {
		X int `json:"x"`
	}
	got := marshalJSONValue(inner{X: 5})
	if got != `{"x":5}` {
		t.Errorf("fallback struct: got %q", got)
	}
}

func TestJSONMarshalValueFallbackSlice(t *testing.T) {
	got := marshalJSONValue([]int{1, 2, 3})
	if got != `[1,2,3]` {
		t.Errorf("fallback slice: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Round-trip: TOML -> JSON -> TOML
// ---------------------------------------------------------------------------

func TestJSONRoundTripTOMLToJSONToTOML(t *testing.T) {
	original := `name = "roundtrip"
count = 10
enabled = true
`
	jsonBytes, err := ToJSON([]byte(original))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	tomlBytes, err := FromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	// Verify the round-tripped TOML parses to the same values.
	var m map[string]any
	if err := Unmarshal(tomlBytes, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["name"] != "roundtrip" {
		t.Errorf("name = %v", m["name"])
	}
	if m["count"] != int64(10) {
		t.Errorf("count = %v (%T)", m["count"], m["count"])
	}
	if m["enabled"] != true {
		t.Errorf("enabled = %v", m["enabled"])
	}
}

func TestJSONRoundTripNestedTables(t *testing.T) {
	original := `
[database]
host = "db.example.com"
port = 5432
`
	jsonBytes, err := ToJSON([]byte(original))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	tomlBytes, err := FromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	type DB struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Database DB `toml:"database"`
	}
	var cfg Config
	if err := Unmarshal(tomlBytes, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("host = %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("port = %d", cfg.Database.Port)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestJSONToJSONEmptyDocument(t *testing.T) {
	got, err := ToJSON([]byte(""))
	if err != nil {
		t.Fatalf("ToJSON empty: %v", err)
	}
	// Should produce a valid JSON object.
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
}

func TestJSONFromJSONEmptyObject(t *testing.T) {
	got, err := FromJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("FromJSON empty: %v", err)
	}
	// Should produce valid (possibly empty) TOML.
	var m map[string]any
	if err := Unmarshal(got, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

func TestJSONConvertValuesNestedMap(t *testing.T) {
	m := map[string]any{
		"outer": map[string]any{
			"inner": json.Number("99"),
		},
	}
	result := convertJSONValues(m).(map[string]any)
	outer := result["outer"].(map[string]any)
	if outer["inner"] != int64(99) {
		t.Errorf("nested inner = %v (%T)", outer["inner"], outer["inner"])
	}
}

func TestJSONConvertValuesNestedSlice(t *testing.T) {
	s := []any{
		[]any{json.Number("1"), json.Number("2")},
	}
	result := convertJSONValues(s).([]any)
	inner := result[0].([]any)
	if inner[0] != int64(1) || inner[1] != int64(2) {
		t.Errorf("nested slice = %v", inner)
	}
}
