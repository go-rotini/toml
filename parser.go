package toml

import (
	"fmt"
	"strings"
)

type parser struct {
	tokens          []token
	pos             int
	root            *node
	current         *node
	currentPath     string
	implicit        map[string]bool
	defined         map[string]bool
	inline          map[string]bool
	arrayDefined    map[string]bool
	staticArrays    map[string]bool
	dottedImplicit  map[string]bool
	explicitTables  map[string]bool
	warnings        []string
	maxNodes        int
	nodeCount       int
	pendingComment  []string
}

func newParser(tokens []token) *parser {
	return &parser{
		tokens:         tokens,
		implicit:       make(map[string]bool),
		defined:        make(map[string]bool),
		inline:         make(map[string]bool),
		arrayDefined:   make(map[string]bool),
		staticArrays:   make(map[string]bool),
		dottedImplicit: make(map[string]bool),
		explicitTables: make(map[string]bool),
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

	// After table header + optional comment, only newline or EOF is allowed
	if p.pos < len(p.tokens) {
		next := p.peek()
		if next.kind != tokenNewline && next.kind != tokenEOF && next.kind != tokenComment && next.kind != tokenStreamEnd {
			return p.syntaxError("extra content after table header")
		}
	}

	path := "\x00" + joinKeys(keys)
	if p.defined[path] {
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].pos}
	}
	if p.inline[path] {
		return &InlineTableError{Table: displayPath(keys), Pos: keys[0].pos}
	}
	if p.arrayDefined[path] {
		return &ConflictError{Key: displayPath(keys), Message: "cannot define table: already defined as array of tables", Pos: keys[0].pos}
	}
	if p.dottedImplicit[path] {
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].pos}
	}
	// Check that no prefix is a non-table value
	for i := range keys[:len(keys)-1] {
		prefix := "\x00" + joinKeyStrings(keyStrings(keys[:i+1]))
		if p.defined[prefix] && !p.implicit[prefix] && !p.explicitTables[prefix] && !p.arrayDefined[prefix] {
			return &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: keys[0].pos}
		}
	}
	p.defined[path] = true
	p.explicitTables[path] = true

	target := p.ensureTable(keys)
	delete(p.implicit, path)
	target.kind = nodeTable
	target.headComment = headComment
	target.comment = lineComment
	p.current = target
	p.currentPath = path
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

	// After array table header + optional comment, only newline or EOF is allowed
	if p.pos < len(p.tokens) {
		next := p.peek()
		if next.kind != tokenNewline && next.kind != tokenEOF && next.kind != tokenComment && next.kind != tokenStreamEnd {
			return p.syntaxError("extra content after array table header")
		}
	}

	path := "\x00" + joinKeys(keys)
	if p.inline[path] {
		return &InlineTableError{Table: displayPath(keys), Pos: keys[0].pos}
	}
	if p.defined[path] && !p.implicit[path] && !p.arrayDefined[path] {
		return &ConflictError{Key: displayPath(keys), Message: "cannot define array of tables: already defined as table", Pos: keys[0].pos}
	}
	if p.staticArrays[path] {
		return &ConflictError{Key: displayPath(keys), Message: "cannot extend static array with array of tables", Pos: keys[0].pos}
	}
	if p.dottedImplicit[path] {
		return &ConflictError{Key: displayPath(keys), Message: "cannot define array of tables: key already defined via dotted keys", Pos: keys[0].pos}
	}
	if p.implicit[path] && !p.arrayDefined[path] {
		return &ConflictError{Key: displayPath(keys), Message: "cannot define array of tables: already implicitly defined as table", Pos: keys[0].pos}
	}
	// Check that no prefix is a non-table value
	for i := range keys[:len(keys)-1] {
		prefix := "\x00" + joinKeyStrings(keyStrings(keys[:i+1]))
		if p.defined[prefix] && !p.implicit[prefix] && !p.explicitTables[prefix] && !p.arrayDefined[prefix] {
			return &ConflictError{Key: displayPath(keys[:i+1]), Message: "cannot use as table: already defined as value", Pos: keys[0].pos}
		}
	}
	p.arrayDefined[path] = true

	// Clear defined keys scoped under this array table path so each
	// entry starts with a clean namespace.
	clearPrefix := path + "\x00"
	for k := range p.defined {
		if strings.HasPrefix(k, clearPrefix) {
			delete(p.defined, k)
		}
	}
	for k := range p.dottedImplicit {
		if strings.HasPrefix(k, clearPrefix) {
			delete(p.dottedImplicit, k)
		}
	}

	parent := p.ensureTable(keys[:len(keys)-1])
	lastKey := keys[len(keys)-1].value
	entry := &node{
		kind:        nodeArrayTable,
		key:         []string{lastKey},
		pos:         keys[0].pos,
		headComment: headComment,
		comment:     lineComment,
	}
	parent.children = append(parent.children, entry)
	p.current = entry
	p.currentPath = path
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

	// Build full path from root
	fullPath := p.currentPath + "\x00" + joinKeys(keys)

	// Check dotted key prefixes don't overwrite existing values
	target := p.current
	for i, k := range keys[:len(keys)-1] {
		subPath := p.currentPath + "\x00" + joinKeyStrings(keyStrings(keys[:i+1]))

		if p.defined[subPath] && !p.implicit[subPath] {
			return &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: keys[0].pos}
		}
		if p.inline[subPath] {
			return &InlineTableError{Table: displayPath(keys[:i+1]), Pos: keys[0].pos}
		}

		existing := findChild(target, k.value)
		if existing == nil {
			sub := &node{
				kind: nodeTable,
				key:  []string{k.value},
				pos:  k.pos,
			}
			target.children = append(target.children, sub)
			p.implicit[subPath] = true
			p.dottedImplicit[subPath] = true
			target = sub
		} else {
			if existing.kind != nodeTable && existing.kind != nodeInlineTable {
				return &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: keys[0].pos}
			}
			target = existing
		}
	}

	lastKey := keys[len(keys)-1].value
	if p.defined[fullPath] || findChild(target, lastKey) != nil {
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].pos}
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
	if value.kind == nodeArray {
		p.staticArrays[fullPath] = true
	}

	target.children = append(target.children, kv)
	return nil
}

