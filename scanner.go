package toml

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

type nestKind int

const (
	nestArray       nestKind = iota // inside [...]
	nestInlineTable                 // inside {...}
)

type scanner struct {
	src    []byte
	pos    int
	line   int
	col    int
	tokens []token

	atLineStart bool // [ at line start is a table header, not an array
	inValue     bool
	nestStack   []nestKind

	tokStart     int // offset in src where the token being scanned begins
	tokStartLine int // line and column where it begins
	tokStartCol  int
	bomLen       int   // bytes of byte-order mark removed from the input
	crlf         []int // offsets in src of each \n whose preceding \r was removed
	lineStarts   []int // offsets in src where each line begins, built on first use
	chunkRunes   []int // characters in src[:i*runeChunk], built with lineStarts
}

func newScanner(src []byte) (*scanner, error) {
	stripped := stripBOM(src)
	bomLen := len(src) - len(stripped)
	src, crlf, err := normalizeCRLF(stripped)
	if err != nil {
		if se, ok := errors.AsType[*SyntaxError](err); ok {
			se.Pos.Offset += bomLen
		}
		return nil, err
	}
	return &scanner{
		src:         src,
		line:        1,
		col:         1,
		atLineStart: true,
		bomLen:      bomLen,
		crlf:        crlf,
	}, nil
}

// bom is the UTF-8 byte-order mark, allowed only at the start of a document.
const bom = "\xef\xbb\xbf"

func stripBOM(src []byte) []byte {
	return bytes.TrimPrefix(src, []byte(bom))
}

// normalizeCRLF replaces each \r\n with \n. It also returns the offset in the
// result of every \n that lost its \r, so positions can be mapped back to the input.
func normalizeCRLF(src []byte) ([]byte, []int, error) {
	if !bytes.ContainsRune(src, '\r') {
		return src, nil, nil
	}
	buf := make([]byte, 0, len(src))
	var crlf []int
	line, lineStart := 1, 0
	for i := range src {
		if src[i] == '\r' {
			if i+1 < len(src) && src[i+1] == '\n' {
				crlf = append(crlf, len(buf))
				continue
			}
			return nil, nil, &SyntaxError{
				Message: "bare carriage return not allowed",
				Pos:     Position{Line: line, Column: utf8.RuneCount(buf[lineStart:]) + 1, Offset: i},
			}
		}
		buf = append(buf, src[i])
		if src[i] == '\n' {
			line++
			lineStart = len(buf)
		}
	}
	return buf, crlf, nil
}

// Valid reports whether data is valid TOML.
func Valid(data []byte) bool {
	_, _, err := parseDocument(data, 0, 0)
	return err == nil
}

func (s *scanner) scan() ([]token, error) {
	s.emit(tokenStreamStart, "")
	for !s.atEnd() {
		if err := s.scanNext(); err != nil {
			return nil, err
		}
	}
	s.markStart()
	s.emit(tokenStreamEnd, "")
	return s.tokens, nil
}

func (s *scanner) scanNext() error {
	s.skipWhitespace()
	if s.atEnd() {
		return nil
	}
	s.markStart()

	ch := s.peek()

	switch {
	case ch == '#':
		return s.scanComment()
	case ch == '\n':
		s.emit(tokenNewline, "")
		s.advance()
		s.atLineStart = true
		if len(s.nestStack) == 0 {
			s.inValue = false
		}
		return nil
	case ch == '=' && !s.inValue:
		s.emit(tokenKeyValueSep, "")
		s.advance()
		s.inValue = true
		return nil
	case ch == '.' && !s.inValue:
		s.emit(tokenDot, "")
		s.advance()
		return nil
	case ch == ',' && (s.inValue || len(s.nestStack) > 0):
		s.emit(tokenComma, "")
		s.advance()
		if len(s.nestStack) > 0 && s.nestStack[len(s.nestStack)-1] == nestInlineTable {
			s.inValue = false
		}
		return nil
	case ch == '{' && s.inValue:
		s.emit(tokenInlineTableStart, "")
		s.advance()
		s.nestStack = append(s.nestStack, nestInlineTable)
		s.inValue = false
		return nil
	case ch == '}' && len(s.nestStack) > 0:
		s.emit(tokenInlineTableEnd, "")
		s.advance()
		s.nestStack = s.nestStack[:len(s.nestStack)-1]
		s.inValue = true
		return nil
	case ch == '[' && s.inValue:
		s.emit(tokenArrayStart, "")
		s.advance()
		s.nestStack = append(s.nestStack, nestArray)
		return nil
	case ch == ']' && s.inValue && len(s.nestStack) > 0:
		s.emit(tokenArrayEnd, "")
		s.advance()
		s.nestStack = s.nestStack[:len(s.nestStack)-1]
		s.inValue = true
		return nil
	case ch == '[' && s.atLineStart && !s.inValue && len(s.nestStack) == 0:
		return s.scanTableHeader()
	case ch == '"':
		return s.scanBasicString()
	case ch == '\'':
		return s.scanLiteralString()
	default:
		if s.inValue {
			return s.scanValue()
		}
		return s.scanKey()
	}
}

