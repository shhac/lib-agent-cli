package jsondoc

import (
	"encoding/json"
	"reflect"
)

// Overlay is v laid over the stored document and ordered for writing: what
// creds.Store writes when Overlay is set. stored may be nil, which is simply
// a first write.
func Overlay(stored map[string]any, v any) (any, error) {
	fresh, err := structDoc(v)
	if err != nil {
		return nil, err
	}
	return Ordered(overlay(stored, fresh, deref(reflect.TypeOf(v)))), nil
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

// Decode parses a document into the map it literally is.
func Decode(data []byte) (map[string]any, error) {
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
	return Decode(data)
}
