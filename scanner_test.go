package toml

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// tokenKinds extracts just the kinds from a token slice, ignoring
// STREAM-START and STREAM-END so tests read more naturally.
func tokenKinds(tokens []token) []tokenKind { //nolint:unused
	var out []tokenKind
	for _, t := range tokens {
		if t.kind == tokenStreamStart || t.kind == tokenStreamEnd {
			continue
		}
		out = append(out, t.kind)
	}
	return out
}

// tokenValues returns (kind, value) pairs, excluding stream markers.
func tokenValues(tokens []token) []token {
	var out []token
	for _, t := range tokens {
		if t.kind == tokenStreamStart || t.kind == tokenStreamEnd {
			continue
		}
		out = append(out, t)
	}
	return out
}

// mustScan scans input and fails the test on error.
func mustScan(t *testing.T, input string) []token {
	t.Helper()
	s := newScanner([]byte(input))
	tokens, err := s.scan()
	if err != nil {
		t.Fatalf("scan(%q) unexpected error: %v", input, err)
	}
	return tokens
}

// mustScanError scans input and expects an error.
func mustScanError(t *testing.T, input string) error {
	t.Helper()
	s := newScanner([]byte(input))
	_, err := s.scan()
	if err == nil {
		t.Fatalf("scan(%q) expected error, got nil", input)
	}
	return err
}

// findToken returns the first token of the given kind, or fails.
func findToken(t *testing.T, tokens []token, kind tokenKind) token {
	t.Helper()
	for _, tok := range tokens {
		if tok.kind == kind {
			return tok
		}
	}
	t.Fatalf("token kind %v not found", kind)
	return token{}
}

// findTokenValue returns the first token with the given kind and value, or fails.
func findTokenValue(t *testing.T, tokens []token, kind tokenKind, value string) token {
	t.Helper()
	for _, tok := range tokens {
		if tok.kind == kind && tok.value == value {
			return tok
		}
	}
	t.Fatalf("token kind=%v value=%q not found", kind, value)
	return token{}
}

// ---------------------------------------------------------------------------
// 1. BOM handling
// ---------------------------------------------------------------------------

func TestBOMStripping(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	input := bom + "key = \"value\""
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "key")
	findTokenValue(t, tokens, tokenBasicString, "value")
}

func TestBOMOnlyInput(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	tokens := mustScan(t, bom)
	// Should only have stream markers
	tv := tokenValues(tokens)
	if len(tv) != 0 {
		t.Errorf("expected no tokens besides stream markers, got %d", len(tv))
	}
}

func TestNoBOM(t *testing.T) {
	tokens := mustScan(t, "key = 1")
	findTokenValue(t, tokens, tokenBareKey, "key")
}

// ---------------------------------------------------------------------------
// 2. CRLF normalization
// ---------------------------------------------------------------------------

func TestCRLFNormalization(t *testing.T) {
	input := "key1 = 1\r\nkey2 = 2\r\n"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "key1")
	findTokenValue(t, tokens, tokenBareKey, "key2")
	findTokenValue(t, tokens, tokenInteger, "1")
	findTokenValue(t, tokens, tokenInteger, "2")
}

func TestBareCRNormalization(t *testing.T) {
	// Bare \r (not followed by \n) should be normalized to \n
	input := "key1 = 1\rkey2 = 2"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "key1")
	findTokenValue(t, tokens, tokenBareKey, "key2")
}

func TestCRLFInMultiLineBasicString(t *testing.T) {
	input := "key = \"\"\"\r\nhello\r\nworld\r\n\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	// After CRLF normalization and opening newline trim
	if tok.value != "hello\nworld\n" {
		t.Errorf("got value %q, want %q", tok.value, "hello\nworld\n")
	}
}

// ---------------------------------------------------------------------------
// 3. Basic strings - escape sequences
// ---------------------------------------------------------------------------

func TestBasicStringEscapeSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"backspace", `key = "\b"`, "\b"},
		{"tab", `key = "\t"`, "\t"},
		{"newline", `key = "\n"`, "\n"},
		{"formfeed", `key = "\f"`, "\f"},
		{"carriage return", `key = "\r"`, "\r"},
		{"quote", `key = "\""`, "\""},
		{"backslash", `key = "\\"`, "\\"},
		{"escape (TOML 1.1.0)", `key = "\e"`, "\x1B"},
		{"hex xHH (TOML 1.1.0)", `key = "\x41"`, "A"},
		{"hex xHH lowercase", `key = "\x61"`, "a"},
		{"unicode u4", `key = "A"`, "A"},
		{"unicode u4 non-ASCII", `key = "é"`, "é"},
		{"unicode U8", `key = "\U00000041"`, "A"},
		{"unicode U8 emoji", `key = "\U0001F600"`, "\U0001F600"},
		{"multiple escapes", `key = "\t\n\r"`, "\t\n\r"},
		{"mixed text and escapes", `key = "hello\nworld"`, "hello\nworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenBasicString)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestBasicStringUnicodeSurrogateError(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"high surrogate u", `key = "\uD800"`},
		{"low surrogate u", `key = "\uDFFF"`},
		{"mid surrogate u", `key = "\uD900"`},
		{"high surrogate U", `key = "\U0000D800"`},
		{"low surrogate U", `key = "\U0000DFFF"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mustScanError(t, tt.input)
			if !strings.Contains(err.Error(), "surrogate") {
				t.Errorf("expected surrogate error, got: %v", err)
			}
		})
	}
}

func TestBasicStringInvalidEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid escape a", `key = "\a"`},
		{"invalid escape v", `key = "\v"`},
		{"invalid escape 0", `key = "\0"`},
		{"invalid escape q", `key = "\q"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

func TestBasicStringEmpty(t *testing.T) {
	tokens := mustScan(t, `key = ""`)
	tok := findToken(t, tokens, tokenBasicString)
	if tok.value != "" {
		t.Errorf("got %q, want empty string", tok.value)
	}
}

// ---------------------------------------------------------------------------
// 4. Multi-line basic strings
// ---------------------------------------------------------------------------

func TestMultiLineBasicStringOpeningNewlineTrim(t *testing.T) {
	input := "key = \"\"\"\nhello\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "hello" {
		t.Errorf("got %q, want %q", tok.value, "hello")
	}
}

func TestMultiLineBasicStringNoOpeningNewline(t *testing.T) {
	input := "key = \"\"\"hello\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "hello" {
		t.Errorf("got %q, want %q", tok.value, "hello")
	}
}

