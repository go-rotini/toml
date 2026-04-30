package toml

import (
	"context"
	"encoding"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Unmarshaler is implemented by types that need structured access to
// their TOML data during decoding.
type Unmarshaler interface {
	UnmarshalTOML(unmarshal func(any) error) error
}

// BytesUnmarshaler is implemented by types that can decode themselves
// from raw TOML bytes.
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
	typeErrors []string
	depth      int
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
	d.depth++
	if d.opts.maxDepth > 0 && d.depth > d.opts.maxDepth {
		return &SyntaxError{Message: "maximum nesting depth exceeded", Pos: n.pos}
	}
	defer func() { d.depth-- }()

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		return d.decodeTableToStruct(n, rv)
	case reflect.Map:
		return d.decodeTableToMap(n, rv)
	case reflect.Interface:
		m := make(map[string]any)
		if err := d.decodeTableToGenericMap(n, m); err != nil {
			return err
		}
		if d.opts.useOrderedMap {
			rv.Set(reflect.ValueOf(d.toMapSlice(n)))
		} else {
			rv.Set(reflect.ValueOf(m))
		}
		return nil
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode table into %s", rv.Type()))
		return nil
	}
}

func (d *decoder) decodeTableToStruct(n *node, rv reflect.Value) error {
	sf := getStructFields(rv.Type())

	assigned := make(map[string]bool)
	for _, child := range n.children {
		if child.kind == nodeComment {
			continue
		}
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			key := child.key[0]
			assigned[key] = true
			d.keyCount++
			if d.opts.maxKeys > 0 && d.keyCount > d.opts.maxKeys {
				return &SyntaxError{Message: "maximum key count exceeded", Pos: child.pos}
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
			fi := sf.fields[idx]
			fv := rv.FieldByIndex(fi.index)
			if len(child.children) > 0 {
				if err := d.decodeValue(child.children[0], fv); err != nil {
					return err
				}
			}
		}
		if child.kind == nodeTable && len(child.key) > 0 {
			key := child.key[0]
			assigned[key] = true
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
			fi := sf.fields[idx]
			fv := rv.FieldByIndex(fi.index)
			if err := d.decodeTable(child, fv); err != nil {
				return err
			}
		}
		if child.kind == nodeArrayTable && len(child.key) > 0 {
			key := child.key[0]
			assigned[key] = true
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
			fi := sf.fields[idx]
			fv := rv.FieldByIndex(fi.index)
			if err := d.decodeArrayTable(child, fv); err != nil {
				return err
			}
		}
	}

	// Check required fields and apply defaults
	for _, fi := range sf.fields {
		if assigned[fi.name] {
			continue
		}
		if fi.required {
			return &SyntaxError{
				Message: fmt.Sprintf("required key %q is missing", fi.name),
				Pos:     n.pos,
			}
		}
		if fi.hasDefault && d.opts.applyDefaults {
			fv := rv.FieldByIndex(fi.index)
			if err := d.applyDefault(fi, fv); err != nil {
				return err
			}
		}
	}

	// Validate struct
	if d.opts.validator != nil {
		if err := d.opts.validator.Struct(rv.Addr().Interface()); err != nil {
			return &ValidationError{Err: err, Pos: n.pos}
		}
	}

	return nil
}

