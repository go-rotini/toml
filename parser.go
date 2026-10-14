package toml

import (
	"fmt"
	"strings"
)

// maxNestingDepth bounds how deeply tables, inline tables and arrays may nest
// whatever the options say, so recursion over the tree stays well within the
// goroutine stack.
const maxNestingDepth = 10000

// tableDef records how the parser created a table node, which decides how
// later headers and dotted keys may extend it.
type tableDef uint8

const (
	defNone     tableDef = iota // the root, an [[array]] entry, or an inline table written as a value
	defImplicit                 // a parent of a table named by a header, not yet named itself
	defDotted                   // created by a dotted key
	defExplicit                 // named by a [table] header
)

// indexThreshold is the child count above which a node keeps a map from key
// to child instead of scanning its children.
const indexThreshold = 8

type parser struct {
	tokens         []token
	pos            int
	root           *node
	current        *node
	currentDepth   int // nesting depth of current: 1 for the root, one more per header segment
	commentBreak   int // pendingComment[:commentBreak] ends before a blank line
	maxNodes       int
	maxDepth       int
	nodeCount      int
	pendingComment []string
}

func newParser(tokens []token) *parser {
	return &parser{tokens: tokens, maxDepth: maxNestingDepth}
}

// parseDocument scans and parses data. maxNodes and maxDepth are the
// [WithMaxNodes] and [WithMaxDepth] limits (zero for none).
func parseDocument(data []byte, maxNodes, maxDepth int) (*scanner, *node, error) {
	s, err := newScanner(data)
	if err != nil {
		return nil, nil, err
	}
	tokens, err := s.scan()
	if err != nil {
		return nil, nil, s.inputErrorPos(err)
	}
	p := newParser(tokens)
	p.maxNodes = maxNodes
	if maxDepth > 0 && maxDepth < p.maxDepth {
		p.maxDepth = maxDepth
	}
	root, err := p.parse()
	if err != nil {
		return nil, nil, s.inputErrorPos(err)
	}
	if s.mapsOffsets() {
		s.inputPositions(root)
	}
	return s, root, nil
}

// lookup returns n's child with the given key: a key/value pair, a table,
// or the last entry of an array of tables.
func (n *node) lookup(key string) *node {
	if n.index != nil {
		return n.index[key]
	}
	var found *node
	for _, child := range n.children {
		if len(child.key) > 0 && child.key[0] == key {
			if child.kind != nodeArrayTable {
				return child
			}
			found = child
		}
	}
	return found
}

// addChild appends a keyed child, keeping the lookup index current.
func (n *node) addChild(child *node) {
	n.children = append(n.children, child)
	switch {
	case n.index != nil:
		n.index[child.key[0]] = child
	case len(n.children) > indexThreshold:
		n.index = make(map[string]*node, 2*len(n.children))
		for _, c := range n.children {
			if len(c.key) > 0 {
				n.index[c.key[0]] = c
			}
		}
	}
}

// pairValue returns the value of a key/value pair node.
func pairValue(kv *node) *node {
	if len(kv.children) == 0 {
		return &node{}
	}
	return kv.children[0]
}

func (p *parser) checkNodeLimit() error {
	p.nodeCount++
	if p.maxNodes > 0 && p.nodeCount > p.maxNodes {
		return &SyntaxError{Message: "maximum node count exceeded"}
	}
	return nil
}

// checkDepth reports an error when a table, inline table or array would
// nest depth levels deep, past the limit.
func (p *parser) checkDepth(depth int, at token) error {
	if depth > p.maxDepth {
		return &SyntaxError{Message: "maximum nesting depth exceeded", Pos: at.startPos()}
	}
	return nil
}

func (p *parser) parse() (*node, error) {
	p.root = &node{kind: nodeTable}
	p.current = p.root
	p.currentDepth = 1

	for p.pos < len(p.tokens) {
		tok := p.peek()
		switch tok.kind {
		case tokenStreamStart, tokenStreamEnd:
			p.advance()
		case tokenNewline:
			if len(p.pendingComment) > 0 && p.pos > 0 && p.tokens[p.pos-1].kind == tokenNewline {
				p.commentBreak = len(p.pendingComment)
			}
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
			return nil, p.syntaxError(fmt.Sprintf("unexpected %s", tok.describe()))
		}
	}

	if len(p.pendingComment) > 0 {
		p.addFootComment(len(p.pendingComment))
	}
	return p.root, nil
}