func TestMultiLineBasicStringLineContinuation(t *testing.T) {
	input := "key = \"\"\"\nhello \\\n  world\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "hello world" {
		t.Errorf("got %q, want %q", tok.value, "hello world")
	}
}

func TestMultiLineBasicStringLineContinuationMultipleLines(t *testing.T) {
	input := "key = \"\"\"\nhello \\\n  \n  \n  world\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "hello world" {
		t.Errorf("got %q, want %q", tok.value, "hello world")
	}
}

func TestMultiLineBasicStringQuotesInside(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"one quote", "key = \"\"\"he\"llo\"\"\"", "he\"llo"},
		{"two quotes", "key = \"\"\"he\"\"llo\"\"\"", "he\"\"llo"},
		{"two quotes at end", "key = \"\"\"hello\"\"\"\"\"", "hello\"\""},
		{"one quote at end", "key = \"\"\"hello\"\"\"\"", "hello\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenMultiLineBasicString)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestMultiLineBasicStringTooManyQuotes(t *testing.T) {
	// 6 quotes in a row inside should error (3 closing + 3 extra)
	input := "key = \"\"\"hello\"\"\"\"\"\"\"\"\""
	mustScanError(t, input)
}

func TestMultiLineBasicStringEscapes(t *testing.T) {
	input := "key = \"\"\"\nhello\\nworld\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "hello\nworld" {
		t.Errorf("got %q, want %q", tok.value, "hello\nworld")
	}
}

func TestMultiLineBasicStringEmpty(t *testing.T) {
	input := "key = \"\"\"\"\"\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineBasicString)
	if tok.value != "" {
		t.Errorf("got %q, want empty string", tok.value)
	}
}

func TestMultiLineBasicStringUnterminated(t *testing.T) {
	input := "key = \"\"\"\nhello"
	mustScanError(t, input)
}

// ---------------------------------------------------------------------------
// 5. Literal strings
// ---------------------------------------------------------------------------

func TestLiteralStringNoEscapeProcessing(t *testing.T) {
	input := `key = 'hello\nworld'`
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenLiteralString)
	if tok.value != `hello\nworld` {
		t.Errorf("got %q, want %q", tok.value, `hello\nworld`)
	}
}

func TestLiteralStringBackslash(t *testing.T) {
	input := `key = 'C:\path\to\file'`
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenLiteralString)
	if tok.value != `C:\path\to\file` {
		t.Errorf("got %q, want %q", tok.value, `C:\path\to\file`)
	}
}

func TestLiteralStringEmpty(t *testing.T) {
	input := "key = ''"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenLiteralString)
	if tok.value != "" {
		t.Errorf("got %q, want empty string", tok.value)
	}
}

func TestLiteralStringUnterminated(t *testing.T) {
	mustScanError(t, "key = 'hello")
}

func TestLiteralStringNewlineError(t *testing.T) {
	mustScanError(t, "key = 'hello\nworld'")
}

// ---------------------------------------------------------------------------
// 6. Multi-line literal strings
// ---------------------------------------------------------------------------

func TestMultiLineLiteralStringOpeningNewlineTrim(t *testing.T) {
	input := "key = '''\nhello'''"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineLiteralString)
	if tok.value != "hello" {
		t.Errorf("got %q, want %q", tok.value, "hello")
	}
}

func TestMultiLineLiteralStringNoEscapeProcessing(t *testing.T) {
	input := "key = '''\nhello\\nworld'''"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineLiteralString)
	if tok.value != `hello\nworld` {
		t.Errorf("got %q, want %q", tok.value, `hello\nworld`)
	}
}

func TestMultiLineLiteralStringQuotesInside(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"one quote", "key = '''he'llo'''", "he'llo"},
		{"two quotes", "key = '''he''llo'''", "he''llo"},
		{"two quotes at end", "key = '''hello'''''", "hello''"},
		{"one quote at end", "key = '''hello''''", "hello'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenMultiLineLiteralString)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestMultiLineLiteralStringTooManyQuotes(t *testing.T) {
	input := "key = '''hello''''''''''"
	mustScanError(t, input)
}

