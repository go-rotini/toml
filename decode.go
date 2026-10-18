package toml

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Unmarshaler is implemented by types that control their own TOML
// decoding. It is used for values, inline tables, [table] sections and runs of
// [[array]] entries alike.
// The unmarshal function decodes the TOML value into the pointer it is given,
// as [Unmarshal] would; an any receives the form decoding into an any
// produces (string, int64, float64, bool, a date or time type, []any, or
// map[string]any). A value that does not fit returns a [TypeError].
type Unmarshaler interface {
	UnmarshalTOML(unmarshal func(any) error) error
}

// BytesUnmarshaler is implemented by types that decode themselves from TOML
// bytes. For a scalar the bytes are its text (a string's contents, otherwise
// the literal as written); for an array or inline table they are its TOML
// source; for a [table] section they are its contents written as TOML; and for
// a run of [[array]] entries they are the entries written as an inline array
// of inline tables.
type BytesUnmarshaler interface {
	UnmarshalTOML(data []byte) error
}

// UnmarshalerContext is like [Unmarshaler] but receives a context.
type UnmarshalerContext interface {
	UnmarshalTOML(ctx context.Context, unmarshal func(any) error) error
}

type decoder struct {
	opts       *decoderOptions
	ctx        context.Context
	src        []byte       // the scanner's normalized source, which node spans index
	skipHook   reflect.Type // the next unmarshalCustom call for this type is skipped
	typeErrors []string
	path       []string // key path of the value being decoded, for type errors
	at         Position // where that value is written
	keyCount   int
}

func newDecoder(opts *decoderOptions) *decoder {
	return &decoder{
		opts: opts,
		ctx:  context.Background(),
	}
}

func (d *decoder) decode(root *node, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrNilPointer
	}
	rv = rv.Elem()

	if err := d.decodeTable(root, rv); err != nil {
		return err
	}

	if len(d.typeErrors) > 0 {
		return &TypeError{Errors: d.typeErrors}
	}
	return nil
}

func (d *decoder) decodeTable(n *node, rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if handled, err := d.unmarshalCustom(n, rv); handled {
		return err
	}

	switch rv.Kind() {
	case reflect.Struct:
		return d.decodeTableToStruct(n, rv)
	case reflect.Map:
		return d.decodeTableToMap(n, rv)
	case reflect.Interface:
		if rv.NumMethod() > 0 {
			d.typeError(fmt.Sprintf("cannot decode table into %s", rv.Type()))
			return nil
		}
		v, err := d.tableToInterface(n)
		if err != nil {
			return err
		}
		rv.Set(reflect.ValueOf(v))
		return nil
	default:
		d.typeError(fmt.Sprintf("cannot decode table into %s", rv.Type()))
		return nil
	}
}

// tableToInterface returns a table as decoding into an any produces it: a
// map[string]any, or a [MapSlice] with [WithOrderedMap].
func (d *decoder) tableToInterface(n *node) (any, error) {
	if d.opts.useOrderedMap {
		return d.toMapSlice(n)
	}
	m := make(map[string]any)
	if err := d.decodeTableToGenericMap(n, m); err != nil {
		return nil, err
	}
	return m, nil
}

// arrayTableRuns groups n's [[array]] entries by key, in order, or returns
// nil when n has none.
func arrayTableRuns(n *node) map[string][]*node {
	var runs map[string][]*node
	for _, child := range n.children {
		if child.kind == nodeArrayTable && len(child.key) > 0 {
			if runs == nil {
				runs = make(map[string][]*node)
			}
			runs[child.key[0]] = append(runs[child.key[0]], child)
		}
	}
	return runs
}

func (d *decoder) countKey(child *node) error {
	d.keyCount++
	if d.opts.maxKeys > 0 && d.keyCount > d.opts.maxKeys {
		return &SyntaxError{Message: "maximum key count exceeded", Pos: child.pos}
	}
	return nil
}

func (d *decoder) decodeTableToStruct(n *node, rv reflect.Value) error {
	sf := getStructFields(rv.Type())
	runs := arrayTableRuns(n)

	assigned := make([]bool, len(sf.fields))
	for _, child := range n.children {
		if len(child.key) == 0 {
			continue
		}
		key := child.key[0]
		switch child.kind {
		case nodeKeyValue:
			if err := d.countKey(child); err != nil {
				return err
			}
		case nodeTable:
		case nodeArrayTable:
			if runs[key][0] != child {
				continue
			}
		default:
			continue
		}

		idx, ok := sf.byName[key]
		if !ok {
			idx = d.caseInsensitiveLookup(sf, key)
		}
		if idx < 0 {
			if d.opts.strict {
				return &UnknownFieldError{Field: key, Pos: child.pos}
			}
			continue
		}
		assigned[idx] = true
		fv, ok := d.field(rv, sf.fields[idx].index)
		if !ok {
			continue
		}

		if err := d.decodeChild(child, runs, fv); err != nil {
			return err
		}
	}

	if err := d.decodeAbsent(sf, assigned, rv, "", n.pos, nil); err != nil {
		return err
	}

	if d.opts.validator != nil {
		if err := d.opts.validator.Struct(rv.Addr().Interface()); err != nil {
			return &ValidationError{Err: err, Pos: n.pos}
		}
	}

	return nil
}