func (p *parser) markInline(path string, n *node) {
	p.inline[path] = true
	for _, child := range n.children {
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			childPath := path + "\x00" + child.key[0]
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
		} else if p.pos < len(p.tokens) && p.peek().kind != tokenArrayEnd {
			return nil, p.syntaxError("expected comma or ] in array")
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

	defined := make(map[string]bool)

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

		fullPath := joinKeys(keys)
		if defined[fullPath] {
			return nil, &DuplicateKeyError{Key: fullPath, Pos: keys[0].pos}
		}
		defined[fullPath] = true

		// Expand dotted keys into nested tables
		target := tbl
		for i, k := range keys[:len(keys)-1] {
			subPath := joinKeyStrings(keyStrings(keys[:i+1]))
			if defined[subPath] {
				return nil, &DuplicateKeyError{Key: subPath, Pos: k.pos}
			}
			existing := findChild(target, k.value)
			if existing == nil {
				sub := &node{
					kind: nodeInlineTable,
					key:  []string{k.value},
					pos:  k.pos,
				}
				kv := &node{
					kind:     nodeKeyValue,
					key:      []string{k.value},
					pos:      k.pos,
					children: []*node{sub},
				}
				target.children = append(target.children, kv)
				target = sub
			} else {
				target = existing
			}
		}

		lastKey := keys[len(keys)-1].value
		if findChild(target, lastKey) != nil {
			return nil, &DuplicateKeyError{Key: fullPath, Pos: keys[0].pos}
		}

		kv := &node{
			kind:     nodeKeyValue,
			key:      []string{lastKey},
			pos:      keys[0].pos,
			children: []*node{val},
		}
		target.children = append(target.children, kv)
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
	for i, k := range keys {
		child := findChild(target, k.value)
		if child == nil {
			child = &node{
				kind: nodeTable,
				key:  []string{k.value},
				pos:  k.pos,
			}
			target.children = append(target.children, child)
			subPath := "\x00" + joinKeyStrings(keyStrings(keys[:i+1]))
			p.implicit[subPath] = true
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
	return strings.Join(parts, "\x00")
}

func joinKeyStrings(keys []string) string {
	return strings.Join(keys, "\x00")
}

func keyStrings(keys []token) []string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = k.value
	}
	return s
}

func displayPath(keys []token) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k.value
	}
	return strings.Join(parts, ".")
}