func TestMultiLineLiteralStringUnterminated(t *testing.T) {
	mustScanError(t, "key = '''\nhello")
}

func TestMultiLineLiteralStringEmpty(t *testing.T) {
	input := "key = ''''''"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineLiteralString)
	if tok.value != "" {
		t.Errorf("got %q, want empty string", tok.value)
	}
}

func TestMultiLineLiteralStringPreservesNewlines(t *testing.T) {
	input := "key = '''\nline1\nline2\n'''"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenMultiLineLiteralString)
	want := "line1\nline2\n"
	if tok.value != want {
		t.Errorf("got %q, want %q", tok.value, want)
	}
}

// ---------------------------------------------------------------------------
// 7. Integers
// ---------------------------------------------------------------------------

func TestIntegers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"zero", "key = 0", "0"},
		{"positive", "key = 42", "42"},
		{"positive signed", "key = +42", "+42"},
		{"negative", "key = -42", "-42"},
		{"large", "key = 1000000", "1000000"},
		{"underscores", "key = 1_000_000", "1_000_000"},
		{"hex lowercase", "key = 0xdeadbeef", "0xdeadbeef"},
		{"hex uppercase", "key = 0xDEADBEEF", "0xDEADBEEF"},
		{"hex mixed", "key = 0xDEAD_BEEF", "0xDEAD_BEEF"},
		{"octal", "key = 0o755", "0o755"},
		{"octal underscore", "key = 0o7_5_5", "0o7_5_5"},
		{"binary", "key = 0b11010110", "0b11010110"},
		{"binary underscore", "key = 0b1101_0110", "0b1101_0110"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenInteger)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestIntegerLeadingZeroRejected(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"leading zero", "key = 0123"},
		{"leading zero with sign", "key = +0123"},
		{"leading zero negative", "key = -0123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mustScanError(t, tt.input)
			if !strings.Contains(err.Error(), "leading zero") {
				t.Errorf("expected leading zeros error, got: %v", err)
			}
		})
	}
}

func TestIntegerUnderscoreErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"trailing underscore", "key = 100_"},
		{"double underscore", "key = 1__0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

// ---------------------------------------------------------------------------
// 8. Floats
// ---------------------------------------------------------------------------

func TestFloats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"decimal", "key = 3.14", "3.14"},
		{"positive signed", "key = +3.14", "+3.14"},
		{"negative", "key = -3.14", "-3.14"},
		{"exponent", "key = 1e10", "1e10"},
		{"exponent positive", "key = 1e+10", "1e+10"},
		{"exponent negative", "key = 1e-10", "1e-10"},
		{"decimal and exponent", "key = 6.626e-34", "6.626e-34"},
		{"exponent uppercase", "key = 1E10", "1E10"},
		{"exponent leading zeros (1.1.0)", "key = 1e02", "1e02"},
		{"zero decimal", "key = 0.0", "0.0"},
		{"inf", "key = inf", "inf"},
		{"positive inf", "key = +inf", "+inf"},
		{"negative inf", "key = -inf", "-inf"},
		{"nan", "key = nan", "nan"},
		{"positive nan", "key = +nan", "+nan"},
		{"negative nan", "key = -nan", "-nan"},
		{"underscore in float", "key = 3_000.14", "3_000.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenFloat)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestFloatNoDigitAfterDot(t *testing.T) {
	mustScanError(t, "key = 3.")
}

// ---------------------------------------------------------------------------
// 9. Booleans
// ---------------------------------------------------------------------------

func TestBooleans(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"key = true", "true"},
		{"key = false", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenBoolean)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestBooleanInvalidCase(t *testing.T) {
	tests := []string{
		"key = True",
		"key = TRUE",
		"key = False",
		"key = FALSE",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			mustScanError(t, input)
		})
	}
}

// ---------------------------------------------------------------------------
// 10. Date/time
// ---------------------------------------------------------------------------

func TestOffsetDateTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"UTC Z", "key = 1979-05-27T07:32:00Z", "1979-05-27T07:32:00Z"},
		{"UTC lowercase z", "key = 1979-05-27T07:32:00z", "1979-05-27T07:32:00z"},
		{"positive offset", "key = 1979-05-27T07:32:00+00:00", "1979-05-27T07:32:00+00:00"},
		{"negative offset", "key = 1979-05-27T07:32:00-05:00", "1979-05-27T07:32:00-05:00"},
		{"fractional seconds Z", "key = 1979-05-27T00:32:00.999999Z", "1979-05-27T00:32:00.999999Z"},
		{"space delimiter", "key = 1979-05-27 07:32:00Z", "1979-05-27 07:32:00Z"},
		{"lowercase t", "key = 1979-05-27t07:32:00Z", "1979-05-27t07:32:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenOffsetDateTime)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestLocalDateTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic", "key = 1979-05-27T07:32:00", "1979-05-27T07:32:00"},
		{"fractional", "key = 1979-05-27T00:32:00.999999", "1979-05-27T00:32:00.999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenLocalDateTime)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

func TestLocalDate(t *testing.T) {
	input := "key = 1979-05-27"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenLocalDate)
	if tok.value != "1979-05-27" {
		t.Errorf("got %q, want %q", tok.value, "1979-05-27")
	}
}

func TestLocalTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic", "key = 07:32:00", "07:32:00"},
		{"fractional", "key = 00:32:00.999999", "00:32:00.999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			tok := findToken(t, tokens, tokenLocalTime)
			if tok.value != tt.want {
				t.Errorf("got %q, want %q", tok.value, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 11. Table headers
// ---------------------------------------------------------------------------

func TestTableHeader(t *testing.T) {
	input := "[table]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findTokenValue(t, tokens, tokenBareKey, "table")
	findToken(t, tokens, tokenTableEnd)
}

func TestArrayTableHeader(t *testing.T) {
	input := "[[array]]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayTableStart)
	findTokenValue(t, tokens, tokenBareKey, "array")
	findToken(t, tokens, tokenArrayTableEnd)
}

func TestDottedKeyInTableHeader(t *testing.T) {
	input := "[a.b.c]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	tv := tokenValues(tokens)
	// Expect: TABLE-START, BARE-KEY(a), DOT, BARE-KEY(b), DOT, BARE-KEY(c), TABLE-END
	bareKeys := 0
	dots := 0
	for _, tok := range tv {
		if tok.kind == tokenBareKey {
			bareKeys++
		}
		if tok.kind == tokenDot {
			dots++
		}
	}
	if bareKeys != 3 {
		t.Errorf("expected 3 bare keys, got %d", bareKeys)
	}
	if dots != 2 {
		t.Errorf("expected 2 dots, got %d", dots)
	}
}

func TestQuotedKeyInTableHeader(t *testing.T) {
	input := `["quoted key"]`
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findTokenValue(t, tokens, tokenBasicString, "quoted key")
	findToken(t, tokens, tokenTableEnd)
}

func TestDottedArrayTableHeader(t *testing.T) {
	input := "[[a.b]]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayTableStart)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findToken(t, tokens, tokenDot)
	findTokenValue(t, tokens, tokenBareKey, "b")
	findToken(t, tokens, tokenArrayTableEnd)
}

func TestTableHeaderWithWhitespace(t *testing.T) {
	input := "[  table  ]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findTokenValue(t, tokens, tokenBareKey, "table")
	findToken(t, tokens, tokenTableEnd)
}

// ---------------------------------------------------------------------------
// 12. Inline tables
// ---------------------------------------------------------------------------

func TestInlineTable(t *testing.T) {
	input := `key = {a = 1, b = "two"}`
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenInlineTableStart)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findTokenValue(t, tokens, tokenInteger, "1")
	findToken(t, tokens, tokenComma)
	findTokenValue(t, tokens, tokenBareKey, "b")
	findTokenValue(t, tokens, tokenBasicString, "two")
	findToken(t, tokens, tokenInlineTableEnd)
}

func TestInlineTableEmpty(t *testing.T) {
	input := "key = {}"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenInlineTableStart)
	findToken(t, tokens, tokenInlineTableEnd)
}

func TestInlineTableTrailingComma(t *testing.T) {
	// TOML 1.1.0 allows trailing commas
	input := "key = {a = 1, b = 2,}"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenInlineTableStart)
	findToken(t, tokens, tokenInlineTableEnd)
}

func TestScanInlineTableNewlines(t *testing.T) {
	// TOML 1.1.0 allows newlines in inline tables
	input := "key = {\na = 1,\nb = 2\n}"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenInlineTableStart)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findTokenValue(t, tokens, tokenBareKey, "b")
	findToken(t, tokens, tokenInlineTableEnd)
}

func TestInlineTableNested(t *testing.T) {
	input := `key = {a = {b = 1}}`
	tokens := mustScan(t, input)
	// Should have two InlineTableStart and two InlineTableEnd
	starts := 0
	ends := 0
	for _, tok := range tokens {
		if tok.kind == tokenInlineTableStart {
			starts++
		}
		if tok.kind == tokenInlineTableEnd {
			ends++
		}
	}
	if starts != 2 {
		t.Errorf("expected 2 inline table starts, got %d", starts)
	}
	if ends != 2 {
		t.Errorf("expected 2 inline table ends, got %d", ends)
	}
}

// ---------------------------------------------------------------------------
// 13. Arrays
// ---------------------------------------------------------------------------

func TestArray(t *testing.T) {
	input := "key = [1, 2, 3]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findTokenValue(t, tokens, tokenInteger, "1")
	findTokenValue(t, tokens, tokenInteger, "2")
	findTokenValue(t, tokens, tokenInteger, "3")
	findToken(t, tokens, tokenArrayEnd)
}

func TestArrayTrailingComma(t *testing.T) {
	input := "key = [1, 2, 3,]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findToken(t, tokens, tokenArrayEnd)
}

func TestArrayEmpty(t *testing.T) {
	input := "key = []"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findToken(t, tokens, tokenArrayEnd)
}

func TestArrayWithBooleans(t *testing.T) {
	input := "key = [true, false, true]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findTokenValue(t, tokens, tokenBoolean, "true")
	findTokenValue(t, tokens, tokenBoolean, "false")
	findToken(t, tokens, tokenArrayEnd)
}