func (s *scanner) scanTableHeader() error {
	s.advance() // consume first [
	if !s.atEnd() && s.peek() == '[' {
		s.advance() // consume second [
		s.emit(tokenArrayTableStart, "")
		s.skipWhitespace()
		if err := s.scanTableKey(); err != nil {
			return err
		}
		s.skipWhitespace()
		s.markStart()
		if s.atEnd() || s.peek() != ']' {
			return s.syntaxError("expected ]]")
		}
		s.advance()
		if s.atEnd() || s.peek() != ']' {
			return s.syntaxError("expected ]]")
		}
		s.advance()
		s.emit(tokenArrayTableEnd, "")
	} else {
		s.emit(tokenTableStart, "")
		s.skipWhitespace()
		if err := s.scanTableKey(); err != nil {
			return err
		}
		s.skipWhitespace()
		s.markStart()
		if s.atEnd() || s.peek() != ']' {
			return s.syntaxError("expected ]")
		}
		s.advance()
		s.emit(tokenTableEnd, "")
	}
	s.atLineStart = false
	s.inValue = false
	return nil
}

func (s *scanner) scanTableKey() error {
	for {
		if s.atEnd() {
			return s.syntaxError("expected key")
		}
		s.markStart()
		ch := s.peek()
		switch ch {
		case '"':
			if err := s.scanBasicString(); err != nil {
				return err
			}
		case '\'':
			if err := s.scanLiteralString(); err != nil {
				return err
			}
		default:
			if err := s.scanBareKey(); err != nil {
				return err
			}
		}
		s.skipWhitespace()
		if s.atEnd() {
			return nil
		}
		if s.peek() == '.' {
			s.markStart()
			s.emit(tokenDot, "")
			s.advance()
			s.skipWhitespace()
			if s.atEnd() {
				return s.syntaxError("expected key after dot")
			}
		} else {
			return nil
		}
	}
}

func (s *scanner) scanKey() error {
	ch := s.peek()
	switch ch {
	case '"':
		return s.scanBasicString()
	case '\'':
		return s.scanLiteralString()
	default:
		return s.scanBareKey()
	}
}

func (s *scanner) scanBareKey() error {
	start := s.pos
	for !s.atEnd() {
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8")
		}
		if isBareKeyChar(r) {
			s.pos += size
			s.col++
		} else {
			break
		}
	}
	if s.pos == start {
		return s.syntaxError(fmt.Sprintf("unexpected character %q", s.peek()))
	}
	s.emit(tokenBareKey, string(s.src[start:s.pos]))
	s.atLineStart = false
	return nil
}

func isBareKeyChar(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_'
}

func (s *scanner) scanBasicString() error {
	s.advance() // consume opening "
	if s.pos+1 < len(s.src) && s.src[s.pos] == '"' && s.src[s.pos+1] == '"' {
		s.advance() // consume second "
		s.advance() // consume third "
		return s.scanMultiLineBasicString()
	}
	return s.scanSingleLineBasicString()
}

func (s *scanner) scanSingleLineBasicString() error {
	var buf []byte
	for !s.atEnd() {
		ch := s.peek()
		if ch == '\n' {
			return s.syntaxError("newline in basic string")
		}
		if ch == '"' {
			s.advance()
			kind := tokenBasicString
			if !s.inValue {
				kind = tokenBasicString
			}
			s.emit(kind, string(buf))
			s.atLineStart = false
			return nil
		}
		if ch == '\\' {
			r, err := s.scanEscape()
			if err != nil {
				return err
			}
			buf = utf8.AppendRune(buf, r)
			continue
		}
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8 in string")
		}
		if isControlChar(r) && r != '\t' {
			return s.syntaxError(fmt.Sprintf("control character U+%04X in basic string", r))
		}
		buf = append(buf, s.src[s.pos:s.pos+size]...)
		s.pos += size
		s.col++
	}
	return s.syntaxError("unterminated basic string")
}

