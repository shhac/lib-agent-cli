package creds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type entries struct {
	Items map[string]string `json:"items"`
}

// The failure Update exists to prevent. With Load → mutate → Save, concurrent
// writers each build their write from a snapshot taken before the others
// landed, so all but the last are erased. Measured against a real credential
// store, twenty writers left two entries.
func TestUpdateKeepsConcurrentWrites(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "credentials.json")}

	const writers = 20
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var doc entries
			err := s.Update(&doc, func() error {
				if doc.Items == nil {
					doc.Items = map[string]string{}
				}
				doc.Items[fmt.Sprintf("profile-%02d", i)] = fmt.Sprintf("secret-%02d", i)
				return nil
			})
			if err != nil {
				t.Errorf("Update: %v", err)
			}
		}(i)
	}
	wg.Wait()

	var final entries
	if err := s.Load(&final); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(final.Items) != writers {
		t.Fatalf("%d of %d concurrent writes survived — updates were lost", len(final.Items), writers)
	}
	for i := range writers {
		key := fmt.Sprintf("profile-%02d", i)
		if final.Items[key] != fmt.Sprintf("secret-%02d", i) {
			t.Errorf("%s missing or corrupted: %q", key, final.Items[key])
		}
	}
}

// A rejected mutation must leave the stored document exactly as it was — the
// point of validating inside the lock rather than after the write.
func TestUpdateLeavesTheStoreUntouchedWhenMutateFails(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "credentials.json")}
	if err := s.Save(entries{Items: map[string]string{"keep": "me"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var doc entries
	wantErr := fmt.Errorf("rejected")
	if err := s.Update(&doc, func() error {
		doc.Items["keep"] = "clobbered"
		return wantErr
	}); err != wantErr {
		t.Fatalf("Update should propagate the mutate error, got %v", err)
	}

	var after entries
	if err := s.Load(&after); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if after.Items["keep"] != "me" {
		t.Errorf("a failed mutate wrote anyway: %q", after.Items["keep"])
	}
}

func TestUpdateStartsFromTheStoredDocument(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "credentials.json")}
	if err := s.Save(entries{Items: map[string]string{"existing": "value"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var doc entries
	if err := s.Update(&doc, func() error {
		if doc.Items["existing"] != "value" {
			return fmt.Errorf("mutate saw %v, want the stored document", doc.Items)
		}
		doc.Items["added"] = "second"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var after entries
	_ = s.Load(&after)
	if after.Items["existing"] != "value" || after.Items["added"] != "second" {
		t.Errorf("Update did not merge onto the stored document: %v", after.Items)
	}
}

// The store is replaced by a rename, so nothing may be left behind — a stray
// temp file in a config directory is a secret at rest under a name nothing
// tracks.
func TestWritesLeaveNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	s := Store{Path: filepath.Join(dir, "credentials.json")}

	if err := s.Save(entries{Items: map[string]string{"a": "1"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var doc entries
	if err := s.Update(&doc, func() error { return nil }); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range found {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// The atomic write must not weaken the permissions the plain Save guaranteed.
func TestAtomicWriteKeeps0600(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "nested", "credentials.json")}

	var doc entries
	if err := s.Update(&doc, func() error {
		doc.Items = map[string]string{"a": "secret"}
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	info, err := os.Stat(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perms = %o, want 600", perm)
	}
}
