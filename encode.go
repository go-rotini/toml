package toml

import (
	"bytes"
	"context"
	"encoding"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Marshaler is implemented by types that can encode themselves into a
// TOML-compatible Go value. The returned value is then encoded normally.
type Marshaler interface {
	MarshalTOML() (any, error)
}

// BytesMarshaler is implemented by types that can encode themselves
// directly into raw TOML bytes.
type BytesMarshaler interface {
	MarshalTOML() ([]byte, error)
}

// MarshalerContext is like [Marshaler] but receives a context.
type MarshalerContext interface {
	MarshalTOML(ctx context.Context) (any, error)
}

type encoder struct {
	opts *encoderOptions
	buf  bytes.Buffer
	ctx  context.Context

	scalars *scanner // reused by checkScalar

	depth int               // nested values being encoded
	seen  map[cycleKey]bool // pointers, maps and slices on the current path, past cycleDepth
}

// cycleDepth is the nesting depth past which the encoder looks for a value
// that contains itself; shallower values cannot loop forever.
const cycleDepth = 1000

// cycleKey identifies a pointer, map or slice by what it refers to.
type cycleKey struct {
	t   reflect.Type
	ptr uintptr
	n   int
}

// visit records that encoding descends into rv. Past cycleDepth it reports
// a pointer, map or slice that is already being encoded further up, which
// would otherwise recurse forever. Each visit that returns no error must be
// paired with a call to leave.
func (e *encoder) visit(rv reflect.Value) (cycleKey, error) {
	e.depth++
	if e.depth <= cycleDepth {
		return cycleKey{}, nil
	}
	for rv.Kind() == reflect.Interface && !rv.IsNil() {
		rv = rv.Elem()
	}
	var k cycleKey
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map:
		if rv.IsNil() {
			return cycleKey{}, nil
		}
		k = cycleKey{t: rv.Type(), ptr: rv.Pointer()}
	case reflect.Slice:
		if rv.IsNil() {
			return cycleKey{}, nil
		}
		k = cycleKey{t: rv.Type(), ptr: rv.Pointer(), n: rv.Len()}
	default:
		return cycleKey{}, nil
	}
	if e.seen[k] {
		e.depth--
		return cycleKey{}, fmt.Errorf("toml: %w via %s", errEncodeCycle, rv.Type())
	}
	if e.seen == nil {
		e.seen = make(map[cycleKey]bool)
	}
	e.seen[k] = true
	return k, nil
}

// leave undoes the visit that returned k.
func (e *encoder) leave(k cycleKey) {
	e.depth--
	if k.t != nil {
		delete(e.seen, k)
	}
}

func newEncoder(opts *encoderOptions) *encoder {
	return &encoder{
		opts: opts,
		ctx:  context.Background(),
	}
}

func (e *encoder) encode(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, ErrMarshalTop
	}
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, ErrMarshalTop
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct && rv.Kind() != reflect.Map {
		return nil, ErrMarshalTop
	}

	if err := e.encodeTable(rv, nil); err != nil {
		return nil, err
	}
	return e.buf.Bytes(), nil
}

func (e *encoder) encodeTable(rv reflect.Value, path []string) error {
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return nil
		}
		if rv.Kind() == reflect.Interface && !rv.Elem().IsValid() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		return e.encodeStruct(rv, path)
	case reflect.Map:
		return e.encodeMap(rv, path)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedEncodeType, rv.Kind())
	}
}

func (e *encoder) encodeStruct(rv reflect.Value, path []string) error {
	sf := getStructFields(rv.Type())

	type fieldEntry struct {
		fi    fieldInfo
		value reflect.Value
	}
	var simples, tables []fieldEntry

	for _, fi := range sf.fields {
		fv, err := rv.FieldByIndexErr(fi.index)
		if err != nil || isNil(fv) {
			continue
		}
		if (fi.omitEmpty || e.opts.omitEmpty) && isEmpty(fv) {
			continue
		}
		if isTableValue(fv) && !fi.inline && !fi.commented && !e.opts.inlineTable {
			tables = append(tables, fieldEntry{fi, fv})
		} else {
			simples = append(simples, fieldEntry{fi, fv})
		}
	}

	for _, entry := range simples {
		if err := e.encodeField(entry.fi, entry.value, path); err != nil {
			return err
		}
	}

	for _, entry := range tables {
		if err := e.encodeTableField(entry.fi, entry.value, path); err != nil {
			return err
		}
	}

	return nil
}

