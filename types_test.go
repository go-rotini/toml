package toml

import (
	"testing"
	"time"
)

func TestLocalDateString(t *testing.T) {
	tests := []struct {
		date LocalDate
		want string
	}{
		{LocalDate{Year: 1979, Month: time.May, Day: 27}, "1979-05-27"},
		{LocalDate{Year: 2000, Month: time.January, Day: 1}, "2000-01-01"},
		{LocalDate{Year: 2023, Month: time.December, Day: 31}, "2023-12-31"},
		{LocalDate{Year: 0, Month: time.January, Day: 1}, "0000-01-01"},
	}
	for _, tt := range tests {
		got := tt.date.String()
		if got != tt.want {
			t.Errorf("LocalDate%+v.String() = %q, want %q", tt.date, got, tt.want)
		}
	}
}

func TestLocalDateMarshalText(t *testing.T) {
	d := LocalDate{Year: 1979, Month: time.May, Day: 27}
	data, err := d.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	if string(data) != "1979-05-27" {
		t.Errorf("MarshalText = %q, want %q", data, "1979-05-27")
	}
}

func TestLocalDateUnmarshalText(t *testing.T) {
	var d LocalDate
	if err := d.UnmarshalText([]byte("1979-05-27")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if d.Year != 1979 || d.Month != time.May || d.Day != 27 {
		t.Errorf("got %+v, want 1979-May-27", d)
	}
}

func TestLocalDateUnmarshalTextInvalid(t *testing.T) {
	tests := []string{
		"not-a-date",
		"2023/01/01",
		"",
		"2023-13-01",
		"2023-01-32",
		"hello",
	}
	for _, input := range tests {
		var d LocalDate
		err := d.UnmarshalText([]byte(input))
		if err == nil {
			t.Errorf("UnmarshalText(%q) should have failed", input)
		}
	}
}

func TestLocalDateAsTime(t *testing.T) {
	d := LocalDate{Year: 2023, Month: time.March, Day: 15}
	got := d.AsTime(time.UTC)
	want := time.Date(2023, time.March, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("AsTime(UTC) = %v, want %v", got, want)
	}

	// Test with a different location
	loc := time.FixedZone("EST", -5*60*60)
	got = d.AsTime(loc)
	want = time.Date(2023, time.March, 15, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("AsTime(EST) = %v, want %v", got, want)
	}
}

func TestLocalDateRoundTrip(t *testing.T) {
	original := LocalDate{Year: 2023, Month: time.June, Day: 15}
	data, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var decoded LocalDate
	if err := decoded.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip: got %+v, want %+v", decoded, original)
	}
}

func TestLocalTimeString(t *testing.T) {
	tests := []struct {
		time LocalTime
		want string
	}{
		{LocalTime{Hour: 7, Minute: 32, Second: 0}, "07:32:00"},
		{LocalTime{Hour: 0, Minute: 0, Second: 0}, "00:00:00"},
		{LocalTime{Hour: 23, Minute: 59, Second: 59}, "23:59:59"},
		{LocalTime{Hour: 12, Minute: 30, Second: 45, Nanosecond: 123000000}, "12:30:45.123"},
		{LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 1}, "12:00:00.000000001"},
		{LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 100000000}, "12:00:00.1"},
		{LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 123456789}, "12:00:00.123456789"},
	}
	for _, tt := range tests {
		got := tt.time.String()
		if got != tt.want {
			t.Errorf("LocalTime%+v.String() = %q, want %q", tt.time, got, tt.want)
		}
	}
}

func TestLocalTimeNoNanoseconds(t *testing.T) {
	lt := LocalTime{Hour: 14, Minute: 30, Second: 0, Nanosecond: 0}
	got := lt.String()
	if got != "14:30:00" {
		t.Errorf("String() = %q, want %q", got, "14:30:00")
	}
	// Should not contain a dot
	for _, c := range got {
		if c == '.' {
			t.Error("String() without nanoseconds should not contain '.'")
		}
	}
}

