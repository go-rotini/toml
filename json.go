package toml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ToJSON converts TOML data to JSON.
func ToJSON(tomlData []byte) ([]byte, error) {
	var m map[string]any
	if err := Unmarshal(tomlData, &m); err != nil {
		return nil, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("toml: json marshal: %w", err)
	}
	return b, nil
}

// FromJSON converts JSON data to TOML.
func FromJSON(jsonData []byte) ([]byte, error) {
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("toml: json decode: %w", err)
	}
	converted := convertJSONValues(m)
	if conv, ok := converted.(map[string]any); ok {
		return Marshal(conv)
	}
	return Marshal(converted)
}

func convertJSONValues(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			val[k] = convertJSONValues(child)
		}
		return val
	case []any:
		for i, child := range val {
			val[i] = convertJSONValues(child)
		}
		return val
	case json.Number:
		s := val.String()
		if strings.Contains(s, ".") || strings.Contains(s, "e") || strings.Contains(s, "E") {
			f, err := val.Float64()
			if err == nil {
				return f
			}
		}
		i, err := val.Int64()
		if err == nil {
			return i
		}
		return s
	default:
		return v
	}
}

// marshalJSONValue encodes a Go value as a TOML-safe JSON value.
func marshalJSONValue(v any) string {
	switch val := v.(type) {
	case nil:
		return `""`
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return `""`
		}
		return string(b)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		if math.IsInf(val, 1) {
			return `"inf"`
		}
		if math.IsInf(val, -1) {
			return `"-inf"`
		}
		if math.IsNaN(val) {
			return `"nan"`
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case time.Time:
		return fmt.Sprintf("%q", val.Format(time.RFC3339Nano))
	case LocalDate:
		return fmt.Sprintf("%q", val.String())
	case LocalTime:
		return fmt.Sprintf("%q", val.String())
	case LocalDateTime:
		return fmt.Sprintf("%q", val.String())
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return `""`
		}
		return string(b)
	}
}
