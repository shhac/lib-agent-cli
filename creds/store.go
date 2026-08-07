package creds

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Store is a JSON file holding credentials/config. Saves use 0600 permissions
// (parent dirs 0700) so secrets are never world-readable — one audited place to
// get that right, instead of a copy per CLI.
type Store struct {
	Path string
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
// The write is atomic: see writeAtomic. Save does NOT lock, so two processes
// each doing Load → mutate → Save can still lose an update — the second write
// is built from a snapshot taken before the first landed. Use Update for any
// read-modify-write.
func (s Store) Save(v any) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	return s.writeAtomic(v)
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
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}

	unlock, err := lockStore(s.Path)
	if err != nil {
		return err
	}
	defer unlock()

	if err := s.Load(v); err != nil {
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	return s.writeAtomic(v)
}

// writeAtomic writes v to a temporary file in the same directory and renames it
// over the target.
//
// os.WriteFile truncates first and then writes, so a crash, a full disk, or a
// reader arriving mid-write can see a partial document — for a credential store
// that means losing every entry, not just the one being written. A rename is
// atomic on POSIX, so a reader sees either the whole old file or the whole new
// one. The temp file is created in the SAME directory to keep the rename within
// one filesystem, where cross-device renames would fail.
func (s Store) writeAtomic(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.Path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(s.Path)+".tmp-*")
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
	return os.Rename(tmpName, s.Path)
}