func TestLocalTimeMarshalText(t *testing.T) {
	lt := LocalTime{Hour: 15, Minute: 4, Second: 5, Nanosecond: 500000000}
	data, err := lt.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	if string(data) != "15:04:05.5" {
		t.Errorf("MarshalText = %q, want %q", data, "15:04:05.5")
	}
}

func TestLocalTimeUnmarshalText(t *testing.T) {
	tests := []struct {
		input string
		want  LocalTime
	}{
		{"07:32:00", LocalTime{Hour: 7, Minute: 32, Second: 0}},
		{"23:59:59", LocalTime{Hour: 23, Minute: 59, Second: 59}},
		{"12:30:45.123", LocalTime{Hour: 12, Minute: 30, Second: 45, Nanosecond: 123000000}},
		{"00:00:00.000000001", LocalTime{Hour: 0, Minute: 0, Second: 0, Nanosecond: 1}},
	}
	for _, tt := range tests {
		var lt LocalTime
		if err := lt.UnmarshalText([]byte(tt.input)); err != nil {
			t.Errorf("UnmarshalText(%q): %v", tt.input, err)
			continue
		}
		if lt != tt.want {
			t.Errorf("UnmarshalText(%q) = %+v, want %+v", tt.input, lt, tt.want)
		}
	}
}

func TestLocalTimeUnmarshalTextInvalid(t *testing.T) {
	tests := []string{
		"not-a-time",
		"25:00:00",
		"",
		"12:60:00",
		"abc",
	}
	for _, input := range tests {
		var lt LocalTime
		err := lt.UnmarshalText([]byte(input))
		if err == nil {
			t.Errorf("UnmarshalText(%q) should have failed", input)
		}
	}
}

func TestLocalTimeRoundTrip(t *testing.T) {
	original := LocalTime{Hour: 14, Minute: 30, Second: 45, Nanosecond: 123456789}
	data, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var decoded LocalTime
	if err := decoded.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip: got %+v, want %+v", decoded, original)
	}
}

func TestLocalDateTimeString(t *testing.T) {
	dt := LocalDateTime{
		LocalDate: LocalDate{Year: 1979, Month: time.May, Day: 27},
		LocalTime: LocalTime{Hour: 7, Minute: 32, Second: 0},
	}
	want := "1979-05-27T07:32:00"
	if got := dt.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLocalDateTimeStringWithNanos(t *testing.T) {
	dt := LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.January, Day: 1},
		LocalTime: LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 500000000},
	}
	want := "2023-01-01T12:00:00.5"
	if got := dt.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLocalDateTimeMarshalText(t *testing.T) {
	dt := LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.March, Day: 15},
		LocalTime: LocalTime{Hour: 10, Minute: 30, Second: 0},
	}
	data, err := dt.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	if string(data) != "2023-03-15T10:30:00" {
		t.Errorf("MarshalText = %q, want %q", data, "2023-03-15T10:30:00")
	}
}

func TestLocalDateTimeUnmarshalText(t *testing.T) {
	tests := []struct {
		input string
		want  LocalDateTime
	}{
		{
			"1979-05-27T07:32:00",
			LocalDateTime{
				LocalDate: LocalDate{Year: 1979, Month: time.May, Day: 27},
				LocalTime: LocalTime{Hour: 7, Minute: 32, Second: 0},
			},
		},
		{
			"2023-01-01 12:00:00",
			LocalDateTime{
				LocalDate: LocalDate{Year: 2023, Month: time.January, Day: 1},
				LocalTime: LocalTime{Hour: 12, Minute: 0, Second: 0},
			},
		},
		{
			"2023-06-15T10:30:45.123",
			LocalDateTime{
				LocalDate: LocalDate{Year: 2023, Month: time.June, Day: 15},
				LocalTime: LocalTime{Hour: 10, Minute: 30, Second: 45, Nanosecond: 123000000},
			},
		},
		{
			"2023-06-15 10:30:45.123",
			LocalDateTime{
				LocalDate: LocalDate{Year: 2023, Month: time.June, Day: 15},
				LocalTime: LocalTime{Hour: 10, Minute: 30, Second: 45, Nanosecond: 123000000},
			},
		},
	}
	for _, tt := range tests {
		var dt LocalDateTime
		if err := dt.UnmarshalText([]byte(tt.input)); err != nil {
			t.Errorf("UnmarshalText(%q): %v", tt.input, err)
			continue
		}
		if dt != tt.want {
			t.Errorf("UnmarshalText(%q) = %+v, want %+v", tt.input, dt, tt.want)
		}
	}
}