func (d *decoder) decodeTableToMap(n *node, rv reflect.Value) error {
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	keyType := rv.Type().Key()
	if keyType.Kind() != reflect.String {
		if !keyType.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) &&
			!reflect.PointerTo(keyType).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("map key type %s is not supported (must be string or TextUnmarshaler)", keyType))
			return nil
		}
	}
	valType := rv.Type().Elem()

	for _, child := range n.children {
		if child.kind == nodeComment {
			continue
		}
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			d.keyCount++
			if d.opts.maxKeys > 0 && d.keyCount > d.opts.maxKeys {
				return &SyntaxError{Message: "maximum key count exceeded", Pos: child.pos}
			}
			key := child.key[0]
			mapKey := reflect.New(keyType).Elem()
			if keyType.Kind() == reflect.String {
				mapKey.SetString(key)
			} else {
				if err := setTextUnmarshalerKey(mapKey, key); err != nil {
					return err
				}
			}
			mapVal := reflect.New(valType).Elem()
			if len(child.children) > 0 {
				if err := d.decodeValue(child.children[0], mapVal); err != nil {
					return err
				}
			}
			rv.SetMapIndex(mapKey, mapVal)
		}
		if child.kind == nodeTable && len(child.key) > 0 {
			key := child.key[0]
			mapKey := reflect.New(keyType).Elem()
			if keyType.Kind() == reflect.String {
				mapKey.SetString(key)
			}
			mapVal := reflect.New(valType).Elem()
			if err := d.decodeTable(child, mapVal); err != nil {
				return err
			}
			rv.SetMapIndex(mapKey, mapVal)
		}
		if child.kind == nodeArrayTable && len(child.key) > 0 {
			key := child.key[0]
			mapKey := reflect.New(keyType).Elem()
			if keyType.Kind() == reflect.String {
				mapKey.SetString(key)
			}
			if valType.Kind() == reflect.Interface {
				m := make(map[string]any)
				if err := d.decodeTableToGenericMap(child, m); err != nil {
					return err
				}
				existing := rv.MapIndex(mapKey)
				if existing.IsValid() && existing.Elem().IsValid() {
					if arr, ok := existing.Elem().Interface().([]any); ok {
						rv.SetMapIndex(mapKey, reflect.ValueOf(append(arr, m)))
					} else {
						rv.SetMapIndex(mapKey, reflect.ValueOf([]any{existing.Elem().Interface(), m}))
					}
				} else {
					rv.SetMapIndex(mapKey, reflect.ValueOf([]any{m}))
				}
			} else {
				mapVal := rv.MapIndex(mapKey)
				if !mapVal.IsValid() {
					mapVal = reflect.New(valType).Elem()
				}
				if err := d.decodeArrayTable(child, mapVal); err != nil {
					return err
				}
				rv.SetMapIndex(mapKey, mapVal)
			}
		}
	}
	return nil
}

func (d *decoder) decodeTableToGenericMap(n *node, m map[string]any) error {
	for _, child := range n.children {
		if child.kind == nodeComment {
			continue
		}
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			d.keyCount++
			if d.opts.maxKeys > 0 && d.keyCount > d.opts.maxKeys {
				return &SyntaxError{Message: "maximum key count exceeded", Pos: child.pos}
			}
			key := child.key[0]
			if len(child.children) > 0 {
				val, err := d.nodeToInterface(child.children[0])
				if err != nil {
					return err
				}
				m[key] = val
			}
		}
		if child.kind == nodeTable && len(child.key) > 0 {
			d.depth++
			if d.opts.maxDepth > 0 && d.depth > d.opts.maxDepth {
				return &SyntaxError{Message: "maximum nesting depth exceeded", Pos: child.pos}
			}
			key := child.key[0]
			sub := make(map[string]any)
			if err := d.decodeTableToGenericMap(child, sub); err != nil {
				d.depth--
				return err
			}
			d.depth--
			m[key] = sub
		}
		if child.kind == nodeArrayTable && len(child.key) > 0 {
			d.depth++
			if d.opts.maxDepth > 0 && d.depth > d.opts.maxDepth {
				return &SyntaxError{Message: "maximum nesting depth exceeded", Pos: child.pos}
			}
			key := child.key[0]
			sub := make(map[string]any)
			if err := d.decodeTableToGenericMap(child, sub); err != nil {
				d.depth--
				return err
			}
			d.depth--
			existing, ok := m[key]
			if ok {
				if arr, ok2 := existing.([]any); ok2 {
					m[key] = append(arr, sub)
				} else {
					m[key] = []any{existing, sub}
				}
			} else {
				m[key] = []any{sub}
			}
		}
	}
	return nil
}

