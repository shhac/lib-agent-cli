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
func (s Store) Save(v any) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, append(data, '\n'), 0o600)
}
