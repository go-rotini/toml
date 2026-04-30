package toml

import (
	"errors"
	"math"
	"math/big"
	"testing"
	"time"
)

func TestDecodeAllTypes(t *testing.T) {
	input := `
str = "hello"
int_val = 42
float_val = 3.14
bool_val = true
date = 2024-01-15
time_val = 14:30:00
local_dt = 2024-01-15T14:30:00
offset_dt = 2024-01-15T14:30:00Z
arr = [1, 2, 3]
`
	type Config struct {
		Str      string        `toml:"str"`
		IntVal   int           `toml:"int_val"`
		FloatVal float64       `toml:"float_val"`
		BoolVal  bool          `toml:"bool_val"`
		Date     LocalDate     `toml:"date"`
		TimeVal  LocalTime     `toml:"time_val"`
		LocalDT  LocalDateTime `toml:"local_dt"`
		OffsetDT time.Time     `toml:"offset_dt"`
		Arr      []int         `toml:"arr"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Str != "hello" {
		t.Errorf("Str = %q", cfg.Str)
	}
	if cfg.IntVal != 42 {
		t.Errorf("IntVal = %d", cfg.IntVal)
	}
	if cfg.FloatVal != 3.14 {
		t.Errorf("FloatVal = %f", cfg.FloatVal)
	}
	if !cfg.BoolVal {
		t.Error("BoolVal = false")
	}
	if cfg.Date.Year != 2024 || cfg.Date.Month != time.January || cfg.Date.Day != 15 {
		t.Errorf("Date = %v", cfg.Date)
	}
	if cfg.TimeVal.Hour != 14 || cfg.TimeVal.Minute != 30 {
		t.Errorf("TimeVal = %v", cfg.TimeVal)
	}
	if cfg.LocalDT.Year != 2024 || cfg.LocalDT.Hour != 14 {
		t.Errorf("LocalDT = %v", cfg.LocalDT)
	}
	if cfg.OffsetDT.Year() != 2024 {
		t.Errorf("OffsetDT = %v", cfg.OffsetDT)
	}
	if len(cfg.Arr) != 3 || cfg.Arr[0] != 1 {
		t.Errorf("Arr = %v", cfg.Arr)
	}
}

func TestDecodeIntegerOverflow(t *testing.T) {
	input := `val = 256` + "\n"
	type Config struct {
		Val int8 `toml:"val"`
	}
	var cfg Config
	err := Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("expected overflow error")
	}
	var ovfErr *OverflowError
	if !errors.As(err, &ovfErr) {
		t.Fatalf("expected OverflowError, got %T: %v", err, err)
	}
}

func TestDecodeIntSizes(t *testing.T) {
	input := `
i8 = 127
i16 = 32767
i32 = 2147483647
i64 = 9223372036854775807
u8 = 255
u16 = 65535
u32 = 4294967295
`
	type Config struct {
		I8  int8   `toml:"i8"`
		I16 int16  `toml:"i16"`
		I32 int32  `toml:"i32"`
		I64 int64  `toml:"i64"`
		U8  uint8  `toml:"u8"`
		U16 uint16 `toml:"u16"`
		U32 uint32 `toml:"u32"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.I8 != 127 || cfg.I16 != 32767 || cfg.I32 != 2147483647 {
		t.Errorf("signed ints wrong: %+v", cfg)
	}
	if cfg.U8 != 255 || cfg.U16 != 65535 || cfg.U32 != 4294967295 {
		t.Errorf("unsigned ints wrong: %+v", cfg)
	}
}

func TestDecodeFloat(t *testing.T) {
	input := `
pi = 3.14159
neg = -1.5
pos_inf = inf
neg_inf = -inf
nan_val = nan
sci = 6.022e+23
`
	type Config struct {
		Pi     float64 `toml:"pi"`
		Neg    float64 `toml:"neg"`
		PosInf float64 `toml:"pos_inf"`
		NegInf float64 `toml:"neg_inf"`
		NanVal float64 `toml:"nan_val"`
		Sci    float64 `toml:"sci"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Pi != 3.14159 {
		t.Errorf("Pi = %f", cfg.Pi)
	}
	if !math.IsInf(cfg.PosInf, 1) {
		t.Errorf("PosInf = %f", cfg.PosInf)
	}
	if !math.IsInf(cfg.NegInf, -1) {
		t.Errorf("NegInf = %f", cfg.NegInf)
	}
	if !math.IsNaN(cfg.NanVal) {
		t.Errorf("NanVal = %f", cfg.NanVal)
	}
}

func TestDecodeBigInt(t *testing.T) {
	input := `val = 9223372036854775807` + "\n"
	type Config struct {
		Val *big.Int `toml:"val"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Val == nil {
		t.Fatal("Val is nil")
	}
	expected := new(big.Int).SetInt64(9223372036854775807)
	if cfg.Val.Cmp(expected) != 0 {
		t.Errorf("Val = %s, want %s", cfg.Val, expected)
	}
}

func TestDecodeBigFloat(t *testing.T) {
	input := `val = 3.14159265358979323846` + "\n"
	type Config struct {
		Val *big.Float `toml:"val"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Val == nil {
		t.Fatal("Val is nil")
	}
}

func TestDecodeDuration(t *testing.T) {
	input := `timeout = "5s"` + "\n"
	type Config struct {
		Timeout time.Duration `toml:"timeout"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestDecodeByteSlice(t *testing.T) {
	input := `data = "aGVsbG8="` + "\n"
	type Config struct {
		Data []byte `toml:"data"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(cfg.Data) != "hello" {
		t.Errorf("Data = %q, want %q", cfg.Data, "hello")
	}
}

func TestDecodePointers(t *testing.T) {
	input := `
name = "test"
value = 42
`
	type Config struct {
		Name  *string `toml:"name"`
		Value *int    `toml:"value"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Name == nil || *cfg.Name != "test" {
		t.Errorf("Name = %v", cfg.Name)
	}
	if cfg.Value == nil || *cfg.Value != 42 {
		t.Errorf("Value = %v", cfg.Value)
	}
}

func TestDecodeNestedTables(t *testing.T) {
	input := `
[a]
x = 1

[a.b]
y = 2

[a.b.c]
z = 3
`
	type C struct {
		Z int `toml:"z"`
	}
	type B struct {
		Y int `toml:"y"`
		C C   `toml:"c"`
	}
	type A struct {
		X int `toml:"x"`
		B B   `toml:"b"`
	}
	type Config struct {
		A A `toml:"a"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.A.X != 1 || cfg.A.B.Y != 2 || cfg.A.B.C.Z != 3 {
		t.Errorf("nested = %+v", cfg)
	}
}

func TestDecodeMapStringAny(t *testing.T) {
	input := `
str = "hello"
num = 42
flag = true
arr = [1, 2]
`
	var m map[string]any
	if err := Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["str"] != "hello" {
		t.Errorf("str = %v", m["str"])
	}
	if m["num"] != int64(42) {
		t.Errorf("num = %v (%T)", m["num"], m["num"])
	}
	if m["flag"] != true {
		t.Errorf("flag = %v", m["flag"])
	}
}

func TestDecodeOrderedMap(t *testing.T) {
	input := `
c = 3
a = 1
b = 2
`
	var m map[string]any
	if err := UnmarshalWithOptions([]byte(input), &m, WithOrderedMap()); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// WithOrderedMap produces MapSlice when decoding into any
	var result any
	if err := UnmarshalWithOptions([]byte(input), &result, WithOrderedMap()); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	ms, ok := result.(MapSlice)
	if !ok {
		t.Fatalf("expected MapSlice, got %T", result)
	}
	if len(ms) != 3 {
		t.Fatalf("len = %d, want 3", len(ms))
	}
	if ms[0].Key != "c" || ms[1].Key != "a" || ms[2].Key != "b" {
		t.Errorf("order wrong: %v", ms)
	}
}

func TestDecodeFixedArray(t *testing.T) {
	input := `val = [1, 2, 3]` + "\n"
	type Config struct {
		Val [3]int `toml:"val"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Val != [3]int{1, 2, 3} {
		t.Errorf("Val = %v", cfg.Val)
	}
}

func TestDecodeFixedArrayLengthMismatch(t *testing.T) {
	input := `val = [1, 2]` + "\n"
	type Config struct {
		Val [3]int `toml:"val"`
	}
	var cfg Config
	err := Unmarshal([]byte(input), &cfg)
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestDecodeMaxDepth(t *testing.T) {
	input := `
[a]
[a.b]
[a.b.c]
key = 1
`
	type D struct {
		Key int `toml:"key"`
	}
	type C struct {
		D D `toml:"c"`
	}
	type B struct {
		C C `toml:"b"`
	}
	type A struct {
		B B `toml:"a"`
	}
	var cfg A
	err := UnmarshalWithOptions([]byte(input), &cfg, WithMaxDepth(2))
	if err == nil {
		t.Fatal("expected depth error")
	}
}

func TestDecodeMaxKeys(t *testing.T) {
	input := `
a = 1
b = 2
c = 3
`
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxKeys(2))
	if err == nil {
		t.Fatal("expected max keys error")
	}
}

func TestDecodeMaxDocumentSize(t *testing.T) {
	input := `key = "value"` + "\n"
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxDocumentSize(5))
	if !errors.Is(err, ErrDocumentSize) {
		t.Fatalf("expected ErrDocumentSize, got %v", err)
	}
}

func TestDecodeNilPointer(t *testing.T) {
	err := Unmarshal([]byte(`key = "val"`), nil)
	if !errors.Is(err, ErrNilPointer) {
		t.Fatalf("expected ErrNilPointer, got %v", err)
	}
}

func TestDecodeValidator(t *testing.T) {
	input := `port = -1` + "\n"
	type Config struct {
		Port int `toml:"port"`
	}

	validator := &testValidator{fn: func(v any) error {
		cfg := v.(*Config)
		if cfg.Port < 0 {
			return errors.New("port must be positive")
		}
		return nil
	}}

	var cfg Config
	err := UnmarshalWithOptions([]byte(input), &cfg, WithValidator(validator))
	if err == nil {
		t.Fatal("expected validation error")
	}
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

type testValidator struct {
	fn func(v any) error
}

func (v *testValidator) Struct(val any) error {
	return v.fn(val)
}

func TestDecodeEmbeddedStruct(t *testing.T) {
	input := `
name = "test"
value = 42
`
	type Base struct {
		Name string `toml:"name"`
	}
	type Config struct {
		Base
		Value int `toml:"value"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Name != "test" || cfg.Value != 42 {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestDecodeJSONTagFallback(t *testing.T) {
	input := `name = "test"` + "\n"
	type Config struct {
		Name string `json:"name"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q", cfg.Name)
	}
}

func TestDecodeCaseInsensitive(t *testing.T) {
	input := `Name = "test"` + "\n"
	type Config struct {
		Name string `toml:"name"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q", cfg.Name)
	}
}

func TestDecodeMixedArray(t *testing.T) {
	input := `val = [1, "two", true]` + "\n"
	var m map[string]any
	if err := Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	arr, ok := m["val"].([]any)
	if !ok {
		t.Fatalf("val is %T", m["val"])
	}
	if len(arr) != 3 {
		t.Fatalf("len = %d", len(arr))
	}
	if arr[0] != int64(1) || arr[1] != "two" || arr[2] != true {
		t.Errorf("arr = %v", arr)
	}
}

func TestDecodeSpaceDateTime(t *testing.T) {
	input := `dt = 1979-05-27 07:32:00Z` + "\n"
	type Config struct {
		DT time.Time `toml:"dt"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.DT.Year() != 1979 || cfg.DT.Month() != time.May {
		t.Errorf("DT = %v", cfg.DT)
	}
}

func TestDecodeLocalDateToTime(t *testing.T) {
	input := `date = 2024-06-15` + "\n"
	type Config struct {
		Date time.Time `toml:"date"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Date.Year() != 2024 || cfg.Date.Month() != time.June || cfg.Date.Day() != 15 {
		t.Errorf("Date = %v", cfg.Date)
	}
}

func TestDecodeLocalDateTimeToTime(t *testing.T) {
	input := `dt = 2024-06-15T10:30:00` + "\n"
	type Config struct {
		DT time.Time `toml:"dt"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.DT.Year() != 2024 || cfg.DT.Hour() != 10 {
		t.Errorf("DT = %v", cfg.DT)
	}
}

func TestDecodeLocalTimeZone(t *testing.T) {
	input := `dt = 2024-06-15T10:30:00` + "\n"
	type Config struct {
		DT time.Time `toml:"dt"`
	}
	utc := time.UTC
	var cfg Config
	if err := UnmarshalWithOptions([]byte(input), &cfg, WithLocalTimeZone(utc)); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.DT.Location() != utc {
		t.Errorf("Location = %v, want UTC", cfg.DT.Location())
	}
}

func TestDecodeMaxNodes(t *testing.T) {
	input := `
a = 1
b = 2
c = 3
d = 4
e = 5
`
	var m map[string]any
	err := UnmarshalWithOptions([]byte(input), &m, WithMaxNodes(3))
	if err == nil {
		t.Fatal("expected max nodes error")
	}
}

func TestDecodeSkipField(t *testing.T) {
	input := `
name = "test"
ignored = "skip"
`
	type Config struct {
		Name    string `toml:"name"`
		Ignored string `toml:"-"`
	}
	var cfg Config
	if err := Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.Ignored != "" {
		t.Errorf("Ignored should be empty, got %q", cfg.Ignored)
	}
}
