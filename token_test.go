package toml

import (
	"strings"
	"testing"
)

func TestTokenStringAllKinds(t *testing.T) {
	tests := []struct {
		tok  token
		want string
	}{
		{token{kind: tokenStreamStart}, "STREAM-START"},
		{token{kind: tokenStreamEnd}, "STREAM-END"},
		{token{kind: tokenKeyValueSep}, "KEY-VALUE-SEP(=)"},
		{token{kind: tokenTableStart}, "TABLE-START([)"},
		{token{kind: tokenTableEnd}, "TABLE-END(])"},
		{token{kind: tokenArrayTableStart}, "ARRAY-TABLE-START([[)"},
		{token{kind: tokenArrayTableEnd}, "ARRAY-TABLE-END(]])"},
		{token{kind: tokenInlineTableStart}, "INLINE-TABLE-START({)"},
		{token{kind: tokenInlineTableEnd}, "INLINE-TABLE-END(})"},
		{token{kind: tokenArrayStart}, "ARRAY-START([)"},
		{token{kind: tokenArrayEnd}, "ARRAY-END(])"},
		{token{kind: tokenComma}, "COMMA"},
		{token{kind: tokenDot}, "DOT"},
		{token{kind: tokenBareKey, value: "name"}, "BARE-KEY(name)"},
		{token{kind: tokenBasicString, value: "hello"}, "BASIC-STRING(hello)"},
		{token{kind: tokenMultiLineBasicString, value: "ml"}, "MULTI-LINE-BASIC-STRING(ml)"},
		{token{kind: tokenLiteralString, value: "lit"}, "LITERAL-STRING(lit)"},
		{token{kind: tokenMultiLineLiteralString, value: "mll"}, "MULTI-LINE-LITERAL-STRING(mll)"},
		{token{kind: tokenInteger, value: "42"}, "INTEGER(42)"},
		{token{kind: tokenFloat, value: "3.14"}, "FLOAT(3.14)"},
		{token{kind: tokenBoolean, value: "true"}, "BOOLEAN(true)"},
		{token{kind: tokenOffsetDateTime, value: "2024-01-01T00:00:00Z"}, "OFFSET-DATETIME(2024-01-01T00:00:00Z)"},
		{token{kind: tokenLocalDateTime, value: "2024-01-01T00:00:00"}, "LOCAL-DATETIME(2024-01-01T00:00:00)"},
		{token{kind: tokenLocalDate, value: "2024-01-01"}, "LOCAL-DATE(2024-01-01)"},
		{token{kind: tokenLocalTime, value: "14:30:00"}, "LOCAL-TIME(14:30:00)"},
		{token{kind: tokenComment, value: " a comment"}, "COMMENT( a comment)"},
		{token{kind: tokenNewline}, "NEWLINE"},
		{token{kind: tokenEOF}, "EOF"},
		{token{kind: tokenError, value: "bad"}, "ERROR(bad)"},
		{token{kind: tokenKind(999)}, "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.tok.String()
		if got != tt.want {
			t.Errorf("token{kind: %d}.String() = %q, want %q", tt.tok.kind, got, tt.want)
		}
	}
}

func TestTokenStringContainsValue(t *testing.T) {
	tok := token{kind: tokenBareKey, value: "my_key"}
	s := tok.String()
	if !strings.Contains(s, "my_key") {
		t.Errorf("token.String() should contain value, got %q", s)
	}
}
