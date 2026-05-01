package toml

import (
	"fmt"
	"time"
)

// MapSlice is an ordered slice of key-value pairs. It is used as the
// decoded representation of TOML tables when [WithOrderedMap] is enabled,
// preserving the original key order that a plain map[string]any would lose.
type MapSlice []MapItem

// MapItem is a single key-value pair within a [MapSlice].
type MapItem struct {
	Key   any
	Value any
}

// LocalDate represents a date without a timezone, as specified in TOML's
// local date type (e.g., 1979-05-27). Go's [time.Time] always carries
// timezone information, so this type preserves TOML's semantics.
type LocalDate struct {
	Year  int
	Month time.Month
	Day   int
}

// String returns the date in RFC 3339 full-date format (YYYY-MM-DD).
func (d LocalDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// MarshalText implements [encoding.TextMarshaler].
func (d LocalDate) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (d *LocalDate) UnmarshalText(data []byte) error {
	t, err := time.Parse("2006-01-02", string(data))
	if err != nil {
		return fmt.Errorf("toml: invalid local date %q: %w", data, err)
	}
	d.Year = t.Year()
	d.Month = t.Month()
	d.Day = t.Day()
	return nil
}

// AsTime converts the local date to a [time.Time] in the given location.
func (d LocalDate) AsTime(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// LocalTime represents a time of day without a date or timezone, as
// specified in TOML's local time type (e.g., 07:32:00).
type LocalTime struct {
	Hour       int
	Minute     int
	Second     int
	Nanosecond int
}

// String returns the time in HH:MM:SS format, with fractional seconds appended if non-zero.
func (t LocalTime) String() string {
	s := fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	if t.Nanosecond > 0 {
		frac := fmt.Sprintf("%09d", t.Nanosecond)
		frac = trimTrailingZeros(frac)
		s += "." + frac
	}
	return s
}

// MarshalText implements [encoding.TextMarshaler].
func (t LocalTime) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (t *LocalTime) UnmarshalText(data []byte) error {
	layouts := []string{
		"15:04:05.999999999",
		"15:04:05",
		"15:04",
	}
	s := string(data)
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, s)
		if err == nil {
			t.Hour = parsed.Hour()
			t.Minute = parsed.Minute()
			t.Second = parsed.Second()
			t.Nanosecond = parsed.Nanosecond()
			return nil
		}
	}
	return fmt.Errorf("toml: invalid local time %q", data)
}

// LocalDateTime represents a date and time without a timezone, as
// specified in TOML's local date-time type (e.g., 1979-05-27T07:32:00).
type LocalDateTime struct {
	LocalDate
	LocalTime
}

// String returns the date-time in RFC 3339 format without a timezone offset (YYYY-MM-DDTHH:MM:SS).
func (dt LocalDateTime) String() string {
	return dt.LocalDate.String() + "T" + dt.LocalTime.String()
}

// MarshalText implements [encoding.TextMarshaler].
func (dt LocalDateTime) MarshalText() ([]byte, error) {
	return []byte(dt.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (dt *LocalDateTime) UnmarshalText(data []byte) error {
	layouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	s := string(data)
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			dt.Year = t.Year()
			dt.Month = t.Month()
			dt.Day = t.Day()
			dt.Hour = t.Hour()
			dt.Minute = t.Minute()
			dt.Second = t.Second()
			dt.Nanosecond = t.Nanosecond()
			return nil
		}
	}
	return fmt.Errorf("toml: invalid local date-time %q", data)
}

// AsTime converts the local date-time to a [time.Time] in the given location.
func (dt LocalDateTime) AsTime(loc *time.Location) time.Time {
	return time.Date(dt.Year, dt.Month, dt.Day, dt.Hour, dt.Minute, dt.Second, dt.Nanosecond, loc)
}

func trimTrailingZeros(s string) string {
	i := len(s) - 1
	for i > 0 && s[i] == '0' {
		i--
	}
	return s[:i+1]
}