func TestLocalDateTimeUnmarshalTextInvalid(t *testing.T) {
	tests := []string{
		"not-a-datetime",
		"",
		"1979-05-27",
		"07:32:00",
		"abc123",
	}
	for _, input := range tests {
		var dt LocalDateTime
		err := dt.UnmarshalText([]byte(input))
		if err == nil {
			t.Errorf("UnmarshalText(%q) should have failed", input)
		}
	}
}

func TestLocalDateTimeAsTime(t *testing.T) {
	dt := LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.March, Day: 15},
		LocalTime: LocalTime{Hour: 10, Minute: 30, Second: 45, Nanosecond: 123456789},
	}
	got := dt.AsTime(time.UTC)
	want := time.Date(2023, time.March, 15, 10, 30, 45, 123456789, time.UTC)
	if !got.Equal(want) {
		t.Errorf("AsTime(UTC) = %v, want %v", got, want)
	}
}

func TestLocalDateTimeRoundTrip(t *testing.T) {
	original := LocalDateTime{
		LocalDate: LocalDate{Year: 2023, Month: time.June, Day: 15},
		LocalTime: LocalTime{Hour: 14, Minute: 30, Second: 45, Nanosecond: 123000000},
	}
	data, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var decoded LocalDateTime
	if err := decoded.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip: got %+v, want %+v", decoded, original)
	}
}

func TestMapSliceAndMapItem(t *testing.T) {
	ms := MapSlice{
		{Key: "alpha", Value: 1},
		{Key: "beta", Value: "two"},
		{Key: "gamma", Value: true},
	}

	if len(ms) != 3 {
		t.Fatalf("len(MapSlice) = %d, want 3", len(ms))
	}

	if ms[0].Key != "alpha" || ms[0].Value != 1 {
		t.Errorf("ms[0] = %+v, want {alpha, 1}", ms[0])
	}
	if ms[1].Key != "beta" || ms[1].Value != "two" {
		t.Errorf("ms[1] = %+v, want {beta, two}", ms[1])
	}
	if ms[2].Key != "gamma" || ms[2].Value != true {
		t.Errorf("ms[2] = %+v, want {gamma, true}", ms[2])
	}
}

func TestMapSliceEmpty(t *testing.T) {
	var ms MapSlice
	if ms != nil {
		t.Error("zero-value MapSlice should be nil")
	}
	if len(ms) != 0 {
		t.Errorf("len(nil MapSlice) = %d, want 0", len(ms))
	}
}

func TestMapItemAnyKeyType(t *testing.T) {
	// MapItem.Key is any, so it can hold different types
	items := []MapItem{
		{Key: "string-key", Value: 1},
		{Key: 42, Value: "int-key"},
		{Key: nil, Value: "nil-key"},
	}
	if items[0].Key != "string-key" {
		t.Errorf("string key = %v", items[0].Key)
	}
	if items[1].Key != 42 {
		t.Errorf("int key = %v", items[1].Key)
	}
	if items[2].Key != nil {
		t.Errorf("nil key = %v", items[2].Key)
	}
}

func TestTrimTrailingZeros(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123000000", "123"},
		{"100000000", "1"},
		{"123456789", "123456789"},
		{"000000001", "000000001"},
		{"120000000", "12"},
	}
	for _, tt := range tests {
		got := trimTrailingZeros(tt.input)
		if got != tt.want {
			t.Errorf("trimTrailingZeros(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
