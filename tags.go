package toml

import (
	"reflect"
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
	fields    []fieldInfo
	byName    map[string]int
	conflicts []string
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
	collectFields(t, nil, sf)
	structFieldCache.Store(t, sf)
	return sf
}

func collectFields(t reflect.Type, index []int, sf *structFields) {
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

		if f.Anonymous && fi.name == "" && !fi.inline {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				collectFields(ft, fi.index, sf)
				continue
			}
		}

		if fi.inline {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				collectFields(ft, fi.index, sf)
				continue
			}
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
			existing := sf.fields[idx]
			if len(fi.index) == len(existing.index) {
				sf.conflicts = append(sf.conflicts, fi.name)
			} else if len(fi.index) < len(existing.index) {
				sf.fields[idx] = fi
			}
			continue
		}

		sf.fields = append(sf.fields, fi)
		sf.byName[fi.name] = len(sf.fields) - 1
	}
}