func (p *parser) consumeHeadComment() string {
	p.commentBreak = 0
	if len(p.pendingComment) == 0 {
		return ""
	}
	s := strings.Join(p.pendingComment, "\n")
	p.pendingComment = nil
	return s
}

// consumeFootComment runs before a table header. The pending comments that a
// blank line separates from the header become the foot comment of the
// current table, provided the table has contents for them to follow.
func (p *parser) consumeFootComment() {
	if p.commentBreak > 0 && len(p.current.children) > 0 {
		p.addFootComment(p.commentBreak)
	}
}

// addFootComment moves the first n pending comments to the current table's
// foot comment.
func (p *parser) addFootComment(n int) {
	foot := strings.Join(p.pendingComment[:n], "\n")
	if p.current.footComment != "" {
		foot = p.current.footComment + "\n" + foot
	}
	p.current.footComment = foot
	p.pendingComment = p.pendingComment[n:]
	p.commentBreak = 0
}

func (p *parser) consumeLineComment() string {
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

// parseHeader reads a [table] or [[array]] header up to and including its
// line comment, and checks that nothing else follows it on the line.
func (p *parser) parseHeader(end tokenKind, what string) (keys []token, open, closing token, lineComment string, err error) {
	open = p.peek()
	p.advance() // consume [ or [[
	keys, err = p.parseKeyPath()
	if err != nil {
		return nil, open, closing, "", err
	}
	closing = p.peek()
	if !p.expectToken(end) {
		if end == tokenArrayTableEnd {
			return nil, open, closing, "", p.syntaxError("expected ]]")
		}
		return nil, open, closing, "", p.syntaxError("expected ]")
	}

	lineComment = p.consumeLineComment()

	if p.pos < len(p.tokens) {
		next := p.peek()
		if next.kind != tokenNewline && next.kind != tokenEOF && next.kind != tokenComment && next.kind != tokenStreamEnd {
			return nil, open, closing, "", p.syntaxError("extra content after " + what)
		}
	}
	if err := p.checkDepth(len(keys)+1, keys[0]); err != nil {
		return nil, open, closing, "", err
	}
	return keys, open, closing, lineComment, nil
}

// headerParent walks a header's keys but the last from the root, creating
// missing tables, and returns the table the last key belongs to. An array of
// tables on the way resolves to its last entry.
func (p *parser) headerParent(keys []token, arrayHeader bool) (*node, error) {
	t := p.root
	for i, k := range keys[:len(keys)-1] {
		c := t.lookup(k.value)
		switch {
		case c == nil:
			c = &node{kind: nodeTable, key: []string{k.value}, pos: k.pos, def: defImplicit}
			t.addChild(c)
		case c.kind == nodeKeyValue:
			if pairValue(c).kind == nodeInlineTable {
				return nil, &InlineTableError{Table: displayPath(keys[:i+1]), Pos: keys[0].startPos()}
			}
			if arrayHeader {
				return nil, &ConflictError{Key: displayPath(keys[:i+1]), Message: "cannot use as table: already defined as value", Pos: keys[0].startPos()}
			}
			return nil, &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: keys[0].startPos()}
		}
		t = c
	}
	return t, nil
}

func (p *parser) parseTable() error {
	p.consumeFootComment()
	headComment := p.consumeHeadComment()

	keys, open, closing, lineComment, err := p.parseHeader(tokenTableEnd, "table header")
	if err != nil {
		return err
	}
	parent, err := p.headerParent(keys, false)
	if err != nil {
		return err
	}

	last := keys[len(keys)-1]
	target := parent.lookup(last.value)
	switch {
	case target == nil:
		target = &node{kind: nodeTable, key: []string{last.value}, pos: last.pos}
		parent.addChild(target)
	case target.kind == nodeKeyValue:
		if pairValue(target).kind == nodeInlineTable {
			return &InlineTableError{Table: displayPath(keys), Pos: keys[0].startPos()}
		}
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].startPos()}
	case target.kind == nodeArrayTable:
		return &ConflictError{Key: displayPath(keys), Message: "cannot define table: already defined as array of tables", Pos: keys[0].startPos()}
	case target.def != defImplicit:
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].startPos()}
	}

	target.def = defExplicit
	target.headComment = headComment
	target.comment = lineComment
	target.span = srcSpan{start: int(open.start), end: closing.pos.Offset}
	target.keySpan = keyPathSpan(keys)
	p.current = target
	p.currentDepth = len(keys) + 1
	return nil
}

