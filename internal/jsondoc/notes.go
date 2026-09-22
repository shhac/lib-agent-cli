package jsondoc

import (
	"sort"
	"strings"

	output "github.com/shhac/lib-agent-output"
)

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

// Ordered renders a decoded document as output.Ordered, recursively, so the
// layout survives encoding. The family already has one ordered-JSON emitter
// and this is it; a second would be a second set of escaping decisions.
func Ordered(v any) any {
	switch value := v.(type) {
	case map[string]any:
		fields := make(output.Ordered, 0, len(value))
		for _, key := range orderedKeys(value) {
			fields = append(fields, output.Field{Key: key, Value: Ordered(value[key])})
		}
		return fields
	case []any:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, Ordered(item))
		}
		return items
	default:
		return v
	}
}
