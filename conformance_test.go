package toml

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testSuiteDir = "testdata/toml-test"

func loadTestFiles(t *testing.T) (valid, invalid []string) {
	t.Helper()
	path := filepath.Join(testSuiteDir, "tests", "files-toml-1.1.0")
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("toml-test suite not found at %s: %v", testSuiteDir, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		isTOML := strings.HasSuffix(line, ".toml") || strings.HasSuffix(line, ".multi")
		if !isTOML {
			continue
		}
		switch {
		case strings.HasPrefix(line, "valid/"):
			valid = append(valid, line)
		case strings.HasPrefix(line, "invalid/"):
			invalid = append(invalid, line)
		}
	}
	return valid, invalid
}

func toTaggedJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[k] = toTaggedJSON(v)
		}
		return out
	case []any:
		if isArrayOfTables(val) {
			out := make([]any, len(val))
			for i, elem := range val {
				out[i] = toTaggedJSON(elem)
			}
			return out
		}
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = toTaggedJSON(elem)
		}
		return out
	case string:
		return map[string]any{"type": "string", "value": val}
	case int64:
		return map[string]any{"type": "integer", "value": strconv.FormatInt(val, 10)}
	case float64:
		return map[string]any{"type": "float", "value": formatFloat(val)}
	case bool:
		return map[string]any{"type": "bool", "value": strconv.FormatBool(val)}
	case time.Time:
		return map[string]any{"type": "datetime", "value": formatDatetime(val)}
	case LocalDateTime:
		return map[string]any{"type": "datetime-local", "value": val.String()}
	case LocalDate:
		return map[string]any{"type": "date-local", "value": val.String()}
	case LocalTime:
		return map[string]any{"type": "time-local", "value": val.String()}
	default:
		return map[string]any{"type": "string", "value": fmt.Sprintf("%v", val)}
	}
}

func isArrayOfTables(arr []any) bool {
	for _, v := range arr {
		if _, ok := v.(map[string]any); !ok {
			return false
		}
	}
	return len(arr) > 0
}

