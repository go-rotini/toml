package toml

type tokenKind int32

const (
	tokenStreamStart tokenKind = iota
	tokenStreamEnd
	tokenKeyValueSep
	tokenTableStart
	tokenTableEnd
	tokenArrayTableStart
	tokenArrayTableEnd
	tokenInlineTableStart
	tokenInlineTableEnd
	tokenArrayStart
	tokenArrayEnd
	tokenComma
	tokenDot
	tokenBareKey
	tokenBasicString
	tokenMultiLineBasicString
	tokenLiteralString
	tokenMultiLineLiteralString
	tokenInteger
	tokenFloat
	tokenBoolean
	tokenOffsetDateTime
	tokenLocalDateTime
	tokenLocalDate
	tokenLocalTime
	tokenComment
	tokenNewline
	tokenEOF
)

type token struct {
	kind      tokenKind
	start     int32 // offset in the normalized source where the token begins
	startLine int32 // line and column where the token begins
	startCol  int32
	value     string
	pos       Position // where the token ends; where it begins for a newline and for = . , { } and a value's [ ]
}

// startPos returns the position where the token begins, which errors report.
func (t token) startPos() Position {
	return Position{Line: int(t.startLine), Column: int(t.startCol), Offset: int(t.start)}
}

func (t token) String() string {
	switch t.kind {
	case tokenStreamStart:
		return "STREAM-START"
	case tokenStreamEnd:
		return "STREAM-END"
	case tokenKeyValueSep:
		return "KEY-VALUE-SEP(=)"
	case tokenTableStart:
		return "TABLE-START([)"
	case tokenTableEnd:
		return "TABLE-END(])"
	case tokenArrayTableStart:
		return "ARRAY-TABLE-START([[)"
	case tokenArrayTableEnd:
		return "ARRAY-TABLE-END(]])"
	case tokenInlineTableStart:
		return "INLINE-TABLE-START({)"
	case tokenInlineTableEnd:
		return "INLINE-TABLE-END(})"
	case tokenArrayStart:
		return "ARRAY-START([)"
	case tokenArrayEnd:
		return "ARRAY-END(])"
	case tokenComma:
		return "COMMA"
	case tokenDot:
		return "DOT"
	case tokenBareKey:
		return "BARE-KEY(" + t.value + ")"
	case tokenBasicString:
		return "BASIC-STRING(" + t.value + ")"
	case tokenMultiLineBasicString:
		return "MULTI-LINE-BASIC-STRING(" + t.value + ")"
	case tokenLiteralString:
		return "LITERAL-STRING(" + t.value + ")"
	case tokenMultiLineLiteralString:
		return "MULTI-LINE-LITERAL-STRING(" + t.value + ")"
	case tokenInteger:
		return "INTEGER(" + t.value + ")"
	case tokenFloat:
		return "FLOAT(" + t.value + ")"
	case tokenBoolean:
		return "BOOLEAN(" + t.value + ")"
	case tokenOffsetDateTime:
		return "OFFSET-DATETIME(" + t.value + ")"
	case tokenLocalDateTime:
		return "LOCAL-DATETIME(" + t.value + ")"
	case tokenLocalDate:
		return "LOCAL-DATE(" + t.value + ")"
	case tokenLocalTime:
		return "LOCAL-TIME(" + t.value + ")"
	case tokenComment:
		return "COMMENT(" + t.value + ")"
	case tokenNewline:
		return "NEWLINE"
	case tokenEOF:
		return "EOF"
	default:
		return "UNKNOWN"
	}
}

// describe names the token for an error message.
func (t token) describe() string {
	switch t.kind {
	case tokenNewline:
		return "end of line"
	case tokenEOF, tokenStreamEnd:
		return "end of input"
	case tokenComment:
		return "comment"
	case tokenKeyValueSep:
		return "'='"
	case tokenTableStart, tokenArrayStart:
		return "'['"
	case tokenTableEnd, tokenArrayEnd:
		return "']'"
	case tokenArrayTableStart:
		return "'[['"
	case tokenArrayTableEnd:
		return "']]'"
	case tokenInlineTableStart:
		return "'{'"
	case tokenInlineTableEnd:
		return "'}'"
	case tokenComma:
		return "','"
	case tokenDot:
		return "'.'"
	case tokenBareKey:
		return "key " + t.value
	case tokenBasicString, tokenMultiLineBasicString, tokenLiteralString, tokenMultiLineLiteralString:
		return "string"
	default:
		return "value " + t.value
	}
}
