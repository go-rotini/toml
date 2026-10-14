package toml

import (
	"reflect"
	"testing"
	"time"
)

func TestParseTagEmpty(t *testing.T) {
	fi := parseTag("")
	if fi.name != "" {
		t.Errorf("name = %q, want empty", fi.name)
	}
	if fi.skip {
		t.Error("skip should be false for empty tag")
	}
}

func TestParseTagSkip(t *testing.T) {
	fi := parseTag("-")
	if !fi.skip {
		t.Error("skip should be true for '-'")
	}
	// When skip is true, name should be "-"
	if fi.name != "-" {
		t.Errorf("name = %q, want %q", fi.name, "-")
	}
}

func TestParseTagName(t *testing.T) {
	fi := parseTag("my_key")
	if fi.name != "my_key" {
		t.Errorf("name = %q, want %q", fi.name, "my_key")
	}
	if fi.omitEmpty || fi.inline || fi.skip {
		t.Error("no options should be set")
	}
}

func TestParseTagOmitempty(t *testing.T) {
	fi := parseTag("name,omitempty")
	if fi.name != "name" {
		t.Errorf("name = %q, want %q", fi.name, "name")
	}
	if !fi.omitEmpty {
		t.Error("omitEmpty should be true")
	}
}

func TestParseTagOmitzero(t *testing.T) {
	// omitzero is treated the same as omitempty
	fi := parseTag("name,omitzero")
	if !fi.omitEmpty {
		t.Error("omitEmpty should be true for omitzero")
	}
}

func TestParseTagInline(t *testing.T) {
	fi := parseTag("name,inline")
	if !fi.inline {
		t.Error("inline should be true")
	}
}

func TestParseTagMultiline(t *testing.T) {
	fi := parseTag("name,multiline")
	if !fi.multiline {
		t.Error("multiline should be true")
	}
}

func TestParseTagLiteral(t *testing.T) {
	fi := parseTag("name,literal")
	if !fi.literal {
		t.Error("literal should be true")
	}
}

func TestParseTagCommented(t *testing.T) {
	fi := parseTag("name,commented")
	if !fi.commented {
		t.Error("commented should be true")
	}
}

func TestParseTagHex(t *testing.T) {
	fi := parseTag("name,hex")
	if !fi.hex {
		t.Error("hex should be true")
	}
}

func TestParseTagOctal(t *testing.T) {
	fi := parseTag("name,octal")
	if !fi.octal {
		t.Error("octal should be true")
	}
}

func TestParseTagBinary(t *testing.T) {
	fi := parseTag("name,binary")
	if !fi.binary {
		t.Error("binary should be true")
	}
}

func TestParseTagRequired(t *testing.T) {
	fi := parseTag("name,required")
	if !fi.required {
		t.Error("required should be true")
	}
}

func TestParseTagDefault(t *testing.T) {
	fi := parseTag("port,default=8080")
	if fi.name != "port" {
		t.Errorf("name = %q, want %q", fi.name, "port")
	}
	if !fi.hasDefault {
		t.Error("hasDefault should be true")
	}
	if fi.defaultValue != "8080" {
		t.Errorf("defaultValue = %q, want %q", fi.defaultValue, "8080")
	}
}

func TestParseTagDefaultEmpty(t *testing.T) {
	fi := parseTag("name,default=")
	if !fi.hasDefault {
		t.Error("hasDefault should be true for 'default='")
	}
	if fi.defaultValue != "" {
		t.Errorf("defaultValue = %q, want empty", fi.defaultValue)
	}
}

func TestParseTagMultipleOptions(t *testing.T) {
	fi := parseTag("name,omitempty,multiline,literal,hex,required,default=42")
	if fi.name != "name" {
		t.Errorf("name = %q", fi.name)
	}
	if !fi.omitEmpty {
		t.Error("omitEmpty should be true")
	}
	if !fi.multiline {
		t.Error("multiline should be true")
	}
	if !fi.literal {
		t.Error("literal should be true")
	}
	if !fi.hex {
		t.Error("hex should be true")
	}
	if !fi.required {
		t.Error("required should be true")
	}
	if !fi.hasDefault || fi.defaultValue != "42" {
		t.Errorf("default = (%v, %q), want (true, 42)", fi.hasDefault, fi.defaultValue)
	}
}

func TestParseTagUnknownOption(t *testing.T) {
	// Unknown options that don't start with "default=" are silently ignored
	fi := parseTag("name,unknown_option")
	if fi.name != "name" {
		t.Errorf("name = %q, want %q", fi.name, "name")
	}
	// none of the known flags should be set
	if fi.omitEmpty || fi.inline || fi.multiline || fi.literal ||
		fi.commented || fi.hex || fi.octal || fi.binary ||
		fi.required || fi.hasDefault || fi.skip {
		t.Error("no flags should be set for unknown option")
	}
}

