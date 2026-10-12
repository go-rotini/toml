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
	"strings"
	"time"
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
		fv := rv.FieldByIndex(fi.index)
		if fi.omitEmpty && isEmpty(fv) {
			continue
		}
		if e.opts.omitEmpty && isEmpty(fv) {
			continue
		}
		if isTableValue(fv) && !fi.inline && !e.opts.inlineTable {
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

func (e *encoder) encodeMap(rv reflect.Value, path []string) error {
	if rv.IsNil() {
		return nil
	}

	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})

	type mapEntry struct {
		key   string
		value reflect.Value
	}
	var simples, tables []mapEntry

	for _, k := range keys {
		key := fmt.Sprint(k.Interface())
		val := rv.MapIndex(k)
		for val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		if val.IsValid() && isTableValue(val) && !e.opts.inlineTable {
			tables = append(tables, mapEntry{key, val})
		} else {
			simples = append(simples, mapEntry{key, val})
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
		val := entry.value
		for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		subPath := append(append([]string{}, path...), entry.key)
		if val.Kind() == reflect.Slice && val.Len() > 0 {
			elem := val.Index(0)
			for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Interface {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct || elem.Kind() == reflect.Map {
				if err := e.encodeArrayTables(val, subPath); err != nil {
					return err
				}
				continue
			}
		}
		e.writeTableHeader(subPath)
		if err := e.encodeTable(val, subPath); err != nil {
			return err
		}
		e.writeCommentBlock(e.headerIndent(subPath), e.optionComment(subPath, FootCommentPos))
	}

	return nil
}

// encodeArrayTables writes each element of rv as a [[table]] entry. The
// [WithComment] head and line comments go on the first header and the foot
// comment after the last entry.
func (e *encoder) encodeArrayTables(rv reflect.Value, path []string) error {
	for i := range rv.Len() {
		e.writeArrayTableHeader(path, i == 0)
		if err := e.encodeTable(rv.Index(i), path); err != nil {
			return err
		}
	}
	e.writeCommentBlock(e.headerIndent(path), e.optionComment(path, FootCommentPos))
	return nil
}

func (e *encoder) encodeField(fi fieldInfo, fv reflect.Value, path []string) error {
	if fi.comment != "" {
		e.buf.WriteString(e.indentPrefix(len(path)))
		e.buf.WriteString("# ")
		e.buf.WriteString(fi.comment)
		e.buf.WriteByte('\n')
	}
	prefix := ""
	if fi.commented {
		prefix = "# "
	}
	return e.writeKeyValue(path, fi.name, prefix, func() error {
		return e.encodeValue(fv, fi)
	})
}

// writeKeyValue writes one key/value line under path, with the key's
// [WithComment] comments. prefix is written before the key ("# " for a
// commented field), and value writes the value.
func (e *encoder) writeKeyValue(path []string, key, prefix string, value func() error) error {
	keyPath := append(append([]string{}, path...), key)
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
	for fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil
		}
		fv = fv.Elem()
	}
	for fv.Kind() == reflect.Interface {
		fv = fv.Elem()
	}

	subPath := append(append([]string{}, path...), fi.name)

	if fv.Kind() == reflect.Slice {
		return e.encodeArrayTables(fv, subPath)
	}

	if fi.comment != "" {
		e.buf.WriteString(e.headerIndent(subPath))
		e.buf.WriteString("# ")
		e.buf.WriteString(fi.comment)
		e.buf.WriteByte('\n')
	}
	e.writeTableHeader(subPath)
	if err := e.encodeTable(fv, subPath); err != nil {
		return err
	}
	e.writeCommentBlock(e.headerIndent(subPath), e.optionComment(subPath, FootCommentPos))
	return nil
}

func (e *encoder) encodeValue(rv reflect.Value, fi fieldInfo) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			e.buf.WriteString(`""`)
			return nil
		}
		rv = rv.Elem()
	}
	for rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	if !rv.IsValid() {
		e.buf.WriteString(`""`)
		return nil
	}

	if rv.Kind() == reflect.Struct && isScalarStruct(rv.Type()) {
		if m, ok := reflect.TypeAssert[encoding.TextMarshaler](rv); ok {
			text, err := m.MarshalText()
			if err != nil {
				return err
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
		e.encodeUint(rv.Uint(), fi)
		return nil
	case reflect.Float32, reflect.Float64:
		return e.encodeFloat(rv.Float())
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
		if fi.literal && !strings.Contains(s, "'''") {
			e.buf.WriteString("'''\n")
			e.buf.WriteString(s)
			e.buf.WriteString("'''")
			return
		}
		e.buf.WriteString("\"\"\"\n")
		e.buf.WriteString(escapeMultiLineBasic(s))
		e.buf.WriteString("\"\"\"")
		return
	}

	if fi.literal || (e.opts.literalString && !e.opts.basicStrings) {
		if !strings.Contains(s, "'") && !containsControlChars(s) {
			e.buf.WriteByte('\'')
			e.buf.WriteString(s)
			e.buf.WriteByte('\'')
			return
		}
	}

	e.buf.WriteByte('"')
	e.buf.WriteString(escapeBasic(s))
	e.buf.WriteByte('"')
}

func (e *encoder) encodeInt(v int64, fi fieldInfo) {
	switch {
	case fi.hex:
		fmt.Fprintf(&e.buf, "0x%X", v)
	case fi.octal:
		fmt.Fprintf(&e.buf, "0o%o", v)
	case fi.binary:
		fmt.Fprintf(&e.buf, "0b%b", v)
	default:
		fmt.Fprintf(&e.buf, "%d", v)
	}
}

func (e *encoder) encodeUint(v uint64, fi fieldInfo) {
	switch {
	case fi.hex:
		fmt.Fprintf(&e.buf, "0x%X", v)
	case fi.octal:
		fmt.Fprintf(&e.buf, "0o%o", v)
	case fi.binary:
		fmt.Fprintf(&e.buf, "0b%b", v)
	default:
		fmt.Fprintf(&e.buf, "%d", v)
	}
}

func (e *encoder) encodeFloat(v float64) error {
	switch {
	case math.IsInf(v, 1):
		e.buf.WriteString("inf")
	case math.IsInf(v, -1):
		e.buf.WriteString("-inf")
	case math.IsNaN(v):
		e.buf.WriteString("nan")
	default:
		s := fmt.Sprintf("%v", v)
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
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
		fv := rv.FieldByIndex(fi.index)
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
	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})
	for i, k := range keys {
		if i > 0 {
			e.buf.WriteString(", ")
		}
		key := fmt.Sprint(k.Interface())
		e.writeKey(key)
		e.buf.WriteString(" = ")
		if err := e.encodeValue(rv.MapIndex(k), fieldInfo{}); err != nil {
			return err
		}
	}
	e.buf.WriteString(" }")
	return nil
}