func (s *scanner) scanMultiLineBasicString() error {
	if !s.atEnd() && s.peek() == '\n' {
		s.advance()
	}
	var buf []byte
	for !s.atEnd() {
		ch := s.peek()
		if ch == '"' {
			count := s.countConsecutive('"')
			if count >= 3 {
				extra := count - 3
				if extra > 2 {
					return s.syntaxError("too many quotes in multi-line basic string")
				}
				for range extra {
					buf = append(buf, '"')
				}
				s.pos += count
				s.col += count
				s.emit(tokenMultiLineBasicString, string(buf))
				s.atLineStart = false
				return nil
			}
			for range count {
				buf = append(buf, '"')
				s.advance()
			}
			continue
		}
		if ch == '\\' {
			saved := s.pos
			savedCol := s.col
			s.advance() // consume backslash
			if !s.atEnd() && (s.peek() == '\n' || s.peek() == ' ' || s.peek() == '\t') {
				if ok := s.trimLineContinuation(); !ok {
					return s.syntaxError("invalid escape: backslash followed by whitespace without newline")
				}
				continue
			}
			s.pos = saved
			s.col = savedCol
			r, err := s.scanEscape()
			if err != nil {
				return err
			}
			buf = utf8.AppendRune(buf, r)
			continue
		}
		if ch == '\n' {
			buf = append(buf, '\n')
			s.advance()
			continue
		}
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8 in string")
		}
		if isControlChar(r) && r != '\t' {
			return s.syntaxError(fmt.Sprintf("control character U+%04X in multi-line basic string", r))
		}
		buf = append(buf, s.src[s.pos:s.pos+size]...)
		s.pos += size
		s.col++
	}
	return s.syntaxError("unterminated multi-line basic string")
}

func (s *scanner) trimLineContinuation() bool {
	foundNewline := false
	for !s.atEnd() {
		ch := s.peek()
		if ch == '\n' {
			foundNewline = true
			s.advance()
			continue
		}
		if ch == ' ' || ch == '\t' {
			s.advance()
			continue
		}
		break
	}
	return foundNewline
}

func (s *scanner) scanLiteralString() error {
	s.advance() // consume opening '
	if s.pos+1 < len(s.src) && s.src[s.pos] == '\'' && s.src[s.pos+1] == '\'' {
		s.advance() // consume second '
		s.advance() // consume third '
		return s.scanMultiLineLiteralString()
	}
	return s.scanSingleLineLiteralString()
}

func (s *scanner) scanSingleLineLiteralString() error {
	start := s.pos
	for !s.atEnd() {
		ch := s.peek()
		if ch == '\n' {
			return s.syntaxError("newline in literal string")
		}
		if ch == '\'' {
			val := string(s.src[start:s.pos])
			s.advance()
			s.emit(tokenLiteralString, val)
			s.atLineStart = false
			return nil
		}
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8 in string")
		}
		if isControlChar(r) && r != '\t' {
			return s.syntaxError(fmt.Sprintf("control character U+%04X in literal string", r))
		}
		s.pos += size
		s.col++
	}
	return s.syntaxError("unterminated literal string")
}

func (s *scanner) scanMultiLineLiteralString() error {
	if !s.atEnd() && s.peek() == '\n' {
		s.advance()
	}
	var buf []byte
	for !s.atEnd() {
		ch := s.peek()
		if ch == '\'' {
			count := s.countConsecutive('\'')
			if count >= 3 {
				extra := count - 3
				if extra > 2 {
					return s.syntaxError("too many quotes in multi-line literal string")
				}
				for range extra {
					buf = append(buf, '\'')
				}
				s.pos += count
				s.col += count
				s.emit(tokenMultiLineLiteralString, string(buf))
				s.atLineStart = false
				return nil
			}
			for range count {
				buf = append(buf, '\'')
				s.advance()
			}
			continue
		}
		if ch == '\n' {
			buf = append(buf, '\n')
			s.advance()
			continue
		}
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8 in string")
		}
		if isControlChar(r) && r != '\t' {
			return s.syntaxError(fmt.Sprintf("control character U+%04X in multi-line literal string", r))
		}
		buf = append(buf, s.src[s.pos:s.pos+size]...)
		s.pos += size
		s.col++
	}
	return s.syntaxError("unterminated multi-line literal string")
}

