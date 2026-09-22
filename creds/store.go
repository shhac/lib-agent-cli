package creds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
)

// Store is a JSON file holding credentials/config. Saves use 0600 permissions
// (parent dirs 0700) so secrets are never world-readable — one audited place to
// get that right, instead of a copy per CLI.
type Store struct {
	Path string

	// Overlay writes the value OVER the stored document rather than in place
	// of it, for a config a human reads and edits. Off by default: a
	// credential store is written wholly by the struct, and preserving a key
	// nothing recognises there would be a way to keep a secret alive past the
	// code that knew about it.
	//
	// With it on, a save preserves everything the struct cannot see — the
	// "//" annotations documented in document.go, and keys written by a newer
	// version of the tool — and lays notes out beside the keys they document.
	// A key the SCHEMA owns but the value no longer states is still removed,
	// so unsetting a field works exactly as before; see overlay.
	//
	// Two rules worth knowing before turning it on. Entries of a map-typed
	// field are owned by the struct, so one it no longer lists is deleted
	// (inside an entry, stray keys still survive). Arrays replace wholesale,
	// their elements having no identity to merge on, so this is the one place
	// an unrecognised key does not survive.
	Overlay bool
}

// Load decodes the store's JSON into v. A missing or empty file is not an
// error — v is left untouched and nil is returned, so callers can treat
// "no file yet" as "empty".
func (s Store) Load(v any) error {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

// Save writes v as indented JSON, creating parent directories (0700) and
// writing the file 0600.
//
// The write is atomic: see writeFileAtomic. Save does NOT lock, so two
// processes each doing Load → mutate → Save can still lose an update — the
// second write is built from a snapshot taken before the first landed. Use
// Update for any read-modify-write.
func (s Store) Save(v any) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	return s.write(v)
}

// Update runs a read-modify-write under an exclusive lock: it Loads into v,
// calls mutate, then saves — with concurrent callers serialized rather than
// overwriting each other.
//
// Load and Save are separate calls, so the natural read-mutate-write sequence
// races: two processes read the same snapshot and the second write erases the
// first's change. That is not theoretical — measured against a real credential
// store, twenty concurrent writers left two surviving entries. For a store
// whose entries point at secrets held elsewhere (an OS keychain), a lost entry
// is worse than a lost write: the secret is still in the keychain, but nothing
// references it, so the tool that created it can no longer find or revoke it.
//
// mutate operates on v, which Update has already populated. Returning an error
// from mutate aborts without writing, so a failed validation leaves the stored
// document untouched.
func (s Store) Update(v any, mutate func() error) error {
	return s.WithLock(func() error {
		if err := s.Load(v); err != nil {
			return err
		}
		if err := mutate(); err != nil {
			return err
		}
		return s.write(v)
	})
}

// WithLock runs fn holding the store's exclusive lock, without imposing any
// load/save shape on it.
//
// Update is the convenience for the common case; this is for a store that
// cannot use it — one whose load hydrates secrets from an OS keychain and whose
// save pushes them back, so the critical section has to span more than a JSON
// round-trip. Wrapping an existing Load → mutate → Save in WithLock is the
// smallest change that makes such a store safe, with no restructuring.
//
// The same lock backs Update, so callers can mix the two against one store and
// still serialize correctly.
func (s Store) WithLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	unlock, err := lockStore(s.Path)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// readDoc is the stored document as the map it literally is. Every caller
// decides for itself what a missing or unparseable file means.
func (s Store) readDoc() (map[string]any, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	return decodeDoc(data)
}

// write is the struct-to-file path shared by Save and Update.
func (s Store) write(v any) error {
	payload, err := s.payload(v)
	if err != nil {
		return err
	}
	data, err := encodeDoc(payload)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.Path, data)
}

// encodeDoc is the on-disk form: indented, with a trailing newline so the
// file ends the way an editor would leave it.
func encodeDoc(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeFileAtomic writes data to a temporary file in the same directory and
// renames it over path.
//
// os.WriteFile truncates first and then writes, so a crash, a full disk, or a
// reader arriving mid-write can see a partial document — for a credential store
// that means losing every entry, not just the one being written. A rename is
// atomic on POSIX, so a reader sees either the whole old file or the whole new
// one. The temp file is created in the SAME directory to keep the rename within
// one filesystem, where cross-device renames would fail.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Harmless once the rename has succeeded; on any earlier failure it is what
	// stops a half-written secret being left behind in the config directory.
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	// Flush to disk before the rename, so a crash cannot leave the new name
	// pointing at an empty or partial file.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// payload is what actually gets marshalled: v itself, or — when Overlay is
// set — v laid over the stored document and ordered for reading.
//
// A stored document that will not parse is treated as absent rather than
// failing the write, matching Load's tolerance of a corrupt file: a config
// nobody can parse should not also be a config nobody can fix.
func (s Store) payload(v any) (any, error) {
	if !s.Overlay {
		return v, nil
	}
	fresh, err := structDoc(v)
	if err != nil {
		return nil, err
	}
	stored, _ := s.readDoc()
	return ordered(overlay(stored, fresh, deref(reflect.TypeOf(v)))), nil
}
