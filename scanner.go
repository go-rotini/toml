package toml

import (
	"bytes"
	"fmt"
	"unicode"
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

	// atLineStart tracks whether we are at the beginning of a logical line
	// (after newline + optional whitespace). This determines whether [ is a
	// table header or an array value.
	atLineStart bool

	// inValue tracks whether we are in the value position of a key = value
	// pair. Resets at newline boundaries.
	inValue bool

	// nestStack tracks the type of each nesting level (array vs inline table).
	nestStack []nestKind
}

func newScanner(src []byte) *scanner {
	src = stripBOM(src)
	src = normalizeCRLF(src)
	return &scanner{
		src:         src,
		line:        1,
		col:         1,
		atLineStart: true,
	}
}

func stripBOM(src []byte) []byte {
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		return src[3:]
	}
	return src
}

func normalizeCRLF(src []byte) []byte {
	if !bytes.ContainsRune(src, '\r') {
		return src
	}
	buf := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '\r' {
			if i+1 < len(src) && src[i+1] == '\n' {
				continue
			}
			buf = append(buf, '\n')
		} else {
			buf = append(buf, src[i])
		}
	}
	return buf
}

// Valid reports whether data is valid TOML.
func Valid(data []byte) bool {
	s := newScanner(data)
	tokens, err := s.scan()
	if err != nil {
		return false
	}
	p := newParser(tokens)
	_, err = p.parse()
	return err == nil
}

func (s *scanner) scan() ([]token, error) {
	s.emit(tokenStreamStart, "")
	for !s.atEnd() {
		if err := s.scanNext(); err != nil {
			return nil, err
		}
	}
	s.emit(tokenStreamEnd, "")
	return s.tokens, nil
}