func (s *scanner) scanEscape() (rune, error) {
	s.advance() // consume backslash
	if s.atEnd() {
		return 0, s.syntaxError("unterminated escape sequence")
	}
	ch := s.peek()
	s.advance()
	switch ch {
	case 'b':
		return '\b', nil
	case 't':
		return '\t', nil
	case 'n':
		return '\n', nil
	case 'f':
		return '\f', nil
	case 'r':
		return '\r', nil
	case '"':
		return '"', nil
	case '\\':
		return '\\', nil
	case 'e':
		return 0x1B, nil
	case 'x':
		return s.scanHexEscape(2)
	case 'u':
		return s.scanHexEscape(4)
	case 'U':
		return s.scanHexEscape(8)
	default:
		return 0, s.syntaxError(fmt.Sprintf("invalid escape sequence \\%c", ch))
	}
}

func (s *scanner) scanHexEscape(n int) (rune, error) {
	var val rune
	for range n {
		if s.atEnd() {
			return 0, s.syntaxError("incomplete Unicode escape")
		}
		ch := s.peek()
		s.advance()
		d := hexVal(ch)
		if d < 0 {
			return 0, s.syntaxError(fmt.Sprintf("invalid hex digit %q in escape", ch))
		}
		val = val*16 + rune(d)
	}
	if val >= 0xD800 && val <= 0xDFFF {
		return 0, s.syntaxError("Unicode surrogate in escape sequence")
	}
	if !utf8.ValidRune(val) {
		return 0, s.syntaxError("invalid Unicode codepoint in escape sequence")
	}
	return val, nil
}

func hexVal(ch byte) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10
	default:
		return -1
	}
}

func (s *scanner) scanValue() error {
	ch := s.peek()
	switch {
	case ch == '"':
		return s.scanBasicString()
	case ch == '\'':
		return s.scanLiteralString()
	case ch == 't', ch == 'f':
		return s.scanBoolean()
	case ch == '+', ch == '-':
		return s.scanSignedNumber()
	case ch >= '0' && ch <= '9':
		return s.scanNumberOrDateTime()
	case ch == 'i', ch == 'n':
		return s.scanSpecialFloat()
	default:
		return s.syntaxError(fmt.Sprintf("unexpected character %q in value", ch))
	}
}

func (s *scanner) scanBoolean() error {
	if s.matchWord("true") {
		s.emit(tokenBoolean, "true")
		return nil
	}
	if s.matchWord("false") {
		s.emit(tokenBoolean, "false")
		return nil
	}
	return s.syntaxError("invalid value")
}

func (s *scanner) scanSpecialFloat() error {
	if s.matchWord("inf") {
		s.emit(tokenFloat, "inf")
		return nil
	}
	if s.matchWord("nan") {
		s.emit(tokenFloat, "nan")
		return nil
	}
	return s.syntaxError("invalid value")
}

func (s *scanner) scanSignedNumber() error {
	sign := s.peek()
	s.advance()
	if s.atEnd() {
		return s.syntaxError("unexpected end after sign")
	}
	ch := s.peek()
	if ch == 'i' {
		if s.matchWord("inf") {
			s.emit(tokenFloat, string(sign)+"inf")
			return nil
		}
		return s.syntaxError("invalid value")
	}
	if ch == 'n' {
		if s.matchWord("nan") {
			s.emit(tokenFloat, string(sign)+"nan")
			return nil
		}
		return s.syntaxError("invalid value")
	}
	if ch < '0' || ch > '9' {
		return s.syntaxError(fmt.Sprintf("unexpected character %q after sign", ch))
	}
	return s.scanNumberFrom(s.pos - 1)
}

