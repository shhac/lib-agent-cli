package creds

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	output "github.com/shhac/lib-agent-output"
)

// Annotated config files, and how a write keeps them intact.
//
// # Comments in JSON
//
// JSON has no comments, so a config a human is meant to read has nowhere to
// explain itself. The family convention is the one npm uses in package.json:
// a comment is an ordinary key whose name begins with "//".
//
//	{
//	  "//model_note": "Empty means the engine's own default.",
//	  "model": "opus"
//	}
//
// A key of exactly "//" would allow only one comment per object, since JSON
// object keys are unique, so a note names what it documents: "//<key>" or
// "//<key>_note" is PINNED to the sibling key <key>. A "//" key naming no
// sibling is commentary on the object itself.
//
// # Why a plain Save destroys them
//
// Store.Save marshals whatever it is handed. A config struct has no field for
// a comment, so a load-modify-save round trip silently drops every note --
// and every key written by a NEWER version of the tool, which the struct
// cannot see either. Changing one unrelated setting would strip a config's
// whole documentation, and quietly discard a setting the next release knows
// about.
//
// Store.Overlay fixes that: the struct's view is laid over the stored
// document instead of replacing it. See Store.Overlay for the write rules,
// and orderedKeys for the layout.
//
// # Mechanism, not policy
//
// Nothing here knows any particular schema. Which keys exist comes from the
// struct's own json tags, and what a note says is the CLI's business.

// notePrefix marks an annotation key. Two slashes rather than one, because a
// path-like key is a plausible thing to store and a comment is not.
const notePrefix = "//"

// noteSuffix is the convention's optional trailing marker, so both
// "//model_note" and "//model" pin to "model".
const noteSuffix = "_note"

// isNote reports whether key is an annotation rather than a setting.
func isNote(key string) bool {
	return strings.HasPrefix(key, notePrefix)
}

// pinnedTo reports the sibling key an annotation documents. A note naming no
// sibling is commentary on the object rather than a pin.
func pinnedTo(key string, siblings map[string]any) (string, bool) {
	if !isNote(key) {
		return "", false
	}
	name := strings.TrimPrefix(key, notePrefix)
	if target := strings.TrimSuffix(name, noteSuffix); target != name {
		if _, ok := siblings[target]; ok {
			return target, true
		}
	}
	if _, ok := siblings[name]; ok {
		return name, true
	}
	return "", false
}

// orderedKeys is the layout rule: within an object, sort by SORT NAME. A
// plain key sorts as itself; a pinned note sorts as the key it documents,
// ties breaking note-first. Free-standing commentary leads the object.
//
// Sorting notes by their own name would not do. "/" is below every letter and
// digit in byte order -- which is the order encoding/json emits a map in --
// so every note would sink to the top of its object, away from the key it
// explains.
//
// Because a pinned pair shares one sort name, nothing can sort between a note
// and its key: any other key lands at its own name, before or after the pair
// as a unit. Order is then a pure function of the key set, so writing the
// same content twice produces the same bytes, and a note added by hand
// anywhere in an object moves beside its key on the next write.
func orderedKeys(obj map[string]any) []string {
	type entry struct {
		key      string
		sortName string
		pinned   bool
		loose    bool
	}
	entries := make([]entry, 0, len(obj))
	for key := range obj {
		e := entry{key: key, sortName: key}
		if target, ok := pinnedTo(key, obj); ok {
			e.sortName, e.pinned = target, true
		} else if isNote(key) {
			e.loose = true
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.loose != b.loose {
			return a.loose
		}
		if a.sortName != b.sortName {
			return a.sortName < b.sortName
		}
		if a.pinned != b.pinned {
			return a.pinned
		}
		return a.key < b.key
	})
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.key)
	}
	return keys
}

// ordered renders a decoded document as output.Ordered, recursively, so the
// layout survives encoding. The family already has one ordered-JSON emitter
// and this is it; a second would be a second set of escaping decisions.
func ordered(v any) any {
	switch value := v.(type) {
	case map[string]any:
		fields := make(output.Ordered, 0, len(value))
		for _, key := range orderedKeys(value) {
			fields = append(fields, output.Field{Key: key, Value: ordered(value[key])})
		}
		return fields
	case []any:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, ordered(item))
		}
		return items
	default:
		return v
	}
}

// overlay merges the struct's view of one object over what is stored there,
// with t the struct type describing this level. stored may be nil, which is
// simply a first write.
//
// The rule that makes this precise rather than a guess: a key the SCHEMA owns
// but the struct no longer states was unset, and must go; a key the schema
// has no field for was never ours to remove.
func overlay(stored, fresh map[string]any, t reflect.Type) map[string]any {
	fields := jsonFields(t)
	out := make(map[string]any, len(fresh)+len(stored))

	// Everything the struct cannot speak for, kept as found. A schema key is
	// deliberately not copied here: whether it survives is decided below, by
	// whether the struct still states it.
	for key, value := range stored {
		if _, isSchema := fields[key]; isSchema && !isNote(key) {
			continue
		}
		out[key] = value
	}

	for key, value := range fresh {
		field, known := fields[key]
		child, freshIsObject := value.(map[string]any)
		prior, storedIsObject := stored[key].(map[string]any)
		if !known || !freshIsObject || !storedIsObject {
			// A scalar, an array, or a fresh object with nothing beneath it
			// to preserve. Arrays replace wholesale: their elements have no
			// identity to merge on, so the struct's list is the answer.
			out[key] = value
			continue
		}
		switch kind, next := descend(field); kind {
		case structNode:
			out[key] = overlay(prior, child, next)
		case mapNode:
			out[key] = overlayEntries(prior, child, next)
		default:
			out[key] = value
		}
	}
	return out
}

// overlayEntries merges a map-typed field, where the KEYS are data rather
// than schema. The struct round-trips the whole map, so an entry it no longer
// lists was deleted -- unlike an unknown key in a struct, which the schema
// never claimed. Inside an entry the element's own schema applies again, so a
// stray key there lives on.
func overlayEntries(stored, fresh map[string]any, elem reflect.Type) map[string]any {
	kind, next := descend(elem)
	out := make(map[string]any, len(fresh))
	for key, value := range fresh {
		child, freshIsObject := value.(map[string]any)
		prior, storedIsObject := stored[key].(map[string]any)
		if freshIsObject && storedIsObject && kind == structNode {
			out[key] = overlay(prior, child, next)
			continue
		}
		out[key] = value
	}
	// Annotations sit beside the entries they describe. The struct has no
	// field for one, so it is not an entry the struct dropped.
	for key, value := range stored {
		if isNote(key) {
			if _, taken := out[key]; !taken {
				out[key] = value
			}
		}
	}
	return out
}

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

// decodeDoc parses a document into the map it literally is.
func decodeDoc(data []byte) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// structDoc is v as the struct sees it: the subset that has json fields.
func structDoc(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return decodeDoc(data)
}