func (s *scanner) scanNext() error {
	s.skipWhitespace()
	if s.atEnd() {
		return nil
	}

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
		// After comma in inline table → key position; in array → value position
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
		ch := s.peek()
		switch {
		case ch == '"':
			if err := s.scanBasicString(); err != nil {
				return err
			}
		case ch == '\'':
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
	switch {
	case ch == '"':
		return s.scanBasicString()
	case ch == '\'':
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
			s.col += 1
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
	if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
		return true
	}
	// TOML 1.1.0: Unicode letters and digits
	if r > 127 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		return true
	}
	return false
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
	// trim immediate newline after opening """
	if !s.atEnd() && s.peek() == '\n' {
		s.advance()
	}
	var buf []byte
	for !s.atEnd() {
		ch := s.peek()
		if ch == '"' {
			count := s.countConsecutive('"')
			if count >= 3 {
				// Allow up to 2 extra quotes before closing delimiter
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
			// Check for line-ending backslash
			saved := s.pos
			s.advance() // consume backslash
			if !s.atEnd() && (s.peek() == '\n' || s.peek() == ' ' || s.peek() == '\t') {
				s.trimLineContinuation()
				continue
			}
			s.pos = saved
			s.col-- // restore
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

func (s *scanner) trimLineContinuation() {
	for !s.atEnd() {
		ch := s.peek()
		if ch == '\n' {
			s.advance()
			continue
		}
		if ch == ' ' || ch == '\t' {
			s.advance()
			continue
		}
		break
	}
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
	// trim immediate newline after opening '''
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
	return s.scanNumberFrom(s.pos-1, true)
}

func (s *scanner) scanNumberOrDateTime() error {
	if s.peek() == '0' && s.pos+1 < len(s.src) {
		next := s.src[s.pos+1]
		switch next {
		case 'x', 'X':
			return s.scanPrefixedInt(16, "0x")
		case 'o', 'O':
			return s.scanPrefixedInt(8, "0o")
		case 'b', 'B':
			return s.scanPrefixedInt(2, "0b")
		}
	}
	return s.scanNumberFrom(s.pos, false)
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
	s.emit(tokenInteger, string(s.src[start:s.pos]))
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

func (s *scanner) scanNumberFrom(start int, hasSigned bool) error {
	_ = hasSigned
	// Scan decimal digits
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
		if err := checkLeadingZeros(digitsOnly); err != nil {
			return s.syntaxError(err.Error())
		}
		s.emit(tokenInteger, intPart)
		return nil
	}

	ch := s.peek()
	if ch == '-' && len(digitsOnly) == 4 || (len(digitsOnly) == 5 && (digitsOnly[0] == '+' || digitsOnly[0] == '-')) {
		return s.scanDateTime(start)
	}
	if ch == ':' && len(digitsOnly) == 2 {
		return s.scanLocalTime(start)
	}

	// Float with decimal or exponent
	if ch == '.' {
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
		s.emit(tokenFloat, string(s.src[start:s.pos]))
		return nil
	}

	if ch == 'e' || ch == 'E' {
		if err := s.scanExponent(); err != nil {
			return err
		}
		s.emit(tokenFloat, string(s.src[start:s.pos]))
		return nil
	}

	if err := checkLeadingZeros(digitsOnly); err != nil {
		return s.syntaxError(err.Error())
	}
	s.emit(tokenInteger, string(s.src[start:s.pos]))
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

func (s *scanner) scanDateTime(start int) error {
	// We've already scanned the year (4 digits). Now scan the rest.
	// Possible formats:
	//   1979-05-27 (local date)
	//   1979-05-27T07:32:00 (local datetime)
	//   1979-05-27T07:32:00Z (offset datetime)
	//   1979-05-27T07:32:00+00:00 (offset datetime)
	//   1979-05-27 07:32:00Z (space delimiter)

	// Consume -MM-DD
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

	// Check for time portion
	if s.atEnd() || (s.peek() != 'T' && s.peek() != 't' && s.peek() != ' ') {
		s.emit(tokenLocalDate, string(s.src[start:s.pos]))
		return nil
	}

	// Check if space-delimited: only valid if next non-space is a digit (time)
	if s.peek() == ' ' {
		if s.pos+1 >= len(s.src) || s.src[s.pos+1] < '0' || s.src[s.pos+1] > '9' {
			s.emit(tokenLocalDate, string(s.src[start:s.pos]))
			return nil
		}
	}

	s.advance() // consume T or space

	// Scan time: HH:MM:SS
	if err := s.expectDigits(2); err != nil {
		return err
	}
	if !s.expect(':') {
		return s.syntaxError("expected ':' in time")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	if !s.expect(':') {
		return s.syntaxError("expected ':' in time")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}

	// Fractional seconds
	if !s.atEnd() && s.peek() == '.' {
		s.advance()
		if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
			return s.syntaxError("expected digit in fractional seconds")
		}
		for !s.atEnd() && s.peek() >= '0' && s.peek() <= '9' {
			s.advance()
		}
	}

	// Check for timezone offset
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
		s.advance()
		if err := s.expectDigits(2); err != nil {
			return err
		}
		if !s.expect(':') {
			return s.syntaxError("expected ':' in timezone offset")
		}
		if err := s.expectDigits(2); err != nil {
			return err
		}
		s.emit(tokenOffsetDateTime, string(s.src[start:s.pos]))
		return nil
	}

	s.emit(tokenLocalDateTime, string(s.src[start:s.pos]))
	return nil
}

func (s *scanner) scanLocalTime(start int) error {
	// We've scanned HH, now scan :MM:SS[.fractional]
	if !s.expect(':') {
		return s.syntaxError("expected ':' in time")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	if !s.expect(':') {
		return s.syntaxError("expected ':' in time")
	}
	if err := s.expectDigits(2); err != nil {
		return err
	}
	// Fractional seconds
	if !s.atEnd() && s.peek() == '.' {
		s.advance()
		if s.atEnd() || s.peek() < '0' || s.peek() > '9' {
			return s.syntaxError("expected digit in fractional seconds")
		}
		for !s.atEnd() && s.peek() >= '0' && s.peek() <= '9' {
			s.advance()
		}
	}
	s.emit(tokenLocalTime, string(s.src[start:s.pos]))
	return nil
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

// Helper methods

func (s *scanner) position() Position {
	return Position{Line: s.line, Column: s.col, Offset: s.pos}
}

func (s *scanner) emit(kind tokenKind, value string) {
	s.tokens = append(s.tokens, token{kind: kind, value: value, pos: s.position()})
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
	// Ensure the word is not followed by a bare-key character
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
	if len(d) > 0 && (d[0] == '+' || d[0] == '-') {
		d = d[1:]
	}
	if len(d) > 1 && d[0] == '0' {
		return fmt.Errorf("leading zeros are not allowed in decimal integers")
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