func (s *scanner) scanNumberOrDateTime() error {
	if s.peek() == '0' && s.pos+1 < len(s.src) {
		next := s.src[s.pos+1]
		switch next {
		case 'x':
			return s.scanPrefixedInt(16, "0x")
		case 'o':
			return s.scanPrefixedInt(8, "0o")
		case 'b':
			return s.scanPrefixedInt(2, "0b")
		case 'X', 'O', 'B':
			return s.syntaxError(fmt.Sprintf("invalid prefix 0%c, must be lowercase", next))
		}
	}
	return s.scanNumberFrom(s.pos)
}

func (s *scanner) scanPrefixedInt(base int, prefix string) error {
	start := s.pos
	s.advance() // consume 0
	s.advance() // consume prefix letter
	if s.atEnd() || !isDigitForBase(s.peek(), base) {
		return s.syntaxError(fmt.Sprintf("expected digit after %s", prefix))
	}
	for !s.atEnd() {
		ch := s.peek()
		if ch == '_' {
			if s.pos+1 >= len(s.src) || !isDigitForBase(s.src[s.pos+1], base) {
				return s.syntaxError("underscore must be between digits")
			}
			s.advance()
			continue
		}
		if !isDigitForBase(ch, base) {
			break
		}
		s.advance()
	}
	text := string(s.src[start:s.pos])
	if v, err := strconv.ParseUint(stripUnderscores(text[2:]), base, 64); err != nil || v > math.MaxInt64 {
		return s.syntaxError("integer out of range")
	}
	s.emit(tokenInteger, text)
	return nil
}

func isDigitForBase(ch byte, base int) bool {
	switch base {
	case 2:
		return ch == '0' || ch == '1'
	case 8:
		return ch >= '0' && ch <= '7'
	case 16:
		return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
	default:
		return ch >= '0' && ch <= '9'
	}
}

func (s *scanner) scanNumberFrom(start int) error {
	for !s.atEnd() {
		ch := s.peek()
		if ch >= '0' && ch <= '9' {
			s.advance()
			continue
		}
		if ch == '_' {
			if s.pos+1 >= len(s.src) || s.src[s.pos+1] < '0' || s.src[s.pos+1] > '9' {
				return s.syntaxError("underscore must be between digits")
			}
			s.advance()
			continue
		}
		break
	}

	intPart := string(s.src[start:s.pos])
	digitsOnly := stripUnderscores(intPart)

	if s.atEnd() {
		return s.emitDecimal(intPart, digitsOnly)
	}

	ch := s.peek()
	if ch == '-' && isDigits(intPart, 4) {
		return s.scanDateTime(start)
	}
	if ch == ':' && isDigits(intPart, 2) {
		return s.scanLocalTime(start)
	}

	if ch == '.' {
		if err := checkLeadingZeros(digitsOnly); err != nil {
			return s.syntaxError(err.Error())
		}
		s.advance()
		if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
			return s.syntaxError("expected digit after decimal point")
		}
		s.scanDecimalDigits()
		if !s.atEnd() && (s.peek() == 'e' || s.peek() == 'E') {
			if err := s.scanExponent(); err != nil {
				return err
			}
		}
		return s.emitFloat(string(s.src[start:s.pos]))
	}

	if ch == 'e' || ch == 'E' {
		if err := checkLeadingZeros(digitsOnly); err != nil {
			return s.syntaxError(err.Error())
		}
		if err := s.scanExponent(); err != nil {
			return err
		}
		return s.emitFloat(string(s.src[start:s.pos]))
	}

	return s.emitDecimal(intPart, digitsOnly)
}

// emitFloat emits a float token, rejecting values too large for a 64-bit float.
func (s *scanner) emitFloat(text string) error {
	if _, err := strconv.ParseFloat(stripUnderscores(text), 64); err != nil {
		return s.syntaxError("float out of range")
	}
	s.emit(tokenFloat, text)
	return nil
}

// emitDecimal emits a decimal integer token, rejecting leading zeros and
// values outside the signed 64-bit range.
func (s *scanner) emitDecimal(text, digits string) error {
	if err := checkLeadingZeros(digits); err != nil {
		return s.syntaxError(err.Error())
	}
	if _, err := strconv.ParseInt(digits, 10, 64); err != nil {
		return s.syntaxError("integer out of range")
	}
	s.emit(tokenInteger, text)
	return nil
}

