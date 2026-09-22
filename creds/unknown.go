package creds

import "github.com/shhac/lib-agent-cli/internal/jsondoc"

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
// A missing, unreadable or unparseable file reports nothing: a document
// nobody can read is a different complaint from a key nobody recognises, and
// Load is where that complaint surfaces.
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
	for _, k := range jsondoc.UnknownKeys(doc, schema) {
		found = append(found, UnknownKey{Path: k.Path, Value: k.Value})
	}
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
	value, ok := jsondoc.Lookup(doc, path)
	if !ok {
		return "", false
	}
	return jsondoc.Render(value), true
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
		if removed = jsondoc.Delete(doc, path); !removed {
			return nil
		}
		// doc is already the whole document, so it is written as it stands:
		// there is no struct view to lay over anything.
		data, err := encodeDoc(jsondoc.Ordered(doc))
		if err != nil {
			return err
		}
		return writeFileAtomic(s.Path, data)
	})
	return removed, err
}
