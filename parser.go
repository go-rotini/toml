package toml

import (
	"fmt"
	"strings"
)

type parser struct {
	tokens         []token
	pos            int
	root           *node
	current        *node
	implicit       map[string]bool
	defined        map[string]bool
	inline         map[string]bool
	warnings       []string
	maxNodes       int
	nodeCount      int
	pendingComment []string
}

func newParser(tokens []token) *parser {
	return &parser{
		tokens:   tokens,
		implicit: make(map[string]bool),
		defined:  make(map[string]bool),
		inline:   make(map[string]bool),
	}
}

func (p *parser) checkNodeLimit() error {
	p.nodeCount++
	if p.maxNodes > 0 && p.nodeCount > p.maxNodes {
		return &SyntaxError{Message: "maximum node count exceeded"}
	}
	return nil
}

func (p *parser) parse() (*node, error) {
	p.root = &node{kind: nodeTable}
	p.current = p.root

	for p.pos < len(p.tokens) {
		tok := p.peek()
		switch tok.kind {
		case tokenStreamStart, tokenStreamEnd:
			p.advance()
		case tokenNewline:
			p.advance()
		case tokenComment:
			p.parseComment()
		case tokenTableStart:
			if err := p.parseTable(); err != nil {
				return nil, err
			}
		case tokenArrayTableStart:
			if err := p.parseArrayTable(); err != nil {
				return nil, err
			}
		case tokenBareKey, tokenBasicString, tokenLiteralString:
			if err := p.parseKeyValue(); err != nil {
				return nil, err
			}
		case tokenEOF:
			p.advance()
		default:
			return nil, p.syntaxError(fmt.Sprintf("unexpected token %s", tok))
		}
	}

	return p.root, nil
}

func (p *parser) consumeHeadComment() string {
	if len(p.pendingComment) == 0 {
		return ""
	}
	s := strings.Join(p.pendingComment, "\n")
	p.pendingComment = nil
	return s
}

func (p *parser) consumeLineComment() string {
	// Check if the next token on the same line is a comment
	if p.pos < len(p.tokens) && p.tokens[p.pos].kind == tokenComment {
		c := p.tokens[p.pos].value
		p.advance()
		return c
	}
	return ""
}

func (p *parser) parseComment() {
	p.pendingComment = append(p.pendingComment, p.peek().value)
	p.advance()
}

func (p *parser) parseTable() error {
	headComment := p.consumeHeadComment()

	p.advance() // consume TableStart
	keys, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	if !p.expectToken(tokenTableEnd) {
		return p.syntaxError("expected ]")
	}

	lineComment := p.consumeLineComment()

	path := joinKeys(keys)
	if p.defined[path] {
		return &DuplicateKeyError{Key: path, Pos: keys[0].pos}
	}
	if p.inline[path] {
		return &InlineTableError{Table: path, Pos: keys[0].pos}
	}
	p.defined[path] = true

	target := p.ensureTable(keys)
	target.kind = nodeTable
	target.headComment = headComment
	target.comment = lineComment
	p.current = target
	return nil
}

func (p *parser) parseArrayTable() error {
	headComment := p.consumeHeadComment()

	p.advance() // consume ArrayTableStart
	keys, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	if !p.expectToken(tokenArrayTableEnd) {
		return p.syntaxError("expected ]]")
	}

	lineComment := p.consumeLineComment()

	path := joinKeys(keys)
	if p.inline[path] {
		return &InlineTableError{Table: path, Pos: keys[0].pos}
	}

	// Clear defined keys scoped under this array table path so each
	// entry starts with a clean namespace.
	lastKey := keys[len(keys)-1].value
	prefix := lastKey + "."
	for k := range p.defined {
		if k == lastKey || strings.HasPrefix(k, prefix) {
			delete(p.defined, k)
		}
	}

	parent := p.ensureTable(keys[:len(keys)-1])
	entry := &node{
		kind:        nodeArrayTable,
		key:         []string{lastKey},
		pos:         keys[0].pos,
		headComment: headComment,
		comment:     lineComment,
	}
	parent.children = append(parent.children, entry)
	p.current = entry
	return nil
}

