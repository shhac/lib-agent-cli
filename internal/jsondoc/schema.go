package jsondoc

import (
	"reflect"
	"strings"
)

// jsonFields maps a struct's json names to their types, flattening embedded
// structs exactly as encoding/json does, so a promoted field is spelled at
// the level the document actually holds it.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	if t == nil || t.Kind() != reflect.Struct {
		return out
	}
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		// An embedded struct's exported fields are promoted to this level,
		// and encoding/json does that even when the embedded TYPE is
		// unexported -- so this cannot test IsExported before recursing, or a
		// promoted field reads as a key no schema claims. A tag name makes an
		// embedded field ordinary, which is why that case falls through.
		if f.Anonymous && name == "" {
			if embedded := deref(f.Type); embedded != nil && embedded.Kind() == reflect.Struct {
				for k, v := range jsonFields(embedded) {
					out[k] = v
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}

// node is how the schema continues below a field: what, if anything,
// describes the keys of an object stored there.
type node int

const (
	// leafNode: the schema says nothing below this field, so its value is owned
	// whole, whether a scalar, an array, or a free-form object.
	leafNode node = iota
	// structNode: a struct, whose json fields are the keys one level down.
	structNode
	// mapNode: a map, whose keys are data and whose values are each
	// described by the element type.
	mapNode
)

// descend classifies a field's type, returning the type that describes the
// next level down: the struct for a structNode, the element type for a
// mapNode. Every walk over document and schema together asks this one
// question, so they cannot disagree about where the schema ends.
func descend(field reflect.Type) (node, reflect.Type) {
	t := deref(field)
	if t == nil {
		return leafNode, nil
	}
	switch t.Kind() {
	case reflect.Struct:
		return structNode, t
	case reflect.Map:
		return mapNode, deref(t.Elem())
	default:
		return leafNode, nil
	}
}

func deref(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