func TestArrayNested(t *testing.T) {
	input := "key = [[1, 2], [3, 4]]"
	tokens := mustScan(t, input)
	starts := 0
	ends := 0
	for _, tok := range tokens {
		if tok.kind == tokenArrayStart {
			starts++
		}
		if tok.kind == tokenArrayEnd {
			ends++
		}
	}
	if starts != 3 {
		t.Errorf("expected 3 array starts, got %d", starts)
	}
	if ends != 3 {
		t.Errorf("expected 3 array ends, got %d", ends)
	}
}

func TestArrayStrings(t *testing.T) {
	input := `key = ["a", "b", "c"]`
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBasicString, "a")
	findTokenValue(t, tokens, tokenBasicString, "b")
	findTokenValue(t, tokens, tokenBasicString, "c")
}

func TestArrayMixedTypes(t *testing.T) {
	// Scanner doesn't enforce type constraints; that's the parser's job
	input := `key = [1, "two", true]`
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "1")
	findTokenValue(t, tokens, tokenBasicString, "two")
	findTokenValue(t, tokens, tokenBoolean, "true")
}

// ---------------------------------------------------------------------------
// 14. Comments
// ---------------------------------------------------------------------------

func TestComment(t *testing.T) {
	input := "# this is a comment\nkey = 1"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenComment)
	if tok.value != " this is a comment" {
		t.Errorf("got %q, want %q", tok.value, " this is a comment")
	}
	findTokenValue(t, tokens, tokenBareKey, "key")
}

func TestCommentInline(t *testing.T) {
	input := "key = 1 # inline comment"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "1")
	tok := findToken(t, tokens, tokenComment)
	if tok.value != " inline comment" {
		t.Errorf("got %q, want %q", tok.value, " inline comment")
	}
}

func TestCommentEmpty(t *testing.T) {
	input := "#\nkey = 1"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenComment)
	if tok.value != "" {
		t.Errorf("got %q, want empty", tok.value)
	}
}

func TestCommentControlCharRejected(t *testing.T) {
	// 0x01 is a control char, should be rejected in comments
	input := "# hello\x01world"
	mustScanError(t, input)
}

func TestCommentTabAllowed(t *testing.T) {
	input := "# hello\tworld"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenComment)
	if tok.value != " hello\tworld" {
		t.Errorf("got %q, want %q", tok.value, " hello\tworld")
	}
}

func TestCommentControlCharsU0000toU0008(t *testing.T) {
	for c := byte(0x00); c <= 0x08; c++ {
		input := []byte("# hello")
		input = append(input, c)
		input = append(input, []byte("world")...)
		s := newScanner(input)
		_, err := s.scan()
		if err == nil {
			t.Errorf("expected error for control char U+%04X in comment", c)
		}
	}
}

// ---------------------------------------------------------------------------
// 15. Unicode bare keys (TOML 1.1.0)
// ---------------------------------------------------------------------------

func TestBareKeyASCII(t *testing.T) {
	input := "abc-123_DEF = 1"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "abc-123_DEF")
}

func TestBareKeyUnicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"accented", "é = 1", "é"},           // e with accent
		{"chinese", "世界 = 1", "世界"}, // 世界
		{"japanese", "こんにちは = 1", "こんにちは"}, // こんにちは
		{"mixed ascii unicode", "café = 1", "café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := mustScan(t, tt.input)
			findTokenValue(t, tokens, tokenBareKey, tt.want)
		})
	}
}

// ---------------------------------------------------------------------------
// 16. [ disambiguation (table header vs array value)
// ---------------------------------------------------------------------------

func TestBracketDisambiguation(t *testing.T) {
	// [ at line start should be a table header
	input := "[table]\nkey = [1, 2]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findToken(t, tokens, tokenTableEnd)
	findToken(t, tokens, tokenArrayStart)
	findToken(t, tokens, tokenArrayEnd)
}

func TestBracketAfterEquals(t *testing.T) {
	// [ after = should be array
	input := "key = [1]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findToken(t, tokens, tokenArrayEnd)
}

func TestBracketNotAtLineStart(t *testing.T) {
	// [ in an inline table after comma should be array value
	input := "key = {a = [1, 2]}"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	findToken(t, tokens, tokenArrayEnd)
}

func TestDoubleBracketTableVsNestedArray(t *testing.T) {
	// [[ at line start is array-of-tables
	// [[ in value position is nested arrays
	input := "[[aot]]\nkey = [[1], [2]]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayTableStart)
	findToken(t, tokens, tokenArrayTableEnd)
	// The value position should have nested array starts
	arrayStarts := 0
	for _, tok := range tokens {
		if tok.kind == tokenArrayStart {
			arrayStarts++
		}
	}
	if arrayStarts != 3 {
		t.Errorf("expected 3 array starts, got %d", arrayStarts)
	}
}

func TestTableHeaderAfterNewline(t *testing.T) {
	input := "key1 = 1\n[table]\nkey2 = 2"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findTokenValue(t, tokens, tokenBareKey, "table")
}

// ---------------------------------------------------------------------------
// 17. Error cases
// ---------------------------------------------------------------------------

func TestUnterminatedBasicString(t *testing.T) {
	mustScanError(t, `key = "hello`)
}

func TestUnterminatedMultiLineBasicString(t *testing.T) {
	mustScanError(t, "key = \"\"\"\nhello")
}