func (p *parser) parseKeyValue() error {
	headComment := p.consumeHeadComment()

	keys, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	if !p.expectToken(tokenKeyValueSep) {
		return p.syntaxError("expected =")
	}

	value, err := p.parseValueNode()
	if err != nil {
		return err
	}

	lineComment := p.consumeLineComment()

	// Navigate/create implicit tables for dotted key prefixes
	target := p.current
	for i, k := range keys[:len(keys)-1] {
		subPath := joinKeyStrings(keyStrings(keys[:i+1]))
		existing := findChild(target, k.value)
		if existing == nil {
			sub := &node{
				kind: nodeTable,
				key:  []string{k.value},
				pos:  k.pos,
			}
			target.children = append(target.children, sub)
			p.implicit[subPath] = true
			target = sub
		} else {
			target = existing
		}
	}

	lastKey := keys[len(keys)-1].value
	fullPath := currentPath(p.current, keys)
	if p.defined[fullPath] || findChild(target, lastKey) != nil {
		return &DuplicateKeyError{Key: fullPath, Pos: keys[0].pos}
	}
	p.defined[fullPath] = true

	kv := &node{
		kind:        nodeKeyValue,
		key:         []string{lastKey},
		pos:         keys[0].pos,
		headComment: headComment,
		comment:     lineComment,
	}
	kv.children = append(kv.children, value)

	if value.kind == nodeInlineTable {
		p.markInline(fullPath, value)
	}

	target.children = append(target.children, kv)
	return nil
}

func (p *parser) markInline(path string, n *node) {
	p.inline[path] = true
	for _, child := range n.children {
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			childPath := path + "." + child.key[0]
			p.inline[childPath] = true
			if len(child.children) > 0 && child.children[0].kind == nodeInlineTable {
				p.markInline(childPath, child.children[0])
			}
		}
	}
}

func (p *parser) parseKeyPath() ([]token, error) {
	var keys []token
	tok := p.peek()
	if tok.kind != tokenBareKey && tok.kind != tokenBasicString && tok.kind != tokenLiteralString {
		return nil, p.syntaxError("expected key")
	}
	keys = append(keys, tok)
	p.advance()

	for p.pos < len(p.tokens) && p.peek().kind == tokenDot {
		p.advance() // consume dot
		tok = p.peek()
		if tok.kind != tokenBareKey && tok.kind != tokenBasicString && tok.kind != tokenLiteralString {
			return nil, p.syntaxError("expected key after dot")
		}
		keys = append(keys, tok)
		p.advance()
	}

	return keys, nil
}

func (p *parser) parseValueNode() (*node, error) {
	if err := p.checkNodeLimit(); err != nil {
		return nil, err
	}
	if p.pos >= len(p.tokens) {
		return nil, p.syntaxError("expected value")
	}
	tok := p.peek()
	switch tok.kind {
	case tokenBasicString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleBasic, pos: tok.pos}, nil
	case tokenMultiLineBasicString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleMultiLineBasic, pos: tok.pos}, nil
	case tokenLiteralString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleLiteral, pos: tok.pos}, nil
	case tokenMultiLineLiteralString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleMultiLineLiteral, pos: tok.pos}, nil
	case tokenInteger:
		p.advance()
		return &node{kind: nodeInteger, value: tok.value, pos: tok.pos}, nil
	case tokenFloat:
		p.advance()
		return &node{kind: nodeFloat, value: tok.value, pos: tok.pos}, nil
	case tokenBoolean:
		p.advance()
		return &node{kind: nodeBoolean, value: tok.value, pos: tok.pos}, nil
	case tokenOffsetDateTime:
		p.advance()
		return &node{kind: nodeDateTime, value: tok.value, pos: tok.pos}, nil
	case tokenLocalDateTime:
		p.advance()
		return &node{kind: nodeLocalDateTime, value: tok.value, pos: tok.pos}, nil
	case tokenLocalDate:
		p.advance()
		return &node{kind: nodeLocalDate, value: tok.value, pos: tok.pos}, nil
	case tokenLocalTime:
		p.advance()
		return &node{kind: nodeLocalTime, value: tok.value, pos: tok.pos}, nil
	case tokenArrayStart:
		return p.parseArray()
	case tokenInlineTableStart:
		return p.parseInlineTable()
	default:
		return nil, p.syntaxError(fmt.Sprintf("unexpected token %s in value", tok))
	}
}