// isDigits reports whether s is exactly n decimal digits.
func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// twoDigits returns the value of the two decimal digits at src[i:i+2].
func (s *scanner) twoDigits(i int) int {
	return int(s.src[i]-'0')*10 + int(s.src[i+1]-'0')
}

// checkTime validates the hour, minute and, when present, second of a time
// whose hour starts at src[i]. A second of 60 is a leap second.
func (s *scanner) checkTime(i int, hasSeconds bool) error {
	if s.twoDigits(i) > 23 {
		return s.syntaxError("hour out of range")
	}
	if s.twoDigits(i+3) > 59 {
		return s.syntaxError("minute out of range")
	}
	if hasSeconds && s.twoDigits(i+6) > 60 {
		return s.syntaxError("second out of range")
	}
	return nil
}

func (s *scanner) scanExponent() error {
	s.advance() // consume e/E
	if !s.atEnd() && (s.peek() == '+' || s.peek() == '-') {
		s.advance()
	}
	if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
		return s.syntaxError("expected digit in exponent")
	}
	s.scanDecimalDigits()
	return nil
}

func (s *scanner) scanDecimalDigits() {
	for !s.atEnd() {
		ch := s.peek()
		if ch >= '0' && ch <= '9' {
			s.advance()
			continue
		}
		if ch == '_' {
			if s.pos+1 < len(s.src) && s.src[s.pos+1] >= '0' && s.src[s.pos+1] <= '9' {
				s.advance()
				continue
			}
		}
		break
	}
}