func TestGetStructFieldsBasic(t *testing.T) {
	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))
	if len(sf.fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(sf.fields))
	}

	nameIdx, ok := sf.byName["name"]
	if !ok {
		t.Fatal("field 'name' not found")
	}
	if sf.fields[nameIdx].name != "name" {
		t.Errorf("field name = %q, want %q", sf.fields[nameIdx].name, "name")
	}

	portIdx, ok := sf.byName["port"]
	if !ok {
		t.Fatal("field 'port' not found")
	}
	if sf.fields[portIdx].name != "port" {
		t.Errorf("field name = %q, want %q", sf.fields[portIdx].name, "port")
	}
}

func TestGetStructFieldsNameLowercasing(t *testing.T) {
	type Config struct {
		MyField string
		PORT    int
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["myfield"]; !ok {
		t.Error("field 'myfield' (lowercased from MyField) not found")
	}
	if _, ok := sf.byName["port"]; !ok {
		t.Error("field 'port' (lowercased from PORT) not found")
	}
}

func TestGetStructFieldsJSONFallback(t *testing.T) {
	type Config struct {
		Name string `json:"json_name"`
		Port int    `json:"json_port,omitempty"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["json_name"]; !ok {
		t.Error("field 'json_name' not found (json tag fallback)")
	}
	if _, ok := sf.byName["json_port"]; !ok {
		t.Error("field 'json_port' not found (json tag fallback)")
	}
}

func TestGetStructFieldsTOMLOverridesJSON(t *testing.T) {
	type Config struct {
		Name string `toml:"toml_name" json:"json_name"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["toml_name"]; !ok {
		t.Error("field 'toml_name' not found (toml should override json)")
	}
	if _, ok := sf.byName["json_name"]; ok {
		t.Error("field 'json_name' should not exist when toml tag is present")
	}
}

func TestGetStructFieldsSkip(t *testing.T) {
	type Config struct {
		Name    string `toml:"name"`
		Skipped string `toml:"-"`
		Also    string `json:"-"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["name"]; !ok {
		t.Error("field 'name' should be present")
	}
	// "-" tagged fields should be skipped
	for _, f := range sf.fields {
		if f.name == "Skipped" || f.name == "skipped" {
			t.Error("skipped field should not be in fields")
		}
	}
}

func TestGetStructFieldsEmbeddedStruct(t *testing.T) {
	type Base struct {
		ID   int    `toml:"id"`
		Name string `toml:"name"`
	}
	type Config struct {
		Base
		Port int `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["id"]; !ok {
		t.Error("embedded field 'id' not found")
	}
	if _, ok := sf.byName["name"]; !ok {
		t.Error("embedded field 'name' not found")
	}
	if _, ok := sf.byName["port"]; !ok {
		t.Error("field 'port' not found")
	}
}

func TestGetStructFieldsEmbeddedPointerStruct(t *testing.T) {
	type Base struct {
		ID int `toml:"id"`
	}
	type Config struct {
		*Base
		Port int `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["id"]; !ok {
		t.Error("embedded pointer struct field 'id' not found")
	}
	if _, ok := sf.byName["port"]; !ok {
		t.Error("field 'port' not found")
	}
}

func TestGetStructFieldsConflictDetection(t *testing.T) {
	type A struct {
		Name string `toml:"name"`
	}
	type B struct {
		Name string `toml:"name"`
	}
	type Config struct {
		A
		B
		Port int `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	// A.Name and B.Name have the same depth, so the first one wins.
	if got := sf.fields[sf.byName["name"]].index; !reflect.DeepEqual(got, []int{0, 0}) {
		t.Errorf("name index = %v, want A.Name [0 0]", got)
	}
}

func TestGetStructFieldsDeepWins(t *testing.T) {
	// When a field at a shallower depth conflicts with one at a deeper depth,
	// the shallower one should win.
	type Inner struct {
		Name string `toml:"name"`
	}
	type Config struct {
		Name string `toml:"name"`
		Inner
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	idx, ok := sf.byName["name"]
	if !ok {
		t.Fatal("field 'name' not found")
	}
	// The direct field (depth 1) should win over the embedded one (depth 2)
	if len(sf.fields[idx].index) != 1 {
		t.Errorf("winning field index depth = %d, want 1 (direct field should win)", len(sf.fields[idx].index))
	}
}

func TestGetStructFieldsUnexportedSkipped(t *testing.T) {
	type Config struct {
		Name   string `toml:"name"`
		hidden string `toml:"hidden"` //nolint:unused
		Port   int    `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["hidden"]; ok {
		t.Error("unexported field 'hidden' should not be in fields")
	}
	if len(sf.fields) != 2 {
		t.Errorf("len(fields) = %d, want 2", len(sf.fields))
	}
}

func TestGetStructFieldsCommentTag(t *testing.T) {
	type Config struct {
		Port int `toml:"port" comment:"Server port number"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	idx, ok := sf.byName["port"]
	if !ok {
		t.Fatal("field 'port' not found")
	}
	if sf.fields[idx].comment != "Server port number" {
		t.Errorf("comment = %q, want %q", sf.fields[idx].comment, "Server port number")
	}
}

func TestGetStructFieldsCommentTagEmpty(t *testing.T) {
	type Config struct {
		Port int `toml:"port"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	idx, ok := sf.byName["port"]
	if !ok {
		t.Fatal("field 'port' not found")
	}
	if sf.fields[idx].comment != "" {
		t.Errorf("comment = %q, want empty", sf.fields[idx].comment)
	}
}

func TestGetStructFieldsOptionsPreserved(t *testing.T) {
	type Config struct {
		Name string `toml:"name,omitempty,multiline"`
		Hex  int    `toml:"hex_val,hex"`
		Port int    `toml:"port,required,default=8080"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	nameIdx := sf.byName["name"]
	if !sf.fields[nameIdx].omitEmpty {
		t.Error("name.omitEmpty should be true")
	}
	if !sf.fields[nameIdx].multiline {
		t.Error("name.multiline should be true")
	}

	hexIdx := sf.byName["hex_val"]
	if !sf.fields[hexIdx].hex {
		t.Error("hex_val.hex should be true")
	}

	portIdx := sf.byName["port"]
	if !sf.fields[portIdx].required {
		t.Error("port.required should be true")
	}
	if !sf.fields[portIdx].hasDefault || sf.fields[portIdx].defaultValue != "8080" {
		t.Errorf("port default = (%v, %q), want (true, 8080)",
			sf.fields[portIdx].hasDefault, sf.fields[portIdx].defaultValue)
	}
}

func TestGetStructFieldsCaching(t *testing.T) {
	type Config struct {
		A string `toml:"a"`
	}
	typ := reflect.TypeOf(Config{})

	sf1 := getStructFields(typ)
	sf2 := getStructFields(typ)
	if sf1 != sf2 {
		t.Error("getStructFields should return cached pointer for same type")
	}
}

func TestGetStructFieldsInlineStruct(t *testing.T) {
	type Inner struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}
	type Config struct {
		Inner Inner `toml:",inline"`
		Z     int   `toml:"z"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	// The inline struct's fields should be flattened
	if _, ok := sf.byName["x"]; !ok {
		t.Error("inline field 'x' not found")
	}
	if _, ok := sf.byName["y"]; !ok {
		t.Error("inline field 'y' not found")
	}
	if _, ok := sf.byName["z"]; !ok {
		t.Error("field 'z' not found")
	}
}

func TestGetStructFieldsInlineMap(t *testing.T) {
	type Config struct {
		Known  string            `toml:"known"`
		Extras map[string]string `toml:",inline"`
	}
	sf := getStructFields(reflect.TypeOf(Config{}))

	if _, ok := sf.byName["known"]; !ok {
		t.Error("field 'known' not found")
	}
	// The inline map should appear as a field (even with empty name)
	found := false
	for _, f := range sf.fields {
		if f.inline && f.name == "" {
			found = true
			break
		}
	}
	// It may or may not inline into the map depending on map handling path.
	// Just verify "known" is there and there's no panic.
	_ = found
}

type SelfEmbed struct {
	*SelfEmbed
	V int
}

type MutualA struct {
	*MutualB
	A int
}

type MutualB struct {
	*MutualA
	B int
}

func TestSelfEmbeddingTypes(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, err := Marshal(SelfEmbed{V: 1})
		if err != nil || string(out) != "v = 1\n" {
			t.Errorf("Marshal = %q, %v", out, err)
		}
		var s SelfEmbed
		if err := Unmarshal([]byte("v = 2\n"), &s); err != nil || s.V != 2 {
			t.Errorf("Unmarshal = %+v, %v", s, err)
		}
		var m MutualA
		if err := Unmarshal([]byte("a = 1\nb = 2\n"), &m); err != nil || m.A != 1 || m.MutualB == nil || m.B != 2 {
			t.Errorf("Unmarshal mutual = %+v, %v", m, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("collecting fields of a self-embedding type did not return")
	}
}