// sortedMapKeys returns rv's keys and their TOML spellings, sorted by
// spelling.
func sortedMapKeys(rv reflect.Value) ([]reflect.Value, []string) {
	keys := rv.MapKeys()
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = fmt.Sprint(k.Interface())
	}
	sort.Sort(byName{keys, names})
	return keys, names
}

type byName struct {
	keys  []reflect.Value
	names []string
}

func (b byName) Len() int           { return len(b.keys) }
func (b byName) Less(i, j int) bool { return b.names[i] < b.names[j] }
func (b byName) Swap(i, j int) {
	b.keys[i], b.keys[j] = b.keys[j], b.keys[i]
	b.names[i], b.names[j] = b.names[j], b.names[i]
}

func (e *encoder) encodeMap(rv reflect.Value, path []string) error {
	if rv.IsNil() {
		return nil
	}

	keys, names := sortedMapKeys(rv)

	type mapEntry struct {
		key   string
		value reflect.Value
	}
	var simples, tables []mapEntry

	for i, k := range keys {
		val := rv.MapIndex(k)
		for val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		if isNil(val) {
			continue
		}
		if isTableValue(val) && !e.opts.inlineTable {
			tables = append(tables, mapEntry{names[i], val})
		} else {
			simples = append(simples, mapEntry{names[i], val})
		}
	}

	for _, entry := range simples {
		err := e.writeKeyValue(path, entry.key, "", func() error {
			return e.encodeValue(entry.value, fieldInfo{})
		})
		if err != nil {
			return err
		}
	}

	for _, entry := range tables {
		subPath := append(append([]string{}, path...), entry.key)
		if err := e.encodeMapTable(entry.value, subPath); err != nil {
			return err
		}
	}

	return nil
}

// encodeMapTable writes a map entry that is a table or an array of tables.
func (e *encoder) encodeMapTable(val reflect.Value, path []string) error {
	k, err := e.visit(val)
	if err != nil {
		return err
	}
	defer e.leave(k)
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		val = val.Elem()
	}
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		return e.encodeArrayTables(val, path)
	}
	e.writeTableHeader(path)
	if err := e.encodeTable(val, path); err != nil {
		return err
	}
	e.writeCommentBlock(e.headerIndent(path), e.optionComment(path, FootCommentPos))
	return nil
}

// encodeArrayTables writes each element of rv as a [[table]] entry. The
// [WithComment] head and line comments go on the first header and the foot
// comment after the last entry.
func (e *encoder) encodeArrayTables(rv reflect.Value, path []string) error {
	for i := range rv.Len() {
		e.writeArrayTableHeader(path, i == 0)
		k, err := e.visit(rv.Index(i))
		if err != nil {
			return err
		}
		err = e.encodeTable(rv.Index(i), path)
		e.leave(k)
		if err != nil {
			return err
		}
	}
	e.writeCommentBlock(e.headerIndent(path), e.optionComment(path, FootCommentPos))
	return nil
}

func (e *encoder) encodeField(fi fieldInfo, fv reflect.Value, path []string) error {
	indent := e.indentPrefix(len(path))
	e.writeCommentBlock(indent, fi.comment)
	if !fi.commented {
		return e.writeKeyValue(path, fi.name, "", func() error {
			return e.encodeValue(fv, fi)
		})
	}
	// A commented field may span lines, as a multi-line array or string
	// does; every line is commented out.
	return e.writeKeyValue(path, fi.name, "# ", func() error {
		sub := &encoder{opts: e.opts, ctx: e.ctx}
		if err := sub.encodeValue(fv, fi); err != nil {
			return err
		}
		e.buf.WriteString(strings.ReplaceAll(sub.buf.String(), "\n", "\n"+indent+"# "))
		return nil
	})
}