func (d *decoder) decodeArrayTable(n *node, rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Slice {
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode array of tables into %s", rv.Type()))
		return nil
	}

	elem := reflect.New(rv.Type().Elem()).Elem()
	if err := d.decodeTable(n, elem); err != nil {
		return err
	}
	rv.Set(reflect.Append(rv, elem))
	return nil
}

func (d *decoder) decodeValue(n *node, rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	// Check custom unmarshalers
	if rv.CanAddr() && rv.Addr().CanInterface() {
		iface := rv.Addr().Interface()
		if u, ok := iface.(UnmarshalerContext); ok {
			return u.UnmarshalTOML(d.ctx, func(v any) error {
				val, err := d.nodeToInterface(n)
				if err != nil {
					return err
				}
				return setInterface(v, val)
			})
		}
		if u, ok := iface.(Unmarshaler); ok {
			return u.UnmarshalTOML(func(v any) error {
				val, err := d.nodeToInterface(n)
				if err != nil {
					return err
				}
				return setInterface(v, val)
			})
		}
		if u, ok := iface.(BytesUnmarshaler); ok {
			raw := nodeRawBytes(n)
			return u.UnmarshalTOML(raw)
		}
		if u, ok := iface.(encoding.TextUnmarshaler); ok {
			if n.kind == nodeString || n.kind == nodeInteger || n.kind == nodeFloat || n.kind == nodeBoolean {
				return u.UnmarshalText([]byte(n.value))
			}
		}
	}

	// Check custom unmarshaler registry
	if d.opts.customUnmarshalers != nil {
		if fn, ok := d.opts.customUnmarshalers[rv.Type()]; ok {
			raw := nodeRawBytes(n)
			ptr := reflect.New(rv.Type())
			result := reflect.ValueOf(fn).Call([]reflect.Value{ptr, reflect.ValueOf(raw)})
			if !result[0].IsNil() {
				return result[0].Interface().(error)
			}
			rv.Set(ptr.Elem())
			return nil
		}
	}

	switch n.kind {
	case nodeString:
		return d.decodeString(n.value, rv)
	case nodeInteger:
		return d.decodeInteger(n.value, rv, n.pos)
	case nodeFloat:
		return d.decodeFloat(n.value, rv)
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
	case nodeInlineTable:
		return d.decodeTable(n, rv)
	case nodeTable:
		return d.decodeTable(n, rv)
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("unexpected node kind %d", n.kind))
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
				d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid base64: %v", err))
				return nil
			}
			rv.SetBytes(data)
		} else {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode string into %s", rv.Type()))
		}
	case reflect.Int64:
		if rv.Type() == reflect.TypeOf(time.Duration(0)) {
			dur, err := time.ParseDuration(s)
			if err != nil {
				d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot parse duration %q: %v", s, err))
				return nil
			}
			rv.SetInt(int64(dur))
		} else {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode string into %s", rv.Type()))
		}
	case reflect.Interface:
		rv.Set(reflect.ValueOf(s))
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode string into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeInteger(s string, rv reflect.Value, pos Position) error {
	s = stripUnderscores(s)

	var i64 int64
	var u64 uint64
	var isUnsigned bool
	var err error

	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		u64, err = strconv.ParseUint(s[2:], 16, 64)
		i64 = int64(u64)
	case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
		u64, err = strconv.ParseUint(s[2:], 8, 64)
		i64 = int64(u64)
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		u64, err = strconv.ParseUint(s[2:], 2, 64)
		i64 = int64(u64)
	default:
		i64, err = strconv.ParseInt(s, 10, 64)
		u64 = uint64(i64)
		isUnsigned = false
	}
	_ = isUnsigned
	if err != nil {
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid integer %q: %v", s, err))
		return nil
	}

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if rv.OverflowInt(i64) {
			return &OverflowError{Value: s, Type: rv.Type().String(), Pos: pos}
		}
		rv.SetInt(i64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if rv.OverflowUint(u64) {
			return &OverflowError{Value: s, Type: rv.Type().String(), Pos: pos}
		}
		rv.SetUint(u64)
	case reflect.Float32, reflect.Float64:
		rv.SetFloat(float64(i64))
	case reflect.Interface:
		rv.Set(reflect.ValueOf(i64))
	default:
		if rv.Type() == reflect.TypeOf((*big.Int)(nil)) {
			bi := new(big.Int)
			bi.SetInt64(i64)
			rv.Set(reflect.ValueOf(bi))
			return nil
		}
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode integer into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeFloat(s string, rv reflect.Value) error {
	s = stripUnderscores(s)

	var f float64
	switch s {
	case "inf", "+inf":
		f = math.Inf(1)
	case "-inf":
		f = math.Inf(-1)
	case "nan", "+nan", "-nan":
		f = math.NaN()
	default:
		var err error
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid float %q: %v", s, err))
			return nil
		}
	}

	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		rv.SetFloat(f)
	case reflect.Interface:
		rv.Set(reflect.ValueOf(f))
	default:
		if rv.Type() == reflect.TypeOf((*big.Float)(nil)) {
			bf := new(big.Float).SetFloat64(f)
			rv.Set(reflect.ValueOf(bf))
			return nil
		}
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode float into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeBoolean(s string, rv reflect.Value) error {
	b := s == "true"
	switch rv.Kind() {
	case reflect.Bool:
		rv.SetBool(b)
	case reflect.Interface:
		rv.Set(reflect.ValueOf(b))
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode boolean into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeDateTime(s string, rv reflect.Value) error {
	// Normalize TOML datetime to RFC3339
	s = strings.Replace(s, " ", "T", 1)
	s = strings.Replace(s, "t", "T", 1)
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid datetime %q: %v", s, err))
		return nil
	}

	switch {
	case rv.Type() == reflect.TypeOf(time.Time{}):
		rv.Set(reflect.ValueOf(t))
	case rv.Kind() == reflect.Interface:
		rv.Set(reflect.ValueOf(t))
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode datetime into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalDateTime(s string, rv reflect.Value) error {
	s = strings.Replace(s, " ", "T", 1)
	s = strings.Replace(s, "t", "T", 1)

	switch {
	case rv.Type() == reflect.TypeOf(LocalDateTime{}):
		var ldt LocalDateTime
		if err := ldt.UnmarshalText([]byte(s)); err != nil {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid local datetime %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ldt))
	case rv.Type() == reflect.TypeOf(time.Time{}):
		layouts := []string{
			"2006-01-02T15:04:05.999999999",
			"2006-01-02T15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, s, d.opts.localTimeZone); err == nil {
				rv.Set(reflect.ValueOf(t))
				return nil
			}
		}
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid local datetime %q", s))
	case rv.Kind() == reflect.Interface:
		var ldt LocalDateTime
		if err := ldt.UnmarshalText([]byte(s)); err == nil {
			rv.Set(reflect.ValueOf(ldt))
		}
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode local datetime into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalDate(s string, rv reflect.Value) error {
	switch {
	case rv.Type() == reflect.TypeOf(LocalDate{}):
		var ld LocalDate
		if err := ld.UnmarshalText([]byte(s)); err != nil {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid local date %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(ld))
	case rv.Type() == reflect.TypeOf(time.Time{}):
		t, err := time.ParseInLocation("2006-01-02", s, d.opts.localTimeZone)
		if err != nil {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid local date %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(t))
	case rv.Kind() == reflect.Interface:
		var ld LocalDate
		if err := ld.UnmarshalText([]byte(s)); err == nil {
			rv.Set(reflect.ValueOf(ld))
		}
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode local date into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeLocalTime(s string, rv reflect.Value) error {
	switch {
	case rv.Type() == reflect.TypeOf(LocalTime{}):
		var lt LocalTime
		if err := lt.UnmarshalText([]byte(s)); err != nil {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("invalid local time %q: %v", s, err))
			return nil
		}
		rv.Set(reflect.ValueOf(lt))
	case rv.Kind() == reflect.Interface:
		var lt LocalTime
		if err := lt.UnmarshalText([]byte(s)); err == nil {
			rv.Set(reflect.ValueOf(lt))
		}
	default:
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode local time into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) decodeArray(n *node, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Slice:
		slice := reflect.MakeSlice(rv.Type(), len(n.children), len(n.children))
		for i, child := range n.children {
			if err := d.decodeValue(child, slice.Index(i)); err != nil {
				return err
			}
		}
		rv.Set(slice)
	case reflect.Array:
		if rv.Len() != len(n.children) {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("array length %d does not match target [%d]%s",
				len(n.children), rv.Len(), rv.Type().Elem()))
			return nil
		}
		for i, child := range n.children {
			if err := d.decodeValue(child, rv.Index(i)); err != nil {
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
		d.typeErrors = append(d.typeErrors, fmt.Sprintf("cannot decode array into %s", rv.Type()))
	}
	return nil
}

func (d *decoder) nodeToInterface(n *node) (any, error) {
	switch n.kind {
	case nodeString:
		return n.value, nil
	case nodeInteger:
		s := stripUnderscores(n.value)
		switch {
		case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
			v, err := strconv.ParseInt(s[2:], 16, 64)
			return v, err
		case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
			v, err := strconv.ParseInt(s[2:], 8, 64)
			return v, err
		case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
			v, err := strconv.ParseInt(s[2:], 2, 64)
			return v, err
		default:
			v, err := strconv.ParseInt(s, 10, 64)
			return v, err
		}
	case nodeFloat:
		s := stripUnderscores(n.value)
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
	case nodeBoolean:
		return n.value == "true", nil
	case nodeDateTime:
		s := strings.Replace(n.value, " ", "T", 1)
		s = strings.Replace(s, "t", "T", 1)
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t, err = time.Parse(time.RFC3339, s)
		}
		return t, err
	case nodeLocalDateTime:
		var ldt LocalDateTime
		s := strings.Replace(n.value, " ", "T", 1)
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
	case nodeInlineTable, nodeTable:
		if d.opts.useOrderedMap {
			return d.toMapSlice(n), nil
		}
		m := make(map[string]any)
		if err := d.decodeTableToGenericMap(n, m); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return n.value, nil
	}
}

func (d *decoder) toMapSlice(n *node) MapSlice {
	var ms MapSlice
	for _, child := range n.children {
		if child.kind == nodeComment {
			continue
		}
		if child.kind == nodeKeyValue && len(child.key) > 0 {
			var val any
			if len(child.children) > 0 {
				val, _ = d.nodeToInterface(child.children[0])
			}
			ms = append(ms, MapItem{Key: child.key[0], Value: val})
		}
		if child.kind == nodeTable && len(child.key) > 0 {
			sub := d.toMapSlice(child)
			ms = append(ms, MapItem{Key: child.key[0], Value: sub})
		}
	}
	return ms
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
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
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
		fv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(fi.defaultValue, 10, 64)
		if err != nil {
			return &DefaultError{Field: fi.name, Message: errInvalidUintDefault.Error()}
		}
		fv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(fi.defaultValue, 64)
		if err != nil {
			return &DefaultError{Field: fi.name, Message: errInvalidFloatDefault.Error()}
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
		if strings.ToLower(fi.name) == lower {
			return i
		}
	}
	return -1
}

func setInterface(target any, value any) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer {
		return fmt.Errorf("toml: non-pointer target")
	}
	rv = rv.Elem()
	rv.Set(reflect.ValueOf(value))
	return nil
}

func setTextUnmarshalerKey(rv reflect.Value, key string) error {
	if rv.CanAddr() {
		if u, ok := rv.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return u.UnmarshalText([]byte(key))
		}
	}
	ptr := reflect.New(rv.Type())
	if u, ok := ptr.Interface().(encoding.TextUnmarshaler); ok {
		if err := u.UnmarshalText([]byte(key)); err != nil {
			return err
		}
		rv.Set(ptr.Elem())
		return nil
	}
	return fmt.Errorf("toml: cannot set key type %s", rv.Type())
}

func nodeRawBytes(n *node) []byte {
	return []byte(n.value)
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
			return nil, fmt.Errorf("invalid base64 character %q", ch)
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