// scanDateTime parses date, local-datetime, and offset-datetime tokens.
// Entry: the year (4 digits) has already been scanned.
func (s *scanner) scanDateTime(start int) error {
	if !s.expect('-') {
		return s.syntaxError("expected '-' in date")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	if !s.expect('-') {
		return s.syntaxError("expected '-' in date")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	year := s.twoDigits(start)*100 + s.twoDigits(start+2)
	month, day := s.twoDigits(start+5), s.twoDigits(start+8)
	if month < 1 || month > 12 {
		return s.syntaxError("month out of range")
	}
	if day < 1 || day > daysIn(month, year) {
		return s.syntaxError("day out of range")
	}

	if s.atEnd() || (s.peek() != 'T' && s.peek() != 't' && s.peek() != ' ') {
		s.emit(tokenLocalDate, string(s.src[start:s.pos]))
		return nil
	}

	if s.peek() == ' ' {
		if s.pos+1 >= len(s.src) || s.src[s.pos+1] < '0' || s.src[s.pos+1] > '9' {
			s.emit(tokenLocalDate, string(s.src[start:s.pos]))
			return nil
		}
	}

	s.advance()

	timeStart := s.pos
	if err := s.expectDigits(2); err != nil {
		return err
	}
	if err := s.scanTimeRest(timeStart); err != nil {
		return err
	}

	if s.atEnd() {
		s.emit(tokenLocalDateTime, string(s.src[start:s.pos]))
		return nil
	}

	ch := s.peek()
	if ch == 'Z' || ch == 'z' {
		s.advance()
		s.emit(tokenOffsetDateTime, string(s.src[start:s.pos]))
		return nil
	}
	if ch == '+' || ch == '-' {
		offStart := s.pos
		s.advance()
		if err := s.expectDigits(2); err != nil {
			return err
		}
		offHour := (s.src[offStart+1]-'0')*10 + (s.src[offStart+2] - '0')
		if offHour > 23 {
			return s.syntaxError("timezone offset hour out of range")
		}
		if !s.expect(':') {
			return s.syntaxError("expected ':' in timezone offset")
		}
		if err := s.expectDigits(2); err != nil {
			return err
		}
		offMin := (s.src[s.pos-2]-'0')*10 + (s.src[s.pos-1] - '0')
		if offMin > 59 {
			return s.syntaxError("timezone offset minute out of range")
		}
		s.emit(tokenOffsetDateTime, string(s.src[start:s.pos]))
		return nil
	}

	s.emit(tokenLocalDateTime, string(s.src[start:s.pos]))
	return nil
}

func (s *scanner) scanLocalTime(start int) error {
	if err := s.scanTimeRest(start); err != nil {
		return err
	}
	s.emit(tokenLocalTime, string(s.src[start:s.pos]))
	return nil
}

// scanTimeRest scans a time from just after its two hour digits, which
// begin at start, and validates it.
func (s *scanner) scanTimeRest(start int) error {
	if !s.expect(':') {
		return s.syntaxError("expected ':' in time")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	hasSeconds := false
	if !s.atEnd() && s.peek() == ':' {
		hasSeconds = true
		s.advance()
		if err := s.expectDigits(2); err != nil {
			return err
		}

		if !s.atEnd() && s.peek() == '.' {
			s.advance()
			if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
				return s.syntaxError("expected digit in fractional seconds")
			}
			for !s.atEnd() && s.peek() >= '0' && s.peek() <= '9' {
				s.advance()
			}
		}
	}
	return s.checkTime(start, hasSeconds)
}

// daysIn returns the number of days in month of year.
func daysIn(month, year int) int {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func (s *scanner) scanComment() error {
	s.advance() // consume #
	start := s.pos
	for !s.atEnd() && s.peek() != '\n' {
		r, size := utf8.DecodeRune(s.src[s.pos:])
		if r == utf8.RuneError && size <= 1 {
			return s.syntaxError("invalid UTF-8 in comment")
		}
		if isControlChar(r) && r != '\t' {
			return s.syntaxError(fmt.Sprintf("control character U+%04X in comment", r))
		}
		s.pos += size
		s.col++
	}
	s.emit(tokenComment, string(s.src[start:s.pos]))
	return nil
}

func (s *scanner) position() Position {
	return Position{Line: s.line, Column: s.col, Offset: s.pos}
}

func (s *scanner) emit(kind tokenKind, value string) {
	s.tokens = append(s.tokens, token{kind: kind, value: value, pos: s.position(), start: int32(s.tokStart), startLine: int32(s.tokStartLine), startCol: int32(s.tokStartCol)})
}

func (s *scanner) atEnd() bool {
	return s.pos >= len(s.src)
}

func (s *scanner) peek() byte {
	return s.src[s.pos]
}

func (s *scanner) advance() {
	if s.pos < len(s.src) {
		if s.src[s.pos] == '\n' {
			s.line++
			s.col = 1
		} else {
			s.col++
		}
		s.pos++
	}
}

func (s *scanner) skipWhitespace() {
	for !s.atEnd() {
		ch := s.peek()
		if ch == ' ' || ch == '\t' {
			s.advance()
		} else {
			return
		}
	}
}

func (s *scanner) countConsecutive(ch byte) int {
	count := 0
	for s.pos+count < len(s.src) && s.src[s.pos+count] == ch {
		count++
	}
	return count
}

func (s *scanner) matchWord(word string) bool {
	end := s.pos + len(word)
	if end > len(s.src) {
		return false
	}
	if string(s.src[s.pos:end]) != word {
		return false
	}
	if end < len(s.src) {
		r, _ := utf8.DecodeRune(s.src[end:])
		if isBareKeyChar(r) {
			return false
		}
	}
	for range len(word) {
		s.advance()
	}
	return true
}

func (s *scanner) expect(ch byte) bool {
	if s.atEnd() || s.peek() != ch {
		return false
	}
	s.advance()
	return true
}

func (s *scanner) expectDigits(n int) error {
	for range n {
		if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
			return s.syntaxError("expected digit")
		}
		s.advance()
	}
	return nil
}

func (s *scanner) syntaxError(msg string) error {
	return &SyntaxError{
		Message: msg,
		Pos:     s.position(),
	}
}

func isControlChar(r rune) bool {
	return (r >= 0x00 && r <= 0x08) || (r >= 0x0A && r <= 0x1F) || r == 0x7F
}

func checkLeadingZeros(digits string) error {
	d := digits
	if d != "" && (d[0] == '+' || d[0] == '-') {
		d = d[1:]
	}
	if len(d) > 1 && d[0] == '0' {
		return errLeadingZeros
	}
	return nil
}

func stripUnderscores(s string) string {
	n := 0
	for i := range len(s) {
		if s[i] != '_' {
			n++
		}
	}
	if n == len(s) {
		return s
	}
	buf := make([]byte, 0, n)
	for i := range len(s) {
		if s[i] != '_' {
			buf = append(buf, s[i])
		}
	}
	return string(buf)
}

// markStart records that the next token begins at the current position.
func (s *scanner) markStart() {
	s.tokStart = s.pos
	s.tokStartLine = s.line
	s.tokStartCol = s.col
}