// writeKeyValue writes one key/value line under path, with the key's
// [WithComment] comments. prefix is written before the key ("# " for a
// commented field), and value writes the value.
func (e *encoder) writeKeyValue(path []string, key, prefix string, value func() error) error {
	var keyPath []string
	if e.opts.comments != nil {
		keyPath = append(append([]string{}, path...), key)
	}
	indent := e.indentPrefix(len(path))
	e.writeCommentBlock(indent, e.optionComment(keyPath, HeadCommentPos))
	e.buf.WriteString(indent)
	e.buf.WriteString(prefix)
	e.writeKey(key)
	e.buf.WriteString(" = ")
	if err := value(); err != nil {
		return err
	}
	e.writeLineComment(e.optionComment(keyPath, LineCommentPos))
	e.buf.WriteByte('\n')
	e.writeCommentBlock(indent, e.optionComment(keyPath, FootCommentPos))
	return nil
}

func (e *encoder) encodeTableField(fi fieldInfo, fv reflect.Value, path []string) error {
	subPath := append(append([]string{}, path...), fi.name)
	k, err := e.visit(fv)
	if err != nil {
		return err
	}
	defer e.leave(k)
	for fv.Kind() == reflect.Pointer || fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}

	e.writeCommentBlock(e.headerIndent(subPath), fi.comment)

	if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
		return e.encodeArrayTables(fv, subPath)
	}

	e.writeTableHeader(subPath)
	if err := e.encodeTable(fv, subPath); err != nil {
		return err
	}
	e.writeCommentBlock(e.headerIndent(subPath), e.optionComment(subPath, FootCommentPos))
	return nil
}

// isNil reports whether rv holds no value: it is invalid, or a nil pointer
// or interface. TOML has no null, so such values are left out.
func isNil(rv reflect.Value) bool {
	for rv.Kind() == reflect.Interface && !rv.IsNil() {
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	}
	return false
}

