//go:build linux || windows

package creds

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// These cover the Linux Secret Service / Windows Credential Manager backend,
// which ships to non-macOS users but (unlike the darwin backend) had no test —
// a coverage hole invisible to a darwin-only CI. go-keyring's in-memory mock
// stands in for the real platform keyring.

func TestKeyringStoreRoundTrip(t *testing.T) {
	keyring.MockInit()
	s := keyringStore{service: "app.test.creds.roundtrip"}

	if _, ok := s.get("acct"); ok {
		t.Fatal("expected a miss before set")
	}
	if err := s.set("acct", "secret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, ok := s.get("acct"); !ok || v != "secret" {
		t.Fatalf("get = (%q, %v), want (secret, true)", v, ok)
	}
	if err := s.delete("acct"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := s.get("acct"); ok {
		t.Error("expected a miss after delete")
	}
}

func TestKeyringStoreDeleteAbsentIsIdempotent(t *testing.T) {
	keyring.MockInit()
	s := keyringStore{service: "app.test.creds.idem"}
	// The backend deliberately swallows keyring.ErrNotFound so a delete of an
	// already-absent entry is a no-op, not a hard failure.
	if err := s.delete("nope"); err != nil {
		t.Errorf("delete of an absent entry should be nil, got %v", err)
	}
}

func TestKeyringStoreDeleteAll(t *testing.T) {
	keyring.MockInit()
	s := keyringStore{service: "app.test.creds.all"}
	if err := s.set("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.set("b", "2"); err != nil {
		t.Fatal(err)
	}
	if err := s.deleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	if _, ok := s.get("a"); ok {
		t.Error("entries should be gone after deleteAll")
	}
}

func TestKeyringStoreWrapsBackendError(t *testing.T) {
	boom := errors.New("boom")
	keyring.MockInitWithError(boom)
	defer keyring.MockInit() // restore a clean mock for other tests
	s := keyringStore{service: "app.test.creds.err"}

	err := s.set("acct", "x")
	if err == nil {
		t.Fatal("set should surface the backend error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error should wrap the backend cause, got %v", err)
	}
	for _, want := range []string{"store secret", "acct", "app.test.creds.err"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}
