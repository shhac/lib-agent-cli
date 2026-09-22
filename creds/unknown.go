package creds

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// What a document says that the struct cannot hear.
//
// Store.Overlay stops a save from destroying those keys, but preserving one is
// not the same as surfacing it: a typo, a key from a newer build, and a key
// this release renamed all behave identically to a key that was never written.
// The setting is simply not in effect, and nothing says so.
//
// These are the reads that let a CLI say so, and let `config get`/`unset`
// reach a key its registry has no entry for. The vocabulary stays here; what
// a particular key MEANS — that it moved, that it was a typo, what to do about
// it — is the CLI's to say.

// UnknownKey is one key the document holds and the schema has no field for.
type UnknownKey struct {
	// Path is the dotted path as it would be typed, e.g. "schedule.floor".
	Path string
	// Value is rendered compactly for a log line; empty for an empty object.
	Value string
}

// UnknownKeys reports every key in the stored document that schema has no
// field for, sorted by path so a report is stable between runs. schema is a
// prototype of the config struct — its type is what matters, not its values.
//
// A missing or unparseable file reports nothing: a document nobody can parse
// is a different complaint than a key nobody recognises, and Load already
// treats both as empty.
//
// Annotation keys are skipped. They are comments by construction, so the
// schema deliberately has no field for them and reporting them would make an
// annotated config warn about itself.
func (s Store) UnknownKeys(schema any) []UnknownKey {
	doc, err := s.readDoc()
	if err != nil {
		return nil
	}
	var found []UnknownKey
	walkUnknown(doc, structType(schema), "", &found)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

// RawValue returns the value stored at a dotted path, whether or not the
// schema models it. This is what lets a CLI show what a warned-about key
// actually holds — the registry cannot, because it resolves against the keys
// it knows.
func (s Store) RawValue(path string) (string, bool) {
	doc, err := s.readDoc()
	if err != nil {
		return "", false
	}
	value, ok := lookupPath(doc, strings.Split(path, "."))
	if !ok {
		return "", false
	}
	return render(value), true
}

// RawDelete removes a dotted path from the stored document, reporting whether
// it was there. It rewrites the RAW document under the store's lock, so
// everything the schema cannot see — the annotations, and any other unknown
// key — survives having one of them removed.
//
// For a key the schema DOES model, clear the field and save instead: the
// struct would simply write it back.
func (s Store) RawDelete(path string) (bool, error) {
	var removed bool
	err := s.WithLock(func() error {
		doc, err := s.readDoc()
		if err != nil {
			return err
		}
		if removed = deletePath(doc, strings.Split(path, ".")); !removed {
			return nil
		}
		// doc is already the whole document, so it is written as it stands:
		// there is no struct view to lay over anything.
		data, err := encodeDoc(ordered(doc))
		if err != nil {
			return err
		}
		return writeFileAtomic(s.Path, data)
	})
	return removed, err
}

// walkUnknown descends the document alongside the type that should describe
// it, collecting what the type cannot account for. It stops at the outermost
// key it cannot place: a whole retired section reports once, with its
// contents, rather than once per leaf inside it.
func walkUnknown(doc map[string]any, t reflect.Type, prefix string, found *[]UnknownKey) {
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
			*found = append(*found, UnknownKey{Path: path, Value: render(value)})
			continue
		}
		child, isObject := value.(map[string]any)
		if !isObject {
			continue
		}
		switch next := deref(field); next.Kind() {
		case reflect.Struct:
			walkUnknown(child, next, path, found)
		case reflect.Map:
			// A map's keys are data, not field names, so only its VALUES are
			// described by the schema.
			for name, entry := range child {
				if sub, ok := entry.(map[string]any); ok {
					walkUnknown(sub, deref(next.Elem()), path+"."+name, found)
				}
			}
		}
	}
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

// structType is the struct type behind a prototype value, so callers can pass
// Config{} or &Config{} interchangeably.
func structType(schema any) reflect.Type {
	return deref(reflect.TypeOf(schema))
}

// render prints a value compactly enough for a log line: a string as itself,
// anything else as its JSON, which is the shortest round-trip form. An empty
// object renders empty, which reads as "set, but holding nothing".
func render(v any) string {
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
