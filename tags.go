package toml

import (
	"reflect"
	"slices"
	"strings"
	"sync"
)

type fieldInfo struct {
	name         string
	index        []int
	omitEmpty    bool
	inline       bool
	multiline    bool
	literal      bool
	commented    bool
	hex          bool
	octal        bool
	binary       bool
	required     bool
	skip         bool
	defaultValue string
	hasDefault   bool
	comment      string
}

func parseTag(tag string) fieldInfo {
	var fi fieldInfo
	if tag == "" {
		return fi
	}

	parts := strings.Split(tag, ",")
	fi.name = parts[0]
	if fi.name == "-" {
		fi.skip = true
		return fi
	}

	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty", "omitzero":
			fi.omitEmpty = true
		case "inline":
			fi.inline = true
		case "multiline":
			fi.multiline = true
		case "literal":
			fi.literal = true
		case "commented":
			fi.commented = true
		case "hex":
			fi.hex = true
		case "octal":
			fi.octal = true
		case "binary":
			fi.binary = true
		case "required":
			fi.required = true
		default:
			if v, ok := strings.CutPrefix(opt, "default="); ok {
				fi.defaultValue = v
				fi.hasDefault = true
			}
		}
	}
	return fi
}

type structFields struct {
	fields []fieldInfo
	byName map[string]int
}

var structFieldCache sync.Map

func getStructFields(t reflect.Type) *structFields {
	if cached, ok := structFieldCache.Load(t); ok {
		sf, _ := cached.(*structFields)
		return sf
	}
	sf := &structFields{
		byName: make(map[string]int),
	}
	collectFields(t, nil, []reflect.Type{t}, sf)
	structFieldCache.Store(t, sf)
	return sf
}

// collectFields adds the fields of t to sf. expanding holds the struct types
// being flattened on the current path, so a type that embeds itself, directly
// or through others, is expanded only once.
func collectFields(t reflect.Type, index []int, expanding []reflect.Type, sf *structFields) {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}

		tag := f.Tag.Get("toml")
		if tag == "" {
			tag = f.Tag.Get("json")
		}

		fi := parseTag(tag)
		if fi.skip {
			continue
		}

		if commentTag := f.Tag.Get("comment"); commentTag != "" {
			fi.comment = commentTag
		}

		fi.index = make([]int, len(index)+1)
		copy(fi.index, index)
		fi.index[len(index)] = i

		if ft, ok := flattened(f, fi); ok {
			if !slices.Contains(expanding, ft) {
				collectFields(ft, fi.index, append(expanding, ft), sf)
			}
			continue
		}
		if !f.IsExported() {
			// An unexported embedded field is only reachable for the
			// exported fields of a struct it flattens, handled above.
			continue
		}
		if fi.inline && f.Type.Kind() == reflect.Map {
			sf.fields = append(sf.fields, fi)
			sf.byName[fi.name] = len(sf.fields) - 1
			continue
		}

		if fi.name == "" {
			fi.name = strings.ToLower(f.Name)
		}

		if idx, exists := sf.byName[fi.name]; exists {
			// A shallower field wins; at the same depth the first one does.
			if len(fi.index) < len(sf.fields[idx].index) {
				sf.fields[idx] = fi
			}
			continue
		}

		sf.fields = append(sf.fields, fi)
		sf.byName[fi.name] = len(sf.fields) - 1
	}
}

// flattened returns the struct type whose fields f merges into its parent:
// an untagged embedded struct or a struct tagged inline, or a pointer to one.
func flattened(f reflect.StructField, fi fieldInfo) (reflect.Type, bool) {
	if !fi.inline && (!f.Anonymous || fi.name != "") {
		return nil, false
	}
	ft := f.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	return ft, ft.Kind() == reflect.Struct
}