// decodeAbsent handles the fields of the struct rv that no key set: a
// required field is an error and a default is applied. A struct field that
// is absent too, or a non-nil pointer to one, is handled the same way, so
// its fields' tags apply although its table is missing. prefix is the dotted
// path of rv's table, for messages, and pos the position of the nearest
// table present in the input.
func (d *decoder) decodeAbsent(sf *structFields, assigned []bool, rv reflect.Value, prefix string, pos Position, seen map[uintptr]bool) error {
	for i, fi := range sf.fields {
		if i < len(assigned) && assigned[i] {
			continue
		}
		if fi.required {
			return &SyntaxError{
				Message: fmt.Sprintf("required key %q is missing", prefix+fi.name),
				Pos:     pos,
			}
		}
		if fi.hasDefault && d.opts.applyDefault != nil {
			fv, ok := d.field(rv, fi.index)
			if !ok {
				continue
			}
			if err := d.opts.applyDefault(d, fi, fv); err != nil {
				return err
			}
			continue
		}

		fv, err := rv.FieldByIndexErr(fi.index)
		if err != nil {
			continue
		}
		for fv.Kind() == reflect.Pointer && !fv.IsNil() {
			if seen == nil {
				seen = make(map[uintptr]bool)
			}
			if seen[fv.Pointer()] {
				break
			}
			seen[fv.Pointer()] = true
			fv = fv.Elem()
		}
		if fv.Kind() != reflect.Struct || !fv.CanAddr() || d.decodesItself(fv) {
			continue
		}
		if err := d.decodeAbsent(getStructFields(fv.Type()), nil, fv, prefix+fi.name+".", pos, seen); err != nil {
			return err
		}
	}
	return nil
}

// decodesItself reports whether the addressable struct rv decodes through
// a hook, is one of the struct types decoded as a scalar, or cannot be
// inspected for hooks.
func (d *decoder) decodesItself(rv reflect.Value) bool {
	if _, ok := d.opts.customUnmarshalers[rv.Type()]; ok || isScalarStruct(rv.Type()) || !rv.Addr().CanInterface() {
		return true
	}
	switch rv.Addr().Interface().(type) {
	case Unmarshaler, UnmarshalerContext, BytesUnmarshaler, encoding.TextUnmarshaler:
		return true
	}
	return false
}

// field returns the struct field of rv at index, allocating any nil embedded
// struct pointer on the way. It reports false, recording a type error, when
// such a pointer cannot be set because its type is unexported.
func (d *decoder) field(rv reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				if !rv.CanSet() {
					d.typeError(fmt.Sprintf("cannot set embedded pointer to unexported struct %s", rv.Type().Elem()))
					return reflect.Value{}, false
				}
				rv.Set(reflect.New(rv.Type().Elem()))
			}
			rv = rv.Elem()
		}
		rv = rv.Field(x)
	}
	return rv, true
}

// mapKey converts a TOML key to a map key of type t, through its
// [encoding.TextUnmarshaler] when it has one and otherwise as a string.
func mapKey(t reflect.Type, key string) (reflect.Value, error) {
	k := reflect.New(t)
	if u, ok := reflect.TypeAssert[encoding.TextUnmarshaler](k); ok {
		if err := u.UnmarshalText([]byte(key)); err != nil {
			return reflect.Value{}, err
		}
		return k.Elem(), nil
	}
	if t.Kind() == reflect.String {
		k.Elem().SetString(key)
		return k.Elem(), nil
	}
	return reflect.Value{}, fmt.Errorf("%w: %s", errUnsupportedKeyType, t)
}

func (d *decoder) decodeTableToMap(n *node, rv reflect.Value) error {
	keyType := rv.Type().Key()
	if keyType.Kind() != reflect.String && !reflect.PointerTo(keyType).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		d.typeError(fmt.Sprintf("map key type %s is not supported (must be string or TextUnmarshaler)", keyType))
		return nil
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	valType := rv.Type().Elem()
	runs := arrayTableRuns(n)

	for _, child := range n.children {
		if len(child.key) == 0 {
			continue
		}
		key := child.key[0]
		switch child.kind {
		case nodeKeyValue:
			if err := d.countKey(child); err != nil {
				return err
			}
		case nodeTable:
		case nodeArrayTable:
			if runs[key][0] != child {
				continue
			}
		default:
			continue
		}

		k, err := mapKey(keyType, key)
		if err != nil {
			leave := d.enter(key, child.pos)
			d.typeError(fmt.Sprintf("invalid map key: %v", err))
			leave()
			continue
		}
		mapVal := reflect.New(valType).Elem()
		if err := d.decodeChild(child, runs, mapVal); err != nil {
			return err
		}
		rv.SetMapIndex(k, mapVal)
	}
	return nil
}