func (e *encoder) encodeNode(n *node) ([]byte, error) {
	if n == nil {
		return nil, nil
	}
	e.emitNode(n, nil)
	return e.buf.Bytes(), nil
}

func (e *encoder) emitNode(n *node, path []string) {
	switch n.kind {
	case nodeTable:
		if len(path) > 0 {
			e.writeHeader(path, false, n.headComment, n.comment)
		}
		e.emitTableBody(n, path)
	case nodeArrayTable:
		e.writeHeader(path, true, n.headComment, n.comment)
		e.emitTableBody(n, path)
	}
}

// emitTableBody writes a table's key/value pairs and comments, then its foot
// comment, then its sub-tables, so that no pair lands under a sub-table's
// header.
func (e *encoder) emitTableBody(n *node, path []string) {
	for _, child := range n.children {
		switch child.kind {
		case nodeKeyValue:
			e.emitKeyValue(child)
		case nodeComment:
			e.writeCommentBlock("", child.value)
		}
	}
	e.writeCommentBlock("", n.footComment)
	for _, child := range n.children {
		if child.kind == nodeTable || child.kind == nodeArrayTable {
			e.emitNode(child, append(append([]string{}, path...), child.key...))
		}
	}
}

func (e *encoder) emitKeyValue(n *node) {
	e.writeCommentBlock("", n.headComment)
	e.writeDottedKey(n.key)
	e.buf.WriteString(" = ")
	if len(n.children) > 0 {
		e.emitValue(n.children[0])
	}
	e.writeLineComment(n.comment)
	e.buf.WriteByte('\n')
	e.writeCommentBlock("", n.footComment)
}

func (e *encoder) emitValue(n *node) {
	switch n.kind {
	case nodeString:
		switch n.style {
		case styleBasic:
			e.buf.WriteByte('"')
			e.buf.WriteString(escapeBasic(n.value))
			e.buf.WriteByte('"')
		case styleMultiLineBasic:
			e.buf.WriteString("\"\"\"\n")
			e.buf.WriteString(escapeMultiLineBasic(n.value))
			e.buf.WriteString("\"\"\"")
		case styleLiteral:
			e.buf.WriteByte('\'')
			e.buf.WriteString(n.value)
			e.buf.WriteByte('\'')
		case styleMultiLineLiteral:
			e.buf.WriteString("'''\n")
			e.buf.WriteString(n.value)
			e.buf.WriteString("'''")
		}
	case nodeInteger, nodeFloat, nodeBoolean, nodeDateTime,
		nodeLocalDateTime, nodeLocalDate, nodeLocalTime:
		e.buf.WriteString(n.value)
	case nodeArray:
		e.buf.WriteByte('[')
		for i, child := range n.children {
			if i > 0 {
				e.buf.WriteString(", ")
			}
			e.emitValue(child)
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
				e.emitValue(child.children[0])
			}
		}
		e.buf.WriteString(" }")
	}
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
		e.buf.WriteByte('"')
		e.buf.WriteString(escapeBasic(key))
		e.buf.WriteByte('"')
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
// written back unchanged.
func commentText(text string) string {
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

func isTableValue(rv reflect.Value) bool {
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct:
		if rv.Type() == reflect.TypeFor[time.Time]() ||
			rv.Type() == reflect.TypeFor[LocalDate]() ||
			rv.Type() == reflect.TypeFor[LocalTime]() ||
			rv.Type() == reflect.TypeFor[LocalDateTime]() ||
			rv.Type() == reflect.TypeFor[big.Int]() ||
			rv.Type() == reflect.TypeFor[big.Float]() {
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
		elem := rv.Index(0)
		for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Interface {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct || elem.Kind() == reflect.Map {
			if elem.Kind() == reflect.Struct {
				return !isScalarStruct(elem.Type())
			}
			return true
		}
		return false
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

func escapeBasic(s string) string {
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

func escapeMultiLineBasic(s string) string {
	var buf strings.Builder
	for _, r := range s {
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
