package creds

import (
	"os"
	"path/filepath"
	"testing"
)

// The file store is the active backend whenever the keychain is opted out (the
// headless/CI credential path), so its failure modes are load-bearing.

func TestStore_LoadMalformedJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := (Store{Path: p}).Load(&v); err == nil {
		t.Error("Load should surface malformed JSON rather than returning empty")
	}
}

func TestStore_LoadReadErrorIsNotSwallowed(t *testing.T) {
	// A directory path yields a read error that is not IsNotExist.
	var v map[string]any
	if err := (Store{Path: t.TempDir()}).Load(&v); err == nil {
		t.Error("Load should error when the path is a directory")
	}
}

func TestStore_SaveMkdirAllFails(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// blocker is a file, so creating it as a parent directory must fail.
	p := filepath.Join(blocker, "sub", "c.json")
	if err := (Store{Path: p}).Save(map[string]any{"a": 1}); err == nil {
		t.Error("Save should error when a parent path component is a file")
	}
}

func TestStore_SaveMarshalFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	if err := (Store{Path: p}).Save(make(chan int)); err == nil {
		t.Error("Save should error on an unmarshalable value")
	}
}