func TestUnterminatedLiteralString(t *testing.T) {
	mustScanError(t, "key = 'hello")
}

func TestUnterminatedMultiLineLiteralString(t *testing.T) {
	mustScanError(t, "key = '''\nhello")
}

func TestNewlineInBasicString(t *testing.T) {
	mustScanError(t, "key = \"hello\nworld\"")
}

func TestNewlineInLiteralString(t *testing.T) {
	mustScanError(t, "key = 'hello\nworld'")
}

func TestControlCharInBasicString(t *testing.T) {
	tests := []struct {
		name string
		char byte
	}{
		{"null", 0x00},
		{"soh", 0x01},
		{"bel", 0x07},
		{"vertical tab", 0x0B},
		{"form feed", 0x0C},
		{"delete", 0x7F},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte("key = \"hello")
			input = append(input, tt.char)
			input = append(input, []byte("world\"")...)
			s := newScanner(input)
			_, err := s.scan()
			if err == nil {
				t.Errorf("expected error for control char 0x%02X in basic string", tt.char)
			}
		})
	}
}

func TestTabAllowedInBasicString(t *testing.T) {
	input := "key = \"hello\tworld\""
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenBasicString)
	if tok.value != "hello\tworld" {
		t.Errorf("got %q, want %q", tok.value, "hello\tworld")
	}
}

func TestControlCharInLiteralString(t *testing.T) {
	input := []byte("key = 'hello")
	input = append(input, 0x01)
	input = append(input, []byte("world'")...)
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for control char in literal string")
	}
}

func TestControlCharInMultiLineBasicString(t *testing.T) {
	input := []byte("key = \"\"\"\nhello")
	input = append(input, 0x01)
	input = append(input, []byte("world\"\"\"")...)
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for control char in multi-line basic string")
	}
}

func TestControlCharInMultiLineLiteralString(t *testing.T) {
	input := []byte("key = '''\nhello")
	input = append(input, 0x01)
	input = append(input, []byte("world'''")...)
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for control char in multi-line literal string")
	}
}

func TestInvalidUTF8InString(t *testing.T) {
	// 0xFF is not valid UTF-8
	input := []byte("key = \"hello")
	input = append(input, 0xFF)
	input = append(input, []byte("world\"")...)
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for invalid UTF-8 in string")
	}
}

func TestInvalidUTF8InComment(t *testing.T) {
	input := []byte("# hello")
	input = append(input, 0xFF)
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for invalid UTF-8 in comment")
	}
}

func TestInvalidUTF8InBareKey(t *testing.T) {
	input := []byte{0xFF, ' ', '=', ' ', '1'}
	s := newScanner(input)
	_, err := s.scan()
	if err == nil {
		t.Error("expected error for invalid UTF-8 in bare key")
	}
}

func TestInvalidEscapeInBasicString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"backslash a", `key = "\a"`},
		{"backslash v", `key = "\v"`},
		{"backslash zero", `key = "\0"`},
		{"backslash space", `key = "\ "`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

func TestIncompleteHexEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"incomplete x", `key = "\x4"`},
		{"incomplete u", `key = "\u004"`},
		{"incomplete U", `key = "\U0000004"`},
		{"empty x", `key = "\x"`},
		{"empty u", `key = "\u"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

func TestInvalidHexDigitInEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"x with g", `key = "\xGG"`},
		{"u with z", `key = "\u00ZZ"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

func TestUnexpectedCharInValue(t *testing.T) {
	mustScanError(t, "key = @")
}

func TestUnexpectedEndAfterSign(t *testing.T) {
	mustScanError(t, "key = +")
}

func TestInvalidPrefixedInt(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"hex no digits", "key = 0x"},
		{"octal no digits", "key = 0o"},
		{"binary no digits", "key = 0b"},
		{"hex invalid digit", "key = 0xGG"},
		{"octal invalid digit", "key = 0o89"},
		{"binary invalid digit", "key = 0b23"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustScanError(t, tt.input)
		})
	}
}

// ---------------------------------------------------------------------------
// 18. Valid() function
// ---------------------------------------------------------------------------

