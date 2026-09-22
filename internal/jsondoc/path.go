package jsondoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Lookup returns the value at a dotted path, e.g. "engine.limits.percent".
func Lookup(doc map[string]any, path string) (any, bool) {
	return lookupPath(doc, strings.Split(path, "."))
}

// Delete removes a dotted path, reporting whether it was there.
func Delete(doc map[string]any, path string) bool {
	return deletePath(doc, strings.Split(path, "."))
}

// lookupPath walks a dotted path through nested objects.
func lookupPath(doc map[string]any, path []string) (any, bool) {
	value, ok := doc[path[0]]
	if !ok {
		return nil, false
	}
	if len(path) == 1 {
		return value, true
	}
	child, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return lookupPath(child, path[1:])
}

// deletePath removes a dotted path, leaving its parents in place: an object
// that is empty afterwards was still written deliberately, and pruning it
// would remove more than was asked for.
func deletePath(doc map[string]any, path []string) bool {
	if len(path) == 1 {
		if _, ok := doc[path[0]]; !ok {
			return false
		}
		delete(doc, path[0])
		return true
	}
	child, ok := doc[path[0]].(map[string]any)
	if !ok {
		return false
	}
	return deletePath(child, path[1:])
}

// Render prints a value compactly enough for a log line: a string as itself,
// anything else as its JSON, which is the shortest round-trip form. An empty
// object renders empty, which reads as "set, but holding nothing".
func Render(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case map[string]any:
		if len(value) == 0 {
			return ""
		}
	}
	out, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(out)
}