func (p *parser) parseArray() (*node, error) {
	start := p.peek().pos
	p.advance() // consume [
	arr := &node{kind: nodeArray, pos: start}

	p.skipNewlinesAndComments()
	for p.pos < len(p.tokens) && p.peek().kind != tokenArrayEnd {
		val, err := p.parseValueNode()
		if err != nil {
			return nil, err
		}
		arr.children = append(arr.children, val)
		p.skipNewlinesAndComments()
		if p.pos < len(p.tokens) && p.peek().kind == tokenComma {
			p.advance()
			p.skipNewlinesAndComments()
		}
	}
	if !p.expectToken(tokenArrayEnd) {
		return nil, p.syntaxError("expected ]")
	}
	return arr, nil
}

func (p *parser) parseInlineTable() (*node, error) {
	start := p.peek().pos
	p.advance() // consume {
	tbl := &node{kind: nodeInlineTable, pos: start}

	p.skipNewlinesAndComments()
	for p.pos < len(p.tokens) && p.peek().kind != tokenInlineTableEnd {
		keys, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		if !p.expectToken(tokenKeyValueSep) {
			return nil, p.syntaxError("expected = in inline table")
		}
		val, err := p.parseValueNode()
		if err != nil {
			return nil, err
		}
		kv := &node{
			kind:     nodeKeyValue,
			key:      keyStrings(keys),
			pos:      keys[0].pos,
			children: []*node{val},
		}
		tbl.children = append(tbl.children, kv)
		p.skipNewlinesAndComments()
		if p.pos < len(p.tokens) && p.peek().kind == tokenComma {
			p.advance()
			p.skipNewlinesAndComments()
		}
	}
	if !p.expectToken(tokenInlineTableEnd) {
		return nil, p.syntaxError("expected }")
	}
	return tbl, nil
}

func (p *parser) skipNewlinesAndComments() {
	for p.pos < len(p.tokens) {
		k := p.peek().kind
		if k == tokenNewline || k == tokenComment {
			p.advance()
		} else {
			return
		}
	}
}

func (p *parser) ensureTable(keys []token) *node {
	target := p.root
	for _, k := range keys {
		child := findChild(target, k.value)
		if child == nil {
			child = &node{
				kind: nodeTable,
				key:  []string{k.value},
				pos:  k.pos,
			}
			target.children = append(target.children, child)
		}
		// For array-of-tables, target the last entry
		if child.kind == nodeArrayTable {
			last := findLastArrayEntry(target, k.value)
			if last != nil {
				child = last
			}
		}
		target = child
	}
	return target
}

func findChild(n *node, key string) *node {
	for _, child := range n.children {
		if len(child.key) > 0 && child.key[0] == key {
			if child.kind == nodeKeyValue {
				if len(child.children) > 0 && (child.children[0].kind == nodeInlineTable || child.children[0].kind == nodeTable) {
					return child.children[0]
				}
				continue
			}
			return child
		}
	}
	return nil
}

func findLastArrayEntry(parent *node, key string) *node {
	var last *node
	for _, child := range parent.children {
		if child.kind == nodeArrayTable && len(child.key) > 0 && child.key[0] == key {
			last = child
		}
	}
	return last
}

// Helper methods

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *parser) expectToken(kind tokenKind) bool {
	if p.pos < len(p.tokens) && p.tokens[p.pos].kind == kind {
		p.advance()
		return true
	}
	return false
}

func (p *parser) syntaxError(msg string) error {
	pos := Position{}
	if p.pos < len(p.tokens) {
		pos = p.tokens[p.pos].pos
	}
	return &SyntaxError{Message: msg, Pos: pos}
}

func joinKeys(keys []token) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k.value
	}
	return strings.Join(parts, ".")
}

func joinKeyStrings(keys []string) string {
	return strings.Join(keys, ".")
}

func keyStrings(keys []token) []string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = k.value
	}
	return s
}

func currentPath(ctx *node, keys []token) string {
	var parts []string
	if ctx != nil && len(ctx.key) > 0 {
		parts = append(parts, ctx.key...)
	}
	for _, k := range keys {
		parts = append(parts, k.value)
	}
	return strings.Join(parts, ".")
}