func (e *encoder) encodeValue(rv reflect.Value, fi fieldInfo) error {
	k, err := e.visit(rv)
	if err != nil {
		return err
	}
	defer e.leave(k)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return errNilValue
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return errNilValue
	}

	if rv.Kind() == reflect.Struct && isScalarStruct(rv.Type()) {
		v := rv.Interface()
		if t, ok := v.(time.Time); ok {
			if _, offset := t.Zone(); offset%60 != 0 {
				// TOML offsets have no seconds; UTC keeps the instant.
				v = t.UTC()
			}
		}
		if m, ok := v.(encoding.TextMarshaler); ok {
			text, err := m.MarshalText()
			if err != nil {
				if fi.name != "" {
					return fmt.Errorf("toml: key %q: %w", fi.name, err)
				}
				return fmt.Errorf("toml: %w", err)
			}
			e.buf.Write(text)
			return nil
		}
	}

	if rv.CanInterface() {
		iface := rv.Interface()
		if m, ok := iface.(MarshalerContext); ok {
			val, err := m.MarshalTOML(e.ctx)
			if err != nil {
				return err
			}
			return e.encodeValue(reflect.ValueOf(val), fi)
		}
		if m, ok := iface.(Marshaler); ok {
			val, err := m.MarshalTOML()
			if err != nil {
				return err
			}
			return e.encodeValue(reflect.ValueOf(val), fi)
		}
		if m, ok := iface.(BytesMarshaler); ok {
			data, err := m.MarshalTOML()
			if err != nil {
				return err
			}
			e.buf.Write(data)
			return nil
		}
		if m, ok := iface.(encoding.TextMarshaler); ok {
			text, err := m.MarshalText()
			if err != nil {
				return err
			}
			e.encodeString(string(text), fi)
			return nil
		}
	}

	if rv.CanAddr() && rv.Addr().CanInterface() {
		iface := rv.Addr().Interface()
		if m, ok := iface.(MarshalerContext); ok {
			val, err := m.MarshalTOML(e.ctx)
			if err != nil {
				return err
			}
			return e.encodeValue(reflect.ValueOf(val), fi)
		}
		if m, ok := iface.(Marshaler); ok {
			val, err := m.MarshalTOML()
			if err != nil {
				return err
			}
			return e.encodeValue(reflect.ValueOf(val), fi)
		}
		if m, ok := iface.(BytesMarshaler); ok {
			data, err := m.MarshalTOML()
			if err != nil {
				return err
			}
			e.buf.Write(data)
			return nil
		}
		if m, ok := iface.(encoding.TextMarshaler); ok {
			text, err := m.MarshalText()
			if err != nil {
				return err
			}
			e.encodeString(string(text), fi)
			return nil
		}
	}

	if e.opts.customMarshalers != nil {
		if fn, ok := e.opts.customMarshalers[rv.Type()]; ok {
			result := reflect.ValueOf(fn).Call([]reflect.Value{rv})
			if !result[1].IsNil() {
				if err, ok := reflect.TypeAssert[error](result[1]); ok {
					return err
				}
			}
			e.buf.Write(result[0].Bytes())
			return nil
		}
	}

	switch rv.Kind() {
	case reflect.String:
		e.encodeString(rv.String(), fi)
		return nil
	case reflect.Bool:
		if rv.Bool() {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if rv.Type() == reflect.TypeFor[time.Duration]() {
			e.encodeString(time.Duration(rv.Int()).String(), fi)
			return nil
		}
		e.encodeInt(rv.Int(), fi)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if rv.Uint() > math.MaxInt64 {
			return fmt.Errorf("%w: %d", errUintOverflow, rv.Uint())
		}
		e.encodeUint(rv.Uint(), fi)
		return nil
	case reflect.Float32, reflect.Float64:
		return e.encodeFloat(rv.Float(), rv.Type().Bits())
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			e.encodeString(encodeBase64(rv.Bytes()), fi)
			return nil
		}
		return e.encodeArray(rv, fi)
	case reflect.Array:
		return e.encodeArray(rv, fi)
	case reflect.Struct:
		return e.encodeInlineTable(rv)
	case reflect.Map:
		return e.encodeInlineMap(rv)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedFieldType, rv.Type())
	}
}

func (e *encoder) encodeString(s string, fi fieldInfo) {
	if fi.multiline || (e.opts.multiLineString && strings.Contains(s, "\n")) {
		if fi.literal && canMultiLineLiteral(s) {
			e.writeMultiLineLiteral(s)
		} else {
			e.writeMultiLineBasic(s)
		}
		return
	}

	if (fi.literal || (e.opts.literalString && !e.opts.basicStrings)) && canLiteral(s) {
		e.writeLiteral(s)
		return
	}
	e.writeBasic(s)
}

func (e *encoder) writeBasic(s string) {
	e.buf.WriteByte('"')
	e.buf.WriteString(escapeBasic(s))
	e.buf.WriteByte('"')
}

func (e *encoder) writeMultiLineBasic(s string) {
	e.buf.WriteString("\"\"\"\n")
	e.buf.WriteString(escapeMultiLineBasic(s))
	e.buf.WriteString("\"\"\"")
}

func (e *encoder) writeLiteral(s string) {
	e.buf.WriteByte('\'')
	e.buf.WriteString(s)
	e.buf.WriteByte('\'')
}

func (e *encoder) writeMultiLineLiteral(s string) {
	e.buf.WriteString("'''\n")
	e.buf.WriteString(s)
	e.buf.WriteString("'''")
}

// canLiteral reports whether a literal string can hold s exactly.
func canLiteral(s string) bool {
	return utf8.ValidString(s) && !strings.Contains(s, "'") && !containsControlChars(s)
}

// canMultiLineLiteral reports whether a multi-line literal string can hold s
// exactly. A \r is excluded because decoding turns \r\n into \n.
func canMultiLineLiteral(s string) bool {
	if !utf8.ValidString(s) || strings.Contains(s, "'''") {
		return false
	}
	for _, r := range s {
		if r != '\n' && (r < 0x20 && r != '\t' || r == 0x7f) {
			return false
		}
	}
	return true
}

