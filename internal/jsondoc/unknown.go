package jsondoc

import (
	"reflect"
	"sort"
)

// Key is one key the document holds and the schema has no field for.
type Key struct {
	// Path is the dotted path as it would be typed, e.g. "schedule.floor".
	Path string
	// Value is rendered compactly for a log line; empty for an empty object.
	Value string
}

// UnknownKeys reports every key in doc that schema has no field for, sorted
// by path so a report is stable between runs. schema is a prototype of the
// config struct, so Config{} and &Config{} are interchangeable; its type is
// what matters, not its values.
//
// Annotation keys are skipped. They are comments by construction, so the
// schema deliberately has no field for them and reporting them would make an
// annotated config warn about itself.
func UnknownKeys(doc map[string]any, schema any) []Key {
	var found []Key
	walkUnknown(doc, structType(schema), "", &found)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

// walkUnknown descends the document alongside the type that should describe
// it, collecting what the type cannot account for. It stops at the outermost
// key it cannot place: a whole retired section reports once, with its
// contents, rather than once per leaf inside it.
func walkUnknown(doc map[string]any, t reflect.Type, prefix string, found *[]Key) {
	fields := jsonFields(t)
	for key, value := range doc {
		if isNote(key) {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		field, known := fields[key]
		if !known {
			*found = append(*found, Key{Path: path, Value: Render(value)})
			continue
		}
		child, isObject := value.(map[string]any)
		if !isObject {
			continue
		}
		switch kind, next := descend(field); kind {
		case structNode:
			walkUnknown(child, next, path, found)
		case mapNode:
			// A map's keys are data, not field names, so only its VALUES are
			// described by the schema, and only when the element is a struct.
			// Any other element is free-form data the struct owns whole, as
			// overlay treats it: its keys are nobody's to call unknown.
			elemKind, elem := descend(next)
			if elemKind != structNode {
				continue
			}
			for name, entry := range child {
				if sub, ok := entry.(map[string]any); ok {
					walkUnknown(sub, elem, path+"."+name, found)
				}
			}
		}
	}
}

// structType is the struct type behind a prototype value, so callers can pass
// Config{} or &Config{} interchangeably.
func structType(schema any) reflect.Type {
	return deref(reflect.TypeOf(schema))
}