func TestValidBasicDocument(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", true},
		{"simple kv", "key = 1", true},
		{"simple string kv", `key = "value"`, true},
		{"table", "[table]\nkey = 1", true},
		{"array table", "[[array]]\nkey = 1", true},
		{"comment only", "# comment", true},
		{"complex", "[server]\nhost = \"localhost\"\nport = 8080\nenabled = true", true},
		{"array", "key = [1, 2, 3]", true},
		{"inline table", "key = {a = 1, b = 2}", true},
		{"multiline basic string", "key = \"\"\"\nhello\n\"\"\"", true},
		{"multiline literal string", "key = '''\nhello\n'''", true},
		{"date", "key = 1979-05-27", true},
		{"datetime", "key = 1979-05-27T07:32:00Z", true},
		{"time", "key = 07:32:00", true},
		{"float", "key = 3.14", true},
		{"bool", "key = true", true},
		{"BOM", "\xEF\xBB\xBFkey = 1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Valid([]byte(tt.input))
			if got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidInvalidDocuments(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated string", `key = "hello`},
		{"invalid escape", `key = "\q"`},
		{"leading zeros", "key = 0123"},
		{"control char in string", "key = \"hello\x01world\""},
		{"bare key with special char", "key! = 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Valid([]byte(tt.input)) {
				t.Errorf("Valid(%q) = true, want false", tt.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestEmptyInput(t *testing.T) {
	tokens := mustScan(t, "")
	tv := tokenValues(tokens)
	if len(tv) != 0 {
		t.Errorf("expected no tokens besides stream markers, got %d", len(tv))
	}
}

func TestWhitespaceOnly(t *testing.T) {
	tokens := mustScan(t, "   \t  ")
	tv := tokenValues(tokens)
	if len(tv) != 0 {
		t.Errorf("expected no tokens besides stream markers, got %d", len(tv))
	}
}

func TestNewlineOnly(t *testing.T) {
	tokens := mustScan(t, "\n\n\n")
	tv := tokenValues(tokens)
	for _, tok := range tv {
		if tok.kind != tokenNewline {
			t.Errorf("expected only newlines, got %v", tok.kind)
		}
	}
}

func TestMultipleKeyValuePairs(t *testing.T) {
	input := "a = 1\nb = 2\nc = 3"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findTokenValue(t, tokens, tokenBareKey, "b")
	findTokenValue(t, tokens, tokenBareKey, "c")
	findTokenValue(t, tokens, tokenInteger, "1")
	findTokenValue(t, tokens, tokenInteger, "2")
	findTokenValue(t, tokens, tokenInteger, "3")
}

func TestScanDottedKeys(t *testing.T) {
	input := "a.b.c = 1"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findToken(t, tokens, tokenDot)
	findTokenValue(t, tokens, tokenBareKey, "b")
	findTokenValue(t, tokens, tokenBareKey, "c")
}

func TestDottedKeysQuoted(t *testing.T) {
	input := `a."b c".d = 1`
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findTokenValue(t, tokens, tokenBasicString, "b c")
	findTokenValue(t, tokens, tokenBareKey, "d")
}

func TestKeyEqualsString(t *testing.T) {
	input := `"key" = "value"`
	tokens := mustScan(t, input)
	// First basic string is the key, second is the value
	strs := []token{}
	for _, tok := range tokens {
		if tok.kind == tokenBasicString {
			strs = append(strs, tok)
		}
	}
	if len(strs) != 2 {
		t.Fatalf("expected 2 basic strings, got %d", len(strs))
	}
	if strs[0].value != "key" {
		t.Errorf("key got %q, want %q", strs[0].value, "key")
	}
	if strs[1].value != "value" {
		t.Errorf("value got %q, want %q", strs[1].value, "value")
	}
}

func TestLiteralStringAsKey(t *testing.T) {
	input := "'key' = 1"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenLiteralString, "key")
}

func TestStreamStartEnd(t *testing.T) {
	tokens := mustScan(t, "key = 1")
	if tokens[0].kind != tokenStreamStart {
		t.Errorf("first token should be STREAM-START, got %v", tokens[0].kind)
	}
	if tokens[len(tokens)-1].kind != tokenStreamEnd {
		t.Errorf("last token should be STREAM-END, got %v", tokens[len(tokens)-1].kind)
	}
}

func TestKeyValueSeparator(t *testing.T) {
	input := "key = 1"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenKeyValueSep)
}

func TestArrayOfInlineTables(t *testing.T) {
	input := "key = [{a = 1}, {b = 2}]"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenArrayStart)
	inlineStarts := 0
	inlineEnds := 0
	for _, tok := range tokens {
		if tok.kind == tokenInlineTableStart {
			inlineStarts++
		}
		if tok.kind == tokenInlineTableEnd {
			inlineEnds++
		}
	}
	if inlineStarts != 2 {
		t.Errorf("expected 2 inline table starts, got %d", inlineStarts)
	}
	if inlineEnds != 2 {
		t.Errorf("expected 2 inline table ends, got %d", inlineEnds)
	}
	findToken(t, tokens, tokenArrayEnd)
}

func TestHexEscapeXBoundary(t *testing.T) {
	// \x00 through \x7F
	input := `key = "\x00"`
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenBasicString)
	if tok.value != "\x00" {
		t.Errorf("got %q, want %q", tok.value, "\x00")
	}
}

func TestHexEscapeXFF(t *testing.T) {
	input := `key = "\xFF"`
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenBasicString)
	// \xFF in TOML is U+00FF (latin small letter y with diaeresis), encoded as UTF-8
	if tok.value != "ÿ" {
		t.Errorf("got %q, want %q", tok.value, "ÿ")
	}
}

func TestMultipleTableHeaders(t *testing.T) {
	input := "[a]\nkey1 = 1\n[b]\nkey2 = 2"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "a")
	findTokenValue(t, tokens, tokenBareKey, "b")
	findTokenValue(t, tokens, tokenBareKey, "key1")
	findTokenValue(t, tokens, tokenBareKey, "key2")
}

func TestTableThenArrayTable(t *testing.T) {
	input := "[a]\nkey = 1\n[[b]]\nval = 2"
	tokens := mustScan(t, input)
	findToken(t, tokens, tokenTableStart)
	findToken(t, tokens, tokenTableEnd)
	findToken(t, tokens, tokenArrayTableStart)
	findToken(t, tokens, tokenArrayTableEnd)
}

func TestComplexDocument(t *testing.T) {
	input := `# This is a TOML document
[database]
server = "192.168.1.1"
ports = [8001, 8001, 8002]
enabled = true

[servers.alpha]
ip = "10.0.0.1"
dc = "eqdc10"

[[products]]
name = "Hammer"
sku = 738594937
`
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenBareKey, "server")
	findTokenValue(t, tokens, tokenBasicString, "192.168.1.1")
	findTokenValue(t, tokens, tokenBareKey, "enabled")
	findTokenValue(t, tokens, tokenBoolean, "true")
	findTokenValue(t, tokens, tokenBareKey, "name")
	findTokenValue(t, tokens, tokenBasicString, "Hammer")
	findTokenValue(t, tokens, tokenInteger, "738594937")
}

func TestIsBareKeyChar(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'-', true},
		{'_', true},
		{' ', false},
		{'.', false},
		{'=', false},
		{'[', false},
		{']', false},
		{'"', false},
		{'\'', false},
		{'#', false},
		{'é', true},  // e with accent - Unicode letter
		{'世', true},  // 世 - CJK character
		{'̀', false},  // combining grave accent - not a letter or digit
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			got := isBareKeyChar(tt.r)
			if got != tt.want {
				t.Errorf("isBareKeyChar(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsControlChar(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{0x00, true},
		{0x08, true},
		{0x09, false},  // tab is excluded
		{0x0A, true},   // LF
		{0x1F, true},
		{0x20, false},  // space
		{0x7E, false},  // tilde
		{0x7F, true},   // DEL
		{'a', false},
	}
	for _, tt := range tests {
		got := isControlChar(tt.r)
		if got != tt.want {
			t.Errorf("isControlChar(%q / U+%04X) = %v, want %v", tt.r, tt.r, got, tt.want)
		}
	}
}

func TestStripUnderscores(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1_000", "1000"},
		{"1000", "1000"},
		{"_1_", "1"},
		{"___", ""},
		{"abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripUnderscores(tt.input)
			if got != tt.want {
				t.Errorf("stripUnderscores(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCheckLeadingZeros(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"0", false},
		{"1", false},
		{"10", false},
		{"01", true},
		{"00", true},
		{"+01", true},
		{"-01", true},
		{"+0", false},
		{"-0", false},
		{"+10", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := checkLeadingZeros(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkLeadingZeros(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeCRLF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no CR", "abc\ndef", "abc\ndef"},
		{"CRLF", "abc\r\ndef", "abc\ndef"},
		{"bare CR", "abc\rdef", "abc\ndef"},
		{"multiple CRLF", "a\r\nb\r\nc", "a\nb\nc"},
		{"mixed", "a\r\nb\rc\nd", "a\nb\nc\nd"},
		{"trailing CRLF", "abc\r\n", "abc\n"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeCRLF([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("normalizeCRLF(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"with BOM", append(bom, []byte("hello")...), "hello"},
		{"without BOM", []byte("hello"), "hello"},
		{"BOM only", bom, ""},
		{"partial BOM 1", []byte{0xEF}, "\xEF"},
		{"partial BOM 2", []byte{0xEF, 0xBB}, "\xEF\xBB"},
		{"empty", []byte{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripBOM(tt.input))
			if got != tt.want {
				t.Errorf("stripBOM(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestScanTokenPositions(t *testing.T) {
	input := "key = 42"
	tokens := mustScan(t, input)
	key := findTokenValue(t, tokens, tokenBareKey, "key")
	if key.pos.Line != 1 {
		t.Errorf("key line = %d, want 1", key.pos.Line)
	}
}

func TestScanMultipleLinesPositions(t *testing.T) {
	input := "a = 1\nb = 2"
	tokens := mustScan(t, input)
	b := findTokenValue(t, tokens, tokenBareKey, "b")
	if b.pos.Line != 2 {
		t.Errorf("b line = %d, want 2", b.pos.Line)
	}
}

func TestIntegerZero(t *testing.T) {
	input := "key = 0"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "0")
}

func TestIntegerPositiveZero(t *testing.T) {
	input := "key = +0"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "+0")
}

func TestIntegerNegativeZero(t *testing.T) {
	input := "key = -0"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "-0")
}

func TestArrayWithCommentsBetweenElements(t *testing.T) {
	// Comments within single-line arrays
	input := "key = [1, 2, 3] # comment"
	tokens := mustScan(t, input)
	findTokenValue(t, tokens, tokenInteger, "1")
	findTokenValue(t, tokens, tokenInteger, "2")
	findTokenValue(t, tokens, tokenInteger, "3")
	comments := 0
	for _, tok := range tokens {
		if tok.kind == tokenComment {
			comments++
		}
	}
	if comments != 1 {
		t.Errorf("expected 1 comment, got %d", comments)
	}
}

func TestUnicodeInBasicString(t *testing.T) {
	input := `key = "日本語"`
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenBasicString)
	if tok.value != "日本語" {
		t.Errorf("got %q, want %q", tok.value, "日本語")
	}
}

func TestUnicodeInLiteralString(t *testing.T) {
	input := "key = '日本語'"
	tokens := mustScan(t, input)
	tok := findToken(t, tokens, tokenLiteralString)
	if tok.value != "日本語" {
		t.Errorf("got %q, want %q", tok.value, "日本語")
	}
}