func (d *decoder) decodeTableToGenericMap(n *node, m map[string]any) error {
	for _, child := range n.children {
		if len(child.key) == 0 {
			continue
		}
		key := child.key[0]
		switch child.kind {
		case nodeKeyValue:
			if err := d.countKey(child); err != nil {
				return err
			}
			if len(child.children) > 0 {
				val, err := d.nodeToInterface(child.children[0])
				if err != nil {
					return err
				}
				m[key] = val
			}
		case nodeTable:
			sub := make(map[string]any)
			if err := d.decodeTableToGenericMap(child, sub); err != nil {
				return err
			}
			m[key] = sub
		case nodeArrayTable:
			sub := make(map[string]any)
			if err := d.decodeTableToGenericMap(child, sub); err != nil {
				return err
			}
			arr, _ := m[key].([]any)
			m[key] = append(arr, sub)
		}
	}
	return nil
}

// decodeArrayTables decodes a run of [[array]] entries with the same key the
// way an array value is decoded: into a slice, replacing its contents, an
// array of the same length, or an empty interface as a []any.
func (d *decoder) decodeArrayTables(entries []*node, rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if d.mayHook(rv) {
		// The run is one value, so a hook on the target sees it as the
		// array it would see for an inline array of tables.
		run := &node{kind: nodeArray, pos: entries[0].pos, children: entries}
		if handled, err := d.unmarshalCustom(run, rv); handled {
			return err
		}
	}

	switch rv.Kind() {
	case reflect.Slice:
		slice := reflect.MakeSlice(rv.Type(), len(entries), len(entries))
		for i, entry := range entries {
			if err := d.decodeElem(i, entry, slice.Index(i)); err != nil {
				return err
			}
		}
		rv.Set(slice)
	case reflect.Array:
		if rv.Len() != len(entries) {
			d.typeError(fmt.Sprintf("array of %d tables does not match target [%d]%s",
				len(entries), rv.Len(), rv.Type().Elem()))
			return nil
		}
		for i, entry := range entries {
			if err := d.decodeElem(i, entry, rv.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if rv.NumMethod() > 0 {
			d.typeError(fmt.Sprintf("cannot decode array of tables into %s", rv.Type()))
			return nil
		}
		arr := make([]any, len(entries))
		for i, entry := range entries {
			v, err := d.tableToInterface(entry)
			if err != nil {
				return err
			}
			arr[i] = v
		}
		rv.Set(reflect.ValueOf(arr))
	default:
		d.typeError(fmt.Sprintf("cannot decode array of tables into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeValue(n *node, rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if n.kind == nodeInlineTable || n.kind == nodeTable {
		return d.decodeTable(n, rv)
	}
	if handled, err := d.unmarshalCustom(n, rv); handled {
		return err
	}
	if rv.Kind() == reflect.Interface && rv.NumMethod() > 0 {
		val, err := d.nodeToInterface(n)
		if err != nil {
			return err
		}
		if vv := reflect.ValueOf(val); vv.Type().AssignableTo(rv.Type()) {
			rv.Set(vv)
		} else {
			d.typeError(fmt.Sprintf("cannot decode %T into %s", val, rv.Type()))
		}
		return nil
	}

	switch n.kind {
	case nodeString:
		return d.decodeString(n.value, rv)
	case nodeInteger:
		return d.decodeInteger(n.value, rv, n.pos)
	case nodeFloat:
		return d.decodeFloat(n.value, rv, n.pos)
	case nodeBoolean:
		return d.decodeBoolean(n.value, rv)
	case nodeDateTime:
		return d.decodeDateTime(n.value, rv)
	case nodeLocalDateTime:
		return d.decodeLocalDateTime(n.value, rv)
	case nodeLocalDate:
		return d.decodeLocalDate(n.value, rv)
	case nodeLocalTime:
		return d.decodeLocalTime(n.value, rv)
	case nodeArray:
		return d.decodeArray(n, rv)
	default:
		d.typeError(fmt.Sprintf("unexpected node kind %d", n.kind))
		return nil
	}
}

func (d *decoder) decodeString(s string, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.String:
		rv.SetString(s)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			data, err := decodeBase64(s)
			if err != nil {
				d.typeError(fmt.Sprintf("invalid base64: %v", err))
				return nil
			}
			rv.SetBytes(data)
		} else {
			d.typeError(fmt.Sprintf("cannot decode string into %s", rv.Type()))
		}
	case reflect.Int64:
		if rv.Type() == reflect.TypeFor[time.Duration]() {
			dur, err := time.ParseDuration(s)
			if err != nil {
				d.typeError(fmt.Sprintf("cannot parse duration %q: %v", s, err))
				return nil
			}
			rv.SetInt(int64(dur))
		} else {
			d.typeError(fmt.Sprintf("cannot decode string into %s", rv.Type()))
		}
	case reflect.Interface:
		rv.Set(reflect.ValueOf(s))
	default:
		d.typeError(fmt.Sprintf("cannot decode string into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeInteger(s string, rv reflect.Value, pos Position) error {
	i64, err := parseInteger(s)
	if err != nil {
		d.typeError(fmt.Sprintf("invalid integer %q: %v", s, err))
		return nil
	}

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if rv.OverflowInt(i64) {
			return &OverflowError{Value: s, Type: rv.Type().String(), Pos: pos}
		}
		rv.SetInt(i64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if i64 < 0 || rv.OverflowUint(uint64(i64)) {
			return &OverflowError{Value: s, Type: rv.Type().String(), Pos: pos}
		}
		rv.SetUint(uint64(i64))
	case reflect.Float32, reflect.Float64:
		rv.SetFloat(float64(i64))
	case reflect.Interface:
		rv.Set(reflect.ValueOf(i64))
	default:
		if rv.Type() == reflect.TypeFor[*big.Int]() {
			bi := new(big.Int)
			bi.SetInt64(i64)
			rv.Set(reflect.ValueOf(bi))
			return nil
		}
		d.typeError(fmt.Sprintf("cannot decode integer into %s", rv.Type()))
	}
	return nil
}

// parseInteger converts the text of a TOML integer, which the scanner has
// checked fits in an int64.
func parseInteger(s string) (int64, error) {
	s = stripUnderscores(s)
	base := 10
	if len(s) > 2 && s[0] == '0' {
		switch s[1] {
		case 'x':
			base = 16
		case 'o':
			base = 8
		case 'b':
			base = 2
		}
	}
	if base != 10 {
		s = s[2:]
	}
	return strconv.ParseInt(s, base, 64)
}

// float32Value rounds the text s, whose float64 value f is already
// known, to a float32 in one step, so text that rounds to MaxFloat32
// decodes and no value is rounded twice. It reports overflow when a
// finite s rounds to an infinity. An infinite or NaN f is returned as is.
func float32Value(s string, f float64) (float64, bool) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return f, false
	}
	f32, err := strconv.ParseFloat(s, 32)
	return f32, err != nil
}

func (d *decoder) decodeFloat(s string, rv reflect.Value, pos Position) error {
	f, err := parseFloat(s)
	if err != nil {
		d.typeError(fmt.Sprintf("invalid float %q: %v", s, err))
		return nil
	}

	switch rv.Kind() {
	case reflect.Float32:
		f32, overflow := float32Value(stripUnderscores(s), f)
		if overflow {
			return &OverflowError{Value: s, Type: rv.Type().String(), Pos: pos}
		}
		rv.SetFloat(f32)
	case reflect.Float64:
		rv.SetFloat(f)
	case reflect.Interface:
		rv.Set(reflect.ValueOf(f))
	default:
		if rv.Type() == reflect.TypeFor[*big.Float]() {
			bf := new(big.Float).SetFloat64(f)
			rv.Set(reflect.ValueOf(bf))
			return nil
		}
		d.typeError(fmt.Sprintf("cannot decode float into %s", rv.Type()))
	}
	return nil
}

// parseFloat converts the text of a TOML float.
func parseFloat(s string) (float64, error) {
	s = stripUnderscores(s)
	switch s {
	case "inf", "+inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	case "nan", "+nan", "-nan":
		return math.NaN(), nil
	default:
		return strconv.ParseFloat(s, 64)
	}
}

func (d *decoder) decodeBoolean(s string, rv reflect.Value) error {
	b := s == "true"
	switch rv.Kind() {
	case reflect.Bool:
		rv.SetBool(b)
	case reflect.Interface:
		rv.Set(reflect.ValueOf(b))
	default:
		d.typeError(fmt.Sprintf("cannot decode boolean into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeDateTime(s string, rv reflect.Value) error {
	t, err := parseDateTime(s)
	if err != nil {
		d.typeError(err.Error())
		return nil
	}

	switch {
	case rv.Type() == reflect.TypeFor[time.Time]():
		rv.Set(reflect.ValueOf(t))
	case rv.Kind() == reflect.Interface:
		rv.Set(reflect.ValueOf(t))
	default:
		d.typeError(fmt.Sprintf("cannot decode datetime into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalDateTime(s string, rv reflect.Value) error {
	s = normalizeDateTimeSeparators(s)

	switch {
	case rv.Type() == reflect.TypeFor[LocalDateTime]():
		var ldt LocalDateTime
		if err := ldt.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local datetime %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ldt))
	case rv.Type() == reflect.TypeFor[time.Time]():
		var ldt LocalDateTime
		if err := ldt.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local datetime %q", s))
			return nil
		}
		rv.Set(reflect.ValueOf(ldt.AsTime(d.opts.localTimeZone)))
	case rv.Kind() == reflect.Interface:
		var ldt LocalDateTime
		if err := ldt.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local datetime %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ldt))
	default:
		d.typeError(fmt.Sprintf("cannot decode local datetime into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalDate(s string, rv reflect.Value) error {
	switch {
	case rv.Type() == reflect.TypeFor[LocalDate]():
		var ld LocalDate
		if err := ld.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local date %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ld))
	case rv.Type() == reflect.TypeFor[time.Time]():
		t, err := time.ParseInLocation("2006-01-02", s, d.opts.localTimeZone)
		if err != nil {
			d.typeError(fmt.Sprintf("invalid local date %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(t))
	case rv.Kind() == reflect.Interface:
		var ld LocalDate
		if err := ld.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local date %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ld))
	default:
		d.typeError(fmt.Sprintf("cannot decode local date into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalTime(s string, rv reflect.Value) error {
	switch {
	case rv.Type() == reflect.TypeFor[LocalTime]():
		var lt LocalTime
		if err := lt.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local time %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(lt))
	case rv.Kind() == reflect.Interface:
		var lt LocalTime
		if err := lt.UnmarshalText([]byte(s)); err != nil {
			d.typeError(fmt.Sprintf("invalid local time %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(lt))
	default:
		d.typeError(fmt.Sprintf("cannot decode local time into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeArray(n *node, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Slice:
		slice := reflect.MakeSlice(rv.Type(), len(n.children), len(n.children))
		for i, child := range n.children {
			if err := d.decodeElem(i, child, slice.Index(i)); err != nil {
				return err
			}
		}
		rv.Set(slice)
	case reflect.Array:
		if rv.Len() != len(n.children) {
			d.typeError(fmt.Sprintf("array length %d does not match target [%d]%s",
				len(n.children), rv.Len(), rv.Type().Elem()))
			return nil
		}
		for i, child := range n.children {
			if err := d.decodeElem(i, child, rv.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		arr := make([]any, len(n.children))
		for i, child := range n.children {
			val, err := d.nodeToInterface(child)
			if err != nil {
				return err
			}
			arr[i] = val
		}
		rv.Set(reflect.ValueOf(arr))
	default:
		d.typeError(fmt.Sprintf("cannot decode array into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) nodeToInterface(n *node) (any, error) {
	switch n.kind {
	case nodeString:
		return n.value, nil
	case nodeInteger:
		return parseInteger(n.value)
	case nodeFloat:
		return parseFloat(n.value)
	case nodeBoolean:
		return n.value == "true", nil
	case nodeDateTime:
		t, err := parseDateTime(n.value)
		if err != nil {
			return t, fmt.Errorf("toml: %w", err)
		}
		return t, nil
	case nodeLocalDateTime:
		var ldt LocalDateTime
		s := normalizeDateTimeSeparators(n.value)
		err := ldt.UnmarshalText([]byte(s))
		return ldt, err
	case nodeLocalDate:
		var ld LocalDate
		err := ld.UnmarshalText([]byte(n.value))
		return ld, err
	case nodeLocalTime:
		var lt LocalTime
		err := lt.UnmarshalText([]byte(n.value))
		return lt, err
	case nodeArray:
		arr := make([]any, len(n.children))
		for i, child := range n.children {
			v, err := d.nodeToInterface(child)
			if err != nil {
				return nil, err
			}
			arr[i] = v
		}
		return arr, nil
	case nodeInlineTable, nodeTable, nodeArrayTable:
		return d.tableToInterface(n)
	default:
		return n.value, nil
	}
}

// toMapSlice returns a table as a [MapSlice] in source order. An array of
// tables becomes a []any of MapSlice values at the position of its first
// entry.
func (d *decoder) toMapSlice(n *node) (MapSlice, error) {
	var ms MapSlice
	var arrays map[string]int // index in ms of each array of tables
	for _, child := range n.children {
		if len(child.key) == 0 {
			continue
		}
		key := child.key[0]
		switch child.kind {
		case nodeKeyValue:
			if err := d.countKey(child); err != nil {
				return nil, err
			}
			var val any
			if len(child.children) > 0 {
				v, err := d.nodeToInterface(child.children[0])
				if err != nil {
					return nil, err
				}
				val = v
			}
			ms = append(ms, MapItem{Key: key, Value: val})
		case nodeTable:
			sub, err := d.toMapSlice(child)
			if err != nil {
				return nil, err
			}
			ms = append(ms, MapItem{Key: key, Value: sub})
		case nodeArrayTable:
			sub, err := d.toMapSlice(child)
			if err != nil {
				return nil, err
			}
			if i, ok := arrays[key]; ok {
				arr, _ := ms[i].Value.([]any)
				arr = append(arr, sub)
				ms[i].Value = arr
				continue
			}
			if arrays == nil {
				arrays = make(map[string]int)
			}
			arrays[key] = len(ms)
			ms = append(ms, MapItem{Key: key, Value: []any{sub}})
		}
	}
	return ms, nil
}

func (d *decoder) applyDefault(fi fieldInfo, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(fi.defaultValue)
	case reflect.Bool:
		switch fi.defaultValue {
		case "true":
			fv.SetBool(true)
		case "false":
			fv.SetBool(false)
		default:
			return &DefaultError{Field: fi.name, Message: errInvalidBoolDefault.Error()}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeFor[time.Duration]() {
			dur, err := time.ParseDuration(fi.defaultValue)
			if err != nil {
				return &DefaultError{Field: fi.name, Message: errInvalidDurationDefault.Error()}
			}
			fv.SetInt(int64(dur))
			return nil
		}
		i, err := strconv.ParseInt(fi.defaultValue, 10, 64)
		if err != nil {
			return &DefaultError{Field: fi.name, Message: errInvalidIntDefault.Error()}
		}
		if fv.OverflowInt(i) {
			return &DefaultError{Field: fi.name, Message: errDefaultOutOfRange.Error()}
		}
		fv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(fi.defaultValue, 10, 64)
		if err != nil {
			return &DefaultError{Field: fi.name, Message: errInvalidUintDefault.Error()}
		}
		if fv.OverflowUint(u) {
			return &DefaultError{Field: fi.name, Message: errDefaultOutOfRange.Error()}
		}
		fv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(fi.defaultValue, 64)
		if err != nil {
			return &DefaultError{Field: fi.name, Message: errInvalidFloatDefault.Error()}
		}
		if fv.Kind() == reflect.Float32 {
			var overflow bool
			if f, overflow = float32Value(fi.defaultValue, f); overflow {
				return &DefaultError{Field: fi.name, Message: errDefaultOutOfRange.Error()}
			}
		}
		fv.SetFloat(f)
	default:
		return &DefaultError{Field: fi.name, Message: errUnsupportedDefault.Error()}
	}
	return nil
}

func (d *decoder) caseInsensitiveLookup(sf *structFields, key string) int {
	lower := strings.ToLower(key)
	for i, fi := range sf.fields {
		if strings.EqualFold(fi.name, lower) {
			return i
		}
	}
	return -1
}

// unmarshalCustom decodes n with the first hook rv's type provides: an
// [UnmarshalerContext], [Unmarshaler], [BytesUnmarshaler], an
// [encoding.TextUnmarshaler] (scalars, dates and times), a [WithCustomUnmarshaler]
// function, or, with [WithJSONUnmarshaler], a [json.Unmarshaler]. It reports
// whether a hook handled the value.
//
// [time.Time], the local date and time types, [math/big.Int] and
// [math/big.Float] keep their own decoding of dates and times.
func (d *decoder) unmarshalCustom(n *node, rv reflect.Value) (bool, error) {
	if d.skipHook != nil && d.skipHook == rv.Type() {
		d.skipHook = nil
		return false, nil
	}
	if rv.CanAddr() && rv.Addr().CanInterface() {
		iface := rv.Addr().Interface()
		if u, ok := iface.(UnmarshalerContext); ok {
			return true, u.UnmarshalTOML(d.ctx, d.unmarshalFunc(n, rv.Type()))
		}
		if u, ok := iface.(Unmarshaler); ok {
			return true, u.UnmarshalTOML(d.unmarshalFunc(n, rv.Type()))
		}
		if u, ok := iface.(BytesUnmarshaler); ok {
			data, err := d.rawBytes(n)
			if err != nil {
				return true, err
			}
			return true, u.UnmarshalTOML(data)
		}
		if u, ok := iface.(encoding.TextUnmarshaler); ok && textNode(n, rv.Type()) {
			return true, u.UnmarshalText([]byte(n.value))
		}
	}

	if fn, ok := d.opts.customUnmarshalers[rv.Type()]; ok {
		data, err := d.rawBytes(n)
		if err != nil {
			return true, err
		}
		ptr := reflect.New(rv.Type())
		result := reflect.ValueOf(fn).Call([]reflect.Value{ptr, reflect.ValueOf(data)})
		if !result[0].IsNil() {
			if err, ok := reflect.TypeAssert[error](result[0]); ok {
				return true, err
			}
		}
		rv.Set(ptr.Elem())
		return true, nil
	}

	if d.opts.unmarshalJSON != nil && !isScalarStruct(rv.Type()) && rv.CanAddr() && rv.Addr().CanInterface() {
		if u, ok := reflect.TypeAssert[json.Unmarshaler](rv.Addr()); ok {
			return true, d.opts.unmarshalJSON(d, n, u)
		}
	}
	return false, nil
}

// mayHook reports whether unmarshalCustom could handle an array or table
// decoded into rv, so callers can skip building its input otherwise.
func (d *decoder) mayHook(rv reflect.Value) bool {
	t := rv.Type()
	if d.skipHook == t {
		return true
	}
	if _, ok := d.opts.customUnmarshalers[t]; ok {
		return true
	}
	if !rv.CanAddr() {
		return false
	}
	pt := reflect.PointerTo(t)
	return pt.Implements(reflect.TypeFor[UnmarshalerContext]()) ||
		pt.Implements(reflect.TypeFor[Unmarshaler]()) ||
		pt.Implements(reflect.TypeFor[BytesUnmarshaler]()) ||
		d.opts.unmarshalJSON != nil && pt.Implements(reflect.TypeFor[json.Unmarshaler]())
}

// unmarshalFunc returns the callback handed to an [Unmarshaler]. The value
// is stored as is when it is assignable to the target and decoded into the
// target otherwise; a value that does not fit returns a [TypeError]. Decoding
// into the unmarshaler's own type, self, skips its hook so it cannot recurse.
func (d *decoder) unmarshalFunc(n *node, self reflect.Type) func(any) error {
	return func(v any) error {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Pointer || rv.IsNil() {
			return errNonPointerTarget
		}
		val, err := d.nodeToInterface(n)
		if err != nil {
			return err
		}
		if vv := reflect.ValueOf(val); vv.IsValid() && vv.Type().AssignableTo(rv.Elem().Type()) {
			rv.Elem().Set(vv)
			return nil
		}
		mark := len(d.typeErrors)
		if rv.Elem().Type() == self {
			d.skipHook = self
		}
		switch n.kind {
		case nodeTable, nodeArrayTable:
			err = d.decodeTable(n, rv.Elem())
		default:
			err = d.decodeValue(n, rv.Elem())
		}
		d.skipHook = nil
		if err != nil {
			return err
		}
		if len(d.typeErrors) > mark {
			errs := append([]string(nil), d.typeErrors[mark:]...)
			d.typeErrors = d.typeErrors[:mark]
			return &TypeError{Errors: errs}
		}
		return nil
	}
}

// unmarshalJSON passes the JSON encoding of n to u. Tables become JSON
// objects even when [WithOrderedMap] is set.
func (d *decoder) unmarshalJSON(n *node, u json.Unmarshaler) error {
	opts := *d.opts
	opts.useOrderedMap = false
	plain := &decoder{opts: &opts, ctx: d.ctx, src: d.src}
	val, err := plain.nodeToInterface(n)
	if err != nil {
		return err
	}
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("toml: json: %w", err)
	}
	if err := u.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("toml: %w", err)
	}
	return nil
}

// rawBytes returns what a [BytesUnmarshaler] or custom unmarshaler receives
// for n: a scalar's text, an array's or inline table's TOML source, and a
// table's contents written as TOML.
func (d *decoder) rawBytes(n *node) ([]byte, error) {
	switch n.kind {
	case nodeArray, nodeInlineTable:
		if n.span.start < n.span.end && n.span.end <= len(d.src) {
			return bytes.Clone(d.src[n.span.start:n.span.end]), nil
		}
		e := newEncoder(defaultEncodeOptions())
		err := e.emitValue(asInline(n))
		return e.buf.Bytes(), err
	case nodeTable, nodeArrayTable:
		e := newEncoder(defaultEncodeOptions())
		err := e.emitTableBody(n, nil)
		return e.buf.Bytes(), err
	default:
		return []byte(n.value), nil
	}
}

// asInline returns n with every table and array-of-tables entry in it
// rewritten as an inline table, so it can be written as a single value. A
// run of [[array]] entries decoded as one value is an array of such entries.
func asInline(n *node) *node {
	switch n.kind {
	case nodeArray:
		out := &node{kind: nodeArray, pos: n.pos, children: make([]*node, len(n.children))}
		for i, child := range n.children {
			out.children[i] = asInline(child)
		}
		return out
	case nodeTable, nodeArrayTable:
		out := &node{kind: nodeInlineTable, pos: n.pos}
		runs := arrayTableRuns(n)
		for _, child := range n.children {
			if len(child.key) == 0 {
				continue
			}
			switch child.kind {
			case nodeKeyValue:
				out.children = append(out.children, child)
			case nodeTable:
				out.children = append(out.children, &node{kind: nodeKeyValue, key: child.key, children: []*node{asInline(child)}})
			case nodeArrayTable:
				if run := runs[child.key[0]]; run[0] == child {
					arr := asInline(&node{kind: nodeArray, children: run})
					out.children = append(out.children, &node{kind: nodeKeyValue, key: child.key, children: []*node{arr}})
				}
			}
		}
		return out
	default:
		return n
	}
}

// parseDateTime converts the text of a TOML offset date-time. A leap second
// (:60) rolls over to the next minute, as [time.Time] cannot hold it.
func parseDateTime(s string) (time.Time, error) {
	s, leap := cutLeapSecond(normalizeDateTimeSeparators(s), 17)
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return t, fmt.Errorf("invalid datetime %q: %w", s, err)
	}
	if leap {
		t = t.Add(time.Second)
	}
	return t, nil
}

// cutLeapSecond reports whether the seconds at s[i:i+2] are a leap second
// (60) and, if so, returns s with them replaced by 59.
func cutLeapSecond(s string, i int) (string, bool) {
	if len(s) >= i+2 && s[i-1] == ':' && s[i:i+2] == "60" {
		return s[:i] + "59" + s[i+2:], true
	}
	return s, false
}

func normalizeDateTimeSeparators(s string) string {
	s = strings.Replace(s, " ", "T", 1)
	if idx := strings.IndexByte(s, 'T'); idx == -1 {
		s = strings.Replace(s, "t", "T", 1)
	}
	if strings.HasSuffix(s, "z") {
		s = s[:len(s)-1] + "Z"
	}
	s = normalizeNoSeconds(s)
	return s
}

// normalizeNoSeconds inserts ":00" seconds into datetime strings like
// "2024-01-01T12:30Z" that omit the seconds component.
func normalizeNoSeconds(s string) string {
	tIdx := strings.IndexByte(s, 'T')
	if tIdx == -1 {
		return s
	}
	i := tIdx + 1
	if i+5 > len(s) || s[i+2] != ':' {
		return s
	}
	i += 5 // past HH:MM
	if i < len(s) && s[i] == ':' {
		return s
	}
	if i < len(s) && s[i] == '.' {
		return s
	}
	return s[:i] + ":00" + s[i:]
}

func decodeBase64(s string) ([]byte, error) {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	s = strings.TrimRight(s, "=")
	buf := make([]byte, 0, len(s)*3/4)
	var val uint32
	var bits int
	for _, ch := range s {
		idx := strings.IndexRune(enc, ch)
		if idx < 0 {
			if ch == '\n' || ch == '\r' || ch == ' ' {
				continue
			}
			return nil, fmt.Errorf("%w: %q", errInvalidBase64Char, ch)
		}
		val = val<<6 | uint32(idx)
		bits += 6
		if bits >= 8 {
			bits -= 8
			buf = append(buf, byte(val>>bits))
			val &= (1 << bits) - 1
		}
	}
	return buf, nil
}

// textNode reports whether n is a scalar handed to an
// [encoding.TextUnmarshaler] of type t: a string, number or boolean, or a
// date or time unless t is one of the types decoded from dates directly.
func textNode(n *node, t reflect.Type) bool {
	switch n.kind {
	case nodeString, nodeInteger, nodeFloat, nodeBoolean:
		return true
	case nodeDateTime, nodeLocalDateTime, nodeLocalDate, nodeLocalTime:
		return !isScalarStruct(t)
	}
	return false
}

// typeError records a value that does not fit its target, prefixed with the
// line and key path of the value being decoded.
func (d *decoder) typeError(msg string) {
	var b strings.Builder
	if d.at.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", d.at.Line)
	}
	if len(d.path) > 0 {
		b.WriteString("key ")
		var key strings.Builder
		for i, seg := range d.path {
			if i > 0 && !strings.HasPrefix(seg, "[") {
				key.WriteByte('.')
			}
			key.WriteString(seg)
		}
		fmt.Fprintf(&b, "%q: ", key.String())
	}
	b.WriteString(msg)
	d.typeErrors = append(d.typeErrors, b.String())
}

// enter records that decoding moves into the value at key, written at pos;
// the returned function moves back out.
func (d *decoder) enter(key string, pos Position) func() {
	at := d.at
	d.path = append(d.path, key)
	d.at = pos
	return func() {
		d.path = d.path[:len(d.path)-1]
		d.at = at
	}
}

// decodeChild decodes a table's child into rv: a key/value pair's value, a
// sub-table, or the run of [[array]] entries the child starts.
func (d *decoder) decodeChild(child *node, runs map[string][]*node, rv reflect.Value) error {
	defer d.enter(child.key[0], child.pos)()
	switch child.kind {
	case nodeKeyValue:
		if len(child.children) > 0 {
			return d.decodeValue(child.children[0], rv)
		}
		return nil
	case nodeTable:
		return d.decodeTable(child, rv)
	default:
		return d.decodeArrayTables(runs[child.key[0]], rv)
	}
}

// decodeElem decodes the i-th element of an array or array of tables.
func (d *decoder) decodeElem(i int, n *node, rv reflect.Value) error {
	defer d.enter("["+strconv.Itoa(i)+"]", n.pos)()
	if n.kind == nodeArrayTable {
		return d.decodeTable(n, rv)
	}
	return d.decodeValue(n, rv)
}