func (p *parser) parseArrayTable() error {
	p.consumeFootComment()
	headComment := p.consumeHeadComment()

	keys, open, closing, lineComment, err := p.parseHeader(tokenArrayTableEnd, "array table header")
	if err != nil {
		return err
	}
	parent, err := p.headerParent(keys, true)
	if err != nil {
		return err
	}

	last := keys[len(keys)-1]
	if existing := parent.lookup(last.value); existing != nil && existing.kind != nodeArrayTable {
		conflict := func(msg string) error {
			return &ConflictError{Key: displayPath(keys), Message: msg, Pos: keys[0].startPos()}
		}
		switch {
		case existing.kind == nodeKeyValue:
			switch pairValue(existing).kind {
			case nodeInlineTable:
				return &InlineTableError{Table: displayPath(keys), Pos: keys[0].startPos()}
			case nodeArray:
				return conflict("cannot extend static array with array of tables")
			default:
				return conflict("cannot define array of tables: already defined as value")
			}
		case existing.def == defDotted:
			return conflict("cannot define array of tables: key already defined via dotted keys")
		case existing.def == defImplicit:
			return conflict("cannot define array of tables: already implicitly defined as table")
		default:
			return conflict("cannot define array of tables: already defined as table")
		}
	}

	entry := &node{
		kind:        nodeArrayTable,
		key:         []string{last.value},
		pos:         keys[0].pos,
		headComment: headComment,
		comment:     lineComment,
		span:        srcSpan{start: int(open.start), end: closing.pos.Offset},
		keySpan:     keyPathSpan(keys),
	}
	parent.addChild(entry)
	p.current = entry
	p.currentDepth = len(keys) + 1
	return nil
}

func (p *parser) parseKeyValue() error {
	headComment := p.consumeHeadComment()

	keys, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	if err := p.checkDepth(p.currentDepth+len(keys)-1, keys[0]); err != nil {
		return err
	}
	if !p.expectToken(tokenKeyValueSep) {
		return p.syntaxError("expected =")
	}

	value, err := p.parseValueNode(p.currentDepth + len(keys))
	if err != nil {
		return err
	}

	lineComment := p.consumeLineComment()

	target := p.current
	for i, k := range keys[:len(keys)-1] {
		c := target.lookup(k.value)
		switch {
		case c == nil:
			c = &node{kind: nodeTable, key: []string{k.value}, pos: k.pos, def: defDotted}
			target.addChild(c)
		case c.kind == nodeKeyValue && pairValue(c).kind == nodeInlineTable:
			return &InlineTableError{Table: displayPath(keys[:i+1]), Pos: keys[0].startPos()}
		case c.kind != nodeTable || c.def == defExplicit:
			return &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: keys[0].startPos()}
		case c.def == defImplicit:
			// Defined by dotted keys from here on, so a later header for it
			// is a redefinition.
			c.def = defDotted
		}
		target = c
	}

	lastKey := keys[len(keys)-1].value
	if target.lookup(lastKey) != nil {
		return &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].startPos()}
	}

	kv := &node{
		kind:        nodeKeyValue,
		key:         []string{lastKey},
		pos:         keys[0].pos,
		headComment: headComment,
		comment:     lineComment,
		span:        srcSpan{start: int(keys[0].start), end: value.span.end},
		keySpan:     keyPathSpan(keys),
		children:    []*node{value},
	}
	target.addChild(kv)
	return nil
}