// encodeInt writes v in the base its tag asks for. TOML has no negative
// hexadecimal, octal or binary integers, so a negative value is written in
// decimal.
func (e *encoder) encodeInt(v int64, fi fieldInfo) {
	if v < 0 {
		e.buf.WriteString(strconv.FormatInt(v, 10))
		return
	}
	e.encodeUint(uint64(v), fi)
}

func (e *encoder) encodeUint(v uint64, fi fieldInfo) {
	switch {
	case fi.hex:
		e.buf.WriteString("0x")
		e.buf.WriteString(strings.ToUpper(strconv.FormatUint(v, 16)))
	case fi.octal:
		e.buf.WriteString("0o")
		e.buf.WriteString(strconv.FormatUint(v, 8))
	case fi.binary:
		e.buf.WriteString("0b")
		e.buf.WriteString(strconv.FormatUint(v, 2))
	default:
		e.buf.WriteString(strconv.FormatUint(v, 10))
	}
}

// encodeFloat writes v with the fewest digits that read back as the same
// float of bitSize bits.
func (e *encoder) encodeFloat(v float64, bitSize int) error {
	switch {
	case math.IsInf(v, 1):
		e.buf.WriteString("inf")
	case math.IsInf(v, -1):
		e.buf.WriteString("-inf")
	case math.IsNaN(v):
		e.buf.WriteString("nan")
	default:
		s := strconv.FormatFloat(v, 'g', -1, bitSize)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		e.buf.WriteString(s)
	}
	return nil
}

func (e *encoder) encodeArray(rv reflect.Value, fi fieldInfo) error {
	if rv.Len() == 0 {
		e.buf.WriteString("[]")
		return nil
	}

	multiline := e.opts.arrayMultiline
	if multiline {
		e.buf.WriteString("[\n")
		for i := range rv.Len() {
			e.buf.WriteString("  ")
			if err := e.encodeValue(rv.Index(i), fi); err != nil {
				return err
			}
			e.buf.WriteString(",\n")
		}
		e.buf.WriteByte(']')
		return nil
	}

	e.buf.WriteByte('[')
	for i := range rv.Len() {
		if i > 0 {
			e.buf.WriteString(", ")
		}
		if err := e.encodeValue(rv.Index(i), fi); err != nil {
			return err
		}
	}
	e.buf.WriteByte(']')
	return nil
}

func (e *encoder) encodeInlineTable(rv reflect.Value) error {
	e.buf.WriteString("{ ")
	sf := getStructFields(rv.Type())
	first := true
	for _, fi := range sf.fields {
		fv, err := rv.FieldByIndexErr(fi.index)
		if err != nil || isNil(fv) {
			continue
		}
		if (fi.omitEmpty || e.opts.omitEmpty) && isEmpty(fv) {
			continue
		}
		if !first {
			e.buf.WriteString(", ")
		}
		e.writeKey(fi.name)
		e.buf.WriteString(" = ")
		if err := e.encodeValue(fv, fi); err != nil {
			return err
		}
		first = false
	}
	e.buf.WriteString(" }")
	return nil
}

func (e *encoder) encodeInlineMap(rv reflect.Value) error {
	if rv.IsNil() {
		e.buf.WriteString("{}")
		return nil
	}
	e.buf.WriteString("{ ")
	keys, names := sortedMapKeys(rv)
	first := true
	for i, k := range keys {
		val := rv.MapIndex(k)
		if isNil(val) {
			continue
		}
		if !first {
			e.buf.WriteString(", ")
		}
		e.writeKey(names[i])
		e.buf.WriteString(" = ")
		if err := e.encodeValue(val, fieldInfo{}); err != nil {
			return err
		}
		first = false
	}
	e.buf.WriteString(" }")
	return nil
}

func (e *encoder) encodeNode(n *node) ([]byte, error) {
	if n == nil {
		return nil, nil
	}
	if err := e.emitNode(n, nil); err != nil {
		return nil, err
	}
	return e.buf.Bytes(), nil
}