func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func formatDatetime(t time.Time) string {
	if t.Nanosecond() > 0 {
		s := t.Format("2006-01-02T15:04:05.999999999Z07:00")
		return s
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}

func compareTaggedJSON(t *testing.T, expected, got any) bool {
	t.Helper()
	switch exp := expected.(type) {
	case map[string]any:
		gotMap, ok := got.(map[string]any)
		if !ok {
			t.Errorf("expected map, got %T", got)
			return false
		}
		if typ, ok := exp["type"]; ok {
			if _, hasVal := exp["value"]; hasVal {
				return compareTaggedValue(t, typ.(string), exp["value"].(string), gotMap)
			}
		}
		for key, expVal := range exp {
			gotVal, ok := gotMap[key]
			if !ok {
				t.Errorf("missing key %q", key)
				return false
			}
			if !compareTaggedJSON(t, expVal, gotVal) {
				return false
			}
		}
		for key := range gotMap {
			if _, ok := exp[key]; !ok {
				t.Errorf("extra key %q", key)
				return false
			}
		}
		return true
	case []any:
		gotArr, ok := got.([]any)
		if !ok {
			t.Errorf("expected array, got %T", got)
			return false
		}
		if len(exp) != len(gotArr) {
			t.Errorf("array length: expected %d, got %d", len(exp), len(gotArr))
			return false
		}
		for i := range exp {
			if !compareTaggedJSON(t, exp[i], gotArr[i]) {
				return false
			}
		}
		return true
	default:
		t.Errorf("unexpected expected type %T", expected)
		return false
	}
}

func compareTaggedValue(t *testing.T, typ, expVal string, got map[string]any) bool {
	t.Helper()
	gotType, _ := got["type"].(string)
	gotVal, _ := got["value"].(string)
	if gotType != typ {
		t.Errorf("type mismatch: expected %q, got %q", typ, gotType)
		return false
	}
	switch typ {
	case "float":
		return compareFloats(t, expVal, gotVal)
	case "datetime":
		return compareDatetimes(t, expVal, gotVal)
	default:
		if expVal != gotVal {
			t.Errorf("value mismatch for type %s: expected %q, got %q", typ, expVal, gotVal)
			return false
		}
		return true
	}
}

func compareFloats(t *testing.T, expected, got string) bool {
	t.Helper()
	if expected == "nan" && got == "nan" {
		return true
	}
	if expected == got {
		return true
	}
	expF, err1 := strconv.ParseFloat(expected, 64)
	gotF, err2 := strconv.ParseFloat(got, 64)
	if err1 != nil || err2 != nil {
		t.Errorf("float parse error: expected %q, got %q", expected, got)
		return false
	}
	if expF != gotF {
		t.Errorf("float value mismatch: expected %s (%g), got %s (%g)", expected, expF, got, gotF)
		return false
	}
	return true
}

func compareDatetimes(t *testing.T, expected, got string) bool {
	t.Helper()
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	var expTime, gotTime time.Time
	var err error
	for _, layout := range layouts {
		expTime, err = time.Parse(layout, expected)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Errorf("cannot parse expected datetime %q: %v", expected, err)
		return false
	}
	for _, layout := range layouts {
		gotTime, err = time.Parse(layout, got)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Errorf("cannot parse got datetime %q: %v", got, err)
		return false
	}
	if !expTime.Equal(gotTime) {
		t.Errorf("datetime mismatch: expected %s, got %s", expected, got)
		return false
	}
	return true
}

func TestTOMLTestSuite(t *testing.T) {
	validFiles, invalidFiles := loadTestFiles(t)

	var passed, failed, errCorrect, errMissed, skipped, timedOut atomic.Int32
	var failedIDs []string
	var failedMu sync.Mutex

	addFailed := func(id string) {
		failedMu.Lock()
		failedIDs = append(failedIDs, id)
		failedMu.Unlock()
	}

	t.Run("valid", func(t *testing.T) {
		for _, file := range validFiles {
			name := strings.TrimPrefix(file, "valid/")
			name = strings.TrimSuffix(name, ".toml")
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				done := make(chan struct{})
				go func() {
					defer close(done)
					runValidTest(t, file, &passed, &failed, &skipped, addFailed)
				}()
				select {
				case <-done:
				case <-ctx.Done():
					timedOut.Add(1)
					addFailed(name + " (timeout)")
					t.Fatalf("test timed out: %s", name)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		for _, file := range invalidFiles {
			name := strings.TrimPrefix(file, "invalid/")
			name = strings.TrimSuffix(name, ".toml")
			name = strings.TrimSuffix(name, ".multi")
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				done := make(chan struct{})
				go func() {
					defer close(done)
					if strings.HasSuffix(file, ".multi") {
						runMultiInvalidTest(t, file, &errCorrect, &errMissed, addFailed)
					} else {
						runInvalidTest(t, file, &errCorrect, &errMissed, addFailed)
					}
				}()
				select {
				case <-done:
				case <-ctx.Done():
					timedOut.Add(1)
					addFailed(name + " (timeout)")
					t.Fatalf("test timed out: %s", name)
				}
			})
		}
	})

	t.Cleanup(func() {
		total := passed.Load() + failed.Load() + errCorrect.Load() + errMissed.Load() + skipped.Load() + timedOut.Load()
		t.Logf("\n=== TOML Test Suite Results ===")
		t.Logf("Total:          %d", total)
		t.Logf("Valid pass:     %d", passed.Load())
		t.Logf("Valid fail:     %d", failed.Load())
		t.Logf("Error correct:  %d (correctly rejected invalid input)", errCorrect.Load())
		t.Logf("Error missed:   %d (should reject but accepted)", errMissed.Load())
		t.Logf("Timed out:      %d", timedOut.Load())
		t.Logf("Skipped:        %d", skipped.Load())
		if total > 0 {
			passRate := float64(passed.Load()+errCorrect.Load()) / float64(total) * 100
			t.Logf("Pass rate:      %.1f%%", passRate)
		}
		if len(failedIDs) > 0 {
			t.Logf("Failed IDs:     %s", strings.Join(failedIDs, ", "))
		}
	})
}

func runValidTest(t *testing.T, file string, passed, failed, skipped *atomic.Int32, addFailed func(string)) {
	t.Helper()

	tomlPath := filepath.Join(testSuiteDir, "tests", file)
	jsonPath := strings.TrimSuffix(tomlPath, ".toml") + ".json"

	tomlData, err := os.ReadFile(tomlPath)
	if err != nil {
		skipped.Add(1)
		t.Skipf("cannot read %s: %v", tomlPath, err)
		return
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		skipped.Add(1)
		t.Skipf("cannot read %s: %v", jsonPath, err)
		return
	}

	var result map[string]any
	if err := Unmarshal(tomlData, &result); err != nil {
		failed.Add(1)
		addFailed(file)
		t.Errorf("Unmarshal failed: %v\n  input: %s", err, truncate(string(tomlData), 200))
		return
	}

	got := toTaggedJSON(result)

	var expected any
	if err := json.Unmarshal(jsonData, &expected); err != nil {
		skipped.Add(1)
		t.Skipf("cannot parse expected JSON for %s: %v", file, err)
		return
	}

	if !compareTaggedJSON(t, expected, got) {
		failed.Add(1)
		addFailed(file)
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		t.Logf("expected JSON:\n%s", string(jsonData))
		t.Logf("got JSON:\n%s", string(gotJSON))
		return
	}

	passed.Add(1)
}

func runInvalidTest(t *testing.T, file string, errCorrect, errMissed *atomic.Int32, addFailed func(string)) {
	t.Helper()

	tomlPath := filepath.Join(testSuiteDir, "tests", file)
	tomlData, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Skipf("cannot read %s: %v", tomlPath, err)
		return
	}

	var result map[string]any
	if err := Unmarshal(tomlData, &result); err != nil {
		errCorrect.Add(1)
		return
	}

	errMissed.Add(1)
	addFailed(file)
	t.Errorf("expected error but parsed successfully: %s\n  input: %s", file, truncate(string(tomlData), 200))
}

func runMultiInvalidTest(t *testing.T, file string, errCorrect, errMissed *atomic.Int32, addFailed func(string)) {
	t.Helper()

	multiPath := filepath.Join(testSuiteDir, "tests", file)
	data, err := os.ReadFile(multiPath)
	if err != nil {
		t.Skipf("cannot read %s: %v", multiPath, err)
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		input := line + "\n"
		var result map[string]any
		if err := Unmarshal([]byte(input), &result); err != nil {
			errCorrect.Add(1)
			continue
		}
		errMissed.Add(1)
		addFailed(file + ": " + truncate(line, 60))
		t.Errorf("expected error for line %q in %s", truncate(line, 80), file)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