func (p *parser) parseKeyPath() ([]token, error) {
	tok := p.peek()
	if tok.kind != tokenBareKey && tok.kind != tokenBasicString && tok.kind != tokenLiteralString {
		return nil, p.syntaxError("expected key")
	}
	keys := []token{tok}
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

// parseValueNode parses a value; depth is the nesting depth it has if it is
// an array or inline table.
func (p *parser) parseValueNode(depth int) (*node, error) {
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
		return &node{kind: nodeString, value: tok.value, style: styleBasic, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenMultiLineBasicString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleMultiLineBasic, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenLiteralString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleLiteral, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenMultiLineLiteralString:
		p.advance()
		return &node{kind: nodeString, value: tok.value, style: styleMultiLineLiteral, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenInteger:
		p.advance()
		return &node{kind: nodeInteger, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenFloat:
		p.advance()
		return &node{kind: nodeFloat, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenBoolean:
		p.advance()
		return &node{kind: nodeBoolean, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenOffsetDateTime:
		p.advance()
		return &node{kind: nodeDateTime, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenLocalDateTime:
		p.advance()
		return &node{kind: nodeLocalDateTime, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenLocalDate:
		p.advance()
		return &node{kind: nodeLocalDate, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenLocalTime:
		p.advance()
		return &node{kind: nodeLocalTime, value: tok.value, pos: tok.pos, span: tokenSpan(tok)}, nil
	case tokenArrayStart:
		return p.parseArray(depth)
	case tokenInlineTableStart:
		return p.parseInlineTable(depth)
	case tokenNewline, tokenComment, tokenStreamEnd, tokenEOF:
		return nil, p.syntaxError("missing value")
	default:
		return nil, p.syntaxError(fmt.Sprintf("unexpected %s in value", tok.describe()))
	}
}

func (p *parser) parseArray(depth int) (*node, error) {
	open := p.peek()
	if err := p.checkDepth(depth, open); err != nil {
		return nil, err
	}
	p.advance() // consume [
	arr := &node{kind: nodeArray, pos: open.pos}

	p.skipNewlinesAndComments()
	for p.pos < len(p.tokens) && p.peek().kind != tokenArrayEnd {
		val, err := p.parseValueNode(depth + 1)
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
	closing := p.peek()
	if !p.expectToken(tokenArrayEnd) {
		return nil, p.syntaxError("expected ]")
	}
	arr.span = srcSpan{start: int(open.start), end: int(closing.start) + 1}
	return arr, nil
}

func (p *parser) parseInlineTable(depth int) (*node, error) {
	open := p.peek()
	if err := p.checkDepth(depth, open); err != nil {
		return nil, err
	}
	p.advance() // consume {
	tbl := &node{kind: nodeInlineTable, pos: open.pos}

	p.skipNewlinesAndComments()
	for p.pos < len(p.tokens) && p.peek().kind != tokenInlineTableEnd {
		keys, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		if err := p.checkDepth(depth+len(keys)-1, keys[0]); err != nil {
			return nil, err
		}
		if !p.expectToken(tokenKeyValueSep) {
			return nil, p.syntaxError("expected = in inline table")
		}
		val, err := p.parseValueNode(depth + len(keys))
		if err != nil {
			return nil, err
		}

		target := tbl
		for i, k := range keys[:len(keys)-1] {
			c := target.lookup(k.value)
			if c == nil {
				sub := &node{kind: nodeInlineTable, key: []string{k.value}, pos: k.pos, def: defDotted}
				target.addChild(&node{kind: nodeKeyValue, key: []string{k.value}, pos: k.pos, children: []*node{sub}})
				target = sub
				continue
			}
			v := pairValue(c)
			if v.kind != nodeInlineTable || v.def != defDotted {
				return nil, &DuplicateKeyError{Key: displayPath(keys[:i+1]), Pos: k.startPos()}
			}
			target = v
		}

		lastKey := keys[len(keys)-1].value
		if target.lookup(lastKey) != nil {
			return nil, &DuplicateKeyError{Key: displayPath(keys), Pos: keys[0].startPos()}
		}

		target.addChild(&node{
			kind:     nodeKeyValue,
			key:      []string{lastKey},
			pos:      keys[0].pos,
			children: []*node{val},
			span:     srcSpan{start: int(keys[0].start), end: val.span.end},
			keySpan:  keyPathSpan(keys),
		})
		p.skipNewlinesAndComments()
		if p.pos < len(p.tokens) && p.peek().kind == tokenComma {
			p.advance()
			p.skipNewlinesAndComments()
		}
	}
	closing := p.peek()
	if !p.expectToken(tokenInlineTableEnd) {
		return nil, p.syntaxError("expected }")
	}
	tbl.span = srcSpan{start: int(open.start), end: int(closing.start) + 1}
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

// syntaxError reports msg at the token the parser stopped at.
func (p *parser) syntaxError(msg string) error {
	tok := p.peek()
	return &SyntaxError{Message: msg, Pos: tok.startPos(), Token: tok.describe()}
}

func displayPath(keys []token) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k.value
	}
	return strings.Join(parts, ".")
}