func (e *encoder) emitNode(n *node, path []string) error {
	switch n.kind {
	case nodeTable:
		if len(path) > 0 {
			e.writeHeader(path, false, n.headComment, n.comment)
		}
		return e.emitTableBody(n, path)
	case nodeArrayTable:
		e.writeHeader(path, true, n.headComment, n.comment)
		return e.emitTableBody(n, path)
	}
	return nil
}

// emitTableBody writes a table's key/value pairs and comments, then its foot
// comment, then its sub-tables, so that no pair lands under a sub-table's
// header.
func (e *encoder) emitTableBody(n *node, path []string) error {
	for _, child := range n.children {
		switch child.kind {
		case nodeKeyValue:
			if err := e.emitKeyValue(child); err != nil {
				return err
			}
		case nodeComment:
			e.writeCommentBlock("", child.value)
		}
	}
	e.writeCommentBlock("", n.footComment)
	for _, child := range n.children {
		if child.kind == nodeTable || child.kind == nodeArrayTable {
			if err := e.emitNode(child, append(append([]string{}, path...), child.key...)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *encoder) emitKeyValue(n *node) error {
	e.writeCommentBlock("", n.headComment)
	e.writeDottedKey(n.key)
	e.buf.WriteString(" = ")
	if len(n.children) > 0 {
		if err := e.emitValue(n.children[0]); err != nil {
			return err
		}
	}
	e.writeLineComment(n.comment)
	e.buf.WriteByte('\n')
	e.writeCommentBlock("", n.footComment)
	return nil
}

// emitValue writes a value node. A string is written in its style when that
// style can hold it exactly, and as a basic string otherwise; any other
// scalar's text must be a single TOML value of its kind.
func (e *encoder) emitValue(n *node) error {
	switch n.kind {
	case nodeString:
		switch {
		case n.style == styleMultiLineBasic:
			e.writeMultiLineBasic(n.value)
		case n.style == styleMultiLineLiteral && canMultiLineLiteral(n.value):
			e.writeMultiLineLiteral(n.value)
		case n.style == styleMultiLineLiteral:
			e.writeMultiLineBasic(n.value)
		case n.style == styleLiteral && canLiteral(n.value):
			e.writeLiteral(n.value)
		default:
			e.writeBasic(n.value)
		}
	case nodeInteger, nodeFloat, nodeBoolean, nodeDateTime,
		nodeLocalDateTime, nodeLocalDate, nodeLocalTime:
		if err := e.checkScalar(n); err != nil {
			return err
		}
		e.buf.WriteString(n.value)
	case nodeArray:
		e.buf.WriteByte('[')
		for i, child := range n.children {
			if i > 0 {
				e.buf.WriteString(", ")
			}
			if err := e.emitValue(child); err != nil {
				return err
			}
		}
		e.buf.WriteByte(']')
	case nodeInlineTable:
		e.buf.WriteString("{ ")
		for i, child := range n.children {
			if i > 0 {
				e.buf.WriteString(", ")
			}
			e.writeDottedKey(child.key)
			e.buf.WriteString(" = ")
			if len(child.children) > 0 {
				if err := e.emitValue(child.children[0]); err != nil {
					return err
				}
			}
		}
		e.buf.WriteString(" }")
	}
	return nil
}

// scalarTokens maps the kinds of non-string scalar nodes to the token their
// text must scan as.
var scalarTokens = map[nodeKind]tokenKind{
	nodeInteger:       tokenInteger,
	nodeFloat:         tokenFloat,
	nodeBoolean:       tokenBoolean,
	nodeDateTime:      tokenOffsetDateTime,
	nodeLocalDateTime: tokenLocalDateTime,
	nodeLocalDate:     tokenLocalDate,
	nodeLocalTime:     tokenLocalTime,
}

// checkScalar reports an error unless n's text is exactly one TOML value of
// n's kind, so that writing it cannot add or break anything around it. It
// scans the text with one scanner reused across the scalars of a tree.
func (e *encoder) checkScalar(n *node) error {
	if e.scalars == nil {
		e.scalars = &scanner{}
	}
	s := e.scalars
	*s = scanner{
		src:       append(s.src[:0], n.value...),
		tokens:    s.tokens[:0],
		nestStack: s.nestStack[:0],
		line:      1,
		col:       1,
		inValue:   true,
	}
	var err error
	if !strings.HasPrefix(n.value, bom) && !s.atEnd() {
		err = s.scanValue()
	}
	if err != nil || !s.atEnd() || len(s.tokens) != 1 || s.tokens[0].kind != scalarTokens[n.kind] {
		return fmt.Errorf("%w %q", errInvalidScalar, n.value)
	}
	return nil
}

func (e *encoder) indentPrefix(depth int) string {
	if e.opts.indent <= 0 || depth <= 0 {
		return ""
	}
	return strings.Repeat(" ", depth*e.opts.indent)
}

// writeTableHeader writes a [table] header with its [WithComment] head and
// line comments.
func (e *encoder) writeTableHeader(path []string) {
	e.writeHeader(path, false, e.optionComment(path, HeadCommentPos), e.optionComment(path, LineCommentPos))
}

// writeArrayTableHeader writes a [[table]] header; withComments adds its
// [WithComment] head and line comments.
func (e *encoder) writeArrayTableHeader(path []string, withComments bool) {
	var head, line string
	if withComments {
		head, line = e.optionComment(path, HeadCommentPos), e.optionComment(path, LineCommentPos)
	}
	e.writeHeader(path, true, head, line)
}

func (e *encoder) writeHeader(path []string, array bool, head, line string) {
	if e.opts.tableSeparator && e.buf.Len() > 0 {
		e.buf.WriteByte('\n')
	}
	indent := e.headerIndent(path)
	e.writeCommentBlock(indent, head)
	e.buf.WriteString(indent)
	if array {
		e.buf.WriteString("[[")
	} else {
		e.buf.WriteByte('[')
	}
	e.writeDottedKey(path)
	if array {
		e.buf.WriteString("]]")
	} else {
		e.buf.WriteByte(']')
	}
	e.writeLineComment(line)
	e.buf.WriteByte('\n')
}

func (e *encoder) headerIndent(path []string) string {
	if e.opts.indentTables && len(path) > 1 {
		return e.indentPrefix(len(path) - 1)
	}
	return ""
}

func (e *encoder) writeKey(key string) {
	if isBareKey(key) {
		e.buf.WriteString(key)
	} else {
		e.writeBasic(key)
	}
}

func (e *encoder) writeDottedKey(keys []string) {
	for i, k := range keys {
		if i > 0 {
			e.buf.WriteByte('.')
		}
		e.writeKey(k)
	}
}

// optionComment returns the [WithComment] texts at pos for the node at path,
// joined by newlines.
func (e *encoder) optionComment(path []string, pos CommentPosition) string {
	if e.opts.comments == nil {
		return ""
	}
	var texts []string
	for _, c := range e.opts.comments[strings.Join(path, ".")] {
		if c.Position == pos {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// writeCommentBlock writes each line of text as an indented comment line.
func (e *encoder) writeCommentBlock(indent, text string) {
	if text == "" {
		return
	}
	for line := range strings.SplitSeq(text, "\n") {
		e.buf.WriteString(indent)
		e.buf.WriteString(commentText(line))
		e.buf.WriteByte('\n')
	}
}

// writeLineComment writes text as a comment after a value or header.
func (e *encoder) writeLineComment(text string) {
	if text == "" {
		return
	}
	e.buf.WriteByte(' ')
	e.buf.WriteString(commentText(strings.ReplaceAll(text, "\n", " ")))
}

// commentText returns text as a comment: "#" followed by a space unless text
// is empty or already starts with whitespace, so a parsed comment's text is
// written back unchanged. Characters a comment cannot hold, control
// characters other than tab and invalid UTF-8, become spaces and U+FFFD.
func commentText(text string) string {
	text = strings.ToValidUTF8(text, "�")
	if containsControlChars(text) {
		text = strings.Map(func(r rune) rune {
			if isControlChar(r) {
				return ' '
			}
			return r
		}, text)
	}
	if text == "" || text[0] == ' ' || text[0] == '\t' {
		return "#" + text
	}
	return "# " + text
}

func isBareKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !isBareKeyChar(r) {
			return false
		}
	}
	return true
}

// isTableValue reports whether rv is written as a table: a struct or map,
// or a non-empty slice whose elements all are.
func isTableValue(rv reflect.Value) bool {
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct:
		if isScalarStruct(rv.Type()) {
			return false
		}
		if _, ok := reflect.TypeAssert[encoding.TextMarshaler](rv); ok {
			return false
		}
		if _, ok := reflect.TypeAssert[Marshaler](rv); ok {
			return false
		}
		if _, ok := reflect.TypeAssert[MarshalerContext](rv); ok {
			return false
		}
		if _, ok := reflect.TypeAssert[BytesMarshaler](rv); ok {
			return false
		}
		return true
	case reflect.Map:
		return true
	case reflect.Slice:
		if rv.Len() == 0 {
			return false
		}
		for i := range rv.Len() {
			elem := rv.Index(i)
			for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Interface {
				if elem.IsNil() {
					return false
				}
				elem = elem.Elem()
			}
			if elem.Kind() != reflect.Struct && elem.Kind() != reflect.Map || !isTableValue(elem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isScalarStruct(t reflect.Type) bool {
	return t == reflect.TypeFor[time.Time]() ||
		t == reflect.TypeFor[LocalDate]() ||
		t == reflect.TypeFor[LocalTime]() ||
		t == reflect.TypeFor[LocalDateTime]() ||
		t == reflect.TypeFor[big.Int]() ||
		t == reflect.TypeFor[big.Float]()
}

func isEmpty(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return rv.IsNil()
	case reflect.Struct:
		return reflect.DeepEqual(rv.Interface(), reflect.Zero(rv.Type()).Interface())
	}
	return false
}

// needsEscape reports whether a basic string must escape b.
func needsEscape(b byte) bool {
	return b < 0x20 || b == 0x7f || b == '"' || b == '\\'
}

func escapeBasic(s string) string {
	clean := true
	for i := range len(s) {
		if needsEscape(s[i]) {
			clean = false
			break
		}
	}
	if clean && utf8.ValidString(s) {
		return s
	}
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '\b':
			buf.WriteString(`\b`)
		case '\t':
			buf.WriteString(`\t`)
		case '\n':
			buf.WriteString(`\n`)
		case '\f':
			buf.WriteString(`\f`)
		case '\r':
			buf.WriteString(`\r`)
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&buf, `\u%04X`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// escapeMultiLineBasic escapes s for a multi-line basic string. Newlines
// and tabs stay as they are; every third quote of a run is escaped so the
// run cannot close the string.
func escapeMultiLineBasic(s string) string {
	var buf strings.Builder
	quotes := 0
	for _, r := range s {
		if r == '"' {
			quotes++
			if quotes == 3 {
				buf.WriteString(`\"`)
				quotes = 0
				continue
			}
			buf.WriteByte('"')
			continue
		}
		quotes = 0
		switch r {
		case '\b':
			buf.WriteString(`\b`)
		case '\t':
			buf.WriteRune('\t')
		case '\n':
			buf.WriteRune('\n')
		case '\f':
			buf.WriteString(`\f`)
		case '\r':
			buf.WriteString(`\r`)
		case '\\':
			buf.WriteString(`\\`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&buf, `\u%04X`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			return true
		}
	}
	return false
}

func encodeBase64(data []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(data) == 0 {
		return ""
	}
	buf := make([]byte, 0, (len(data)+2)/3*4)
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		buf = append(buf, enc[b0>>2], enc[(b0&0x03)<<4|b1>>4])
		if i+1 < len(data) {
			buf = append(buf, enc[(b1&0x0f)<<2|b2>>6])
		} else {
			buf = append(buf, '=')
		}
		if i+2 < len(data) {
			buf = append(buf, enc[b2&0x3f])
		} else {
			buf = append(buf, '=')
		}
	}
	return string(buf)
}
