package toml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ToJSON converts TOML data to JSON. TOML's nan and inf floats have no JSON
// form, so a document holding one returns an error naming its key.
func ToJSON(tomlData []byte) ([]byte, error) {
	var m map[string]any
	if err := Unmarshal(tomlData, &m); err != nil {
		return nil, err
	}
	if key, f, ok := findNonFinite(m, ""); ok {
		return nil, fmt.Errorf("toml: json: key %q: %v %w", key, f, errNonFiniteJSON)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("toml: json marshal: %w", err)
	}
	return b, nil
}

// findNonFinite returns the path of the first nan or inf float in v, in
// key order.
func findNonFinite(v any, path string) (string, float64, bool) {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return path, val, true
		}
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if key, f, ok := findNonFinite(val[k], p); ok {
				return key, f, true
			}
		}
	case []any:
		for i, e := range val {
			if key, f, ok := findNonFinite(e, fmt.Sprintf("%s[%d]", path, i)); ok {
				return key, f, true
			}
		}
	}
	return "", 0, false
}

// FromJSON converts JSON data to TOML. TOML has no null, so object members
// that are null are left out; a null array element returns an error, as
// does an integer outside the int64 range TOML integers have.
func FromJSON(jsonData []byte) ([]byte, error) {
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("toml: json decode: %w", err)
	}
	converted, err := convertJSONValues(m)
	if err != nil {
		return nil, err
	}
	return Marshal(converted)
}

func convertJSONValues(v any) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			c, err := convertJSONValues(child)
			if err != nil {
				return nil, err
			}
			val[k] = c
		}
		return val, nil
	case []any:
		for i, child := range val {
			c, err := convertJSONValues(child)
			if err != nil {
				return nil, err
			}
			val[i] = c
		}
		return val, nil
	case json.Number:
		s := val.String()
		if strings.ContainsAny(s, ".eE") {
			f, err := val.Float64()
			if err != nil {
				return nil, fmt.Errorf("toml: json number %s: %w", s, err)
			}
			return f, nil
		}
		i, err := val.Int64()
		if err != nil {
			return nil, fmt.Errorf("toml: json number %s %w", s, errJSONIntOverflow)
		}
		return i, nil
	default:
		return v, nil
	}
}
