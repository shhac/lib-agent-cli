//go:build darwin

package creds

import (
	"errors"
	"strings"
	"testing"
)

var errBoom = errors.New("exit status 1")

// fakeSecurity builds a securityBackend whose `security` calls are intercepted,
// so the arg construction and error wrapping are tested without a real keychain.
func fakeSecurity(run func(args ...string) (string, error)) *securityBackend {
	return &securityBackend{service: "app.paulie.test", run: run}
}

func TestSecurityBackend_GetSetArgs(t *testing.T) {
	var lastArgs []string
	b := fakeSecurity(func(args ...string) (string, error) {
		lastArgs = args
		if args[0] == "find-generic-password" {
			return "the-secret", nil
		}
		return "", nil
	})

	if v, ok := b.get("acct"); !ok || v != "the-secret" {
		t.Errorf("get = %q, %v", v, ok)
	}
	if lastArgs[0] != "find-generic-password" || lastArgs[2] != "app.paulie.test" || lastArgs[4] != "acct" {
		t.Errorf("get args = %v", lastArgs)
	}
	if err := b.set("acct", "s"); err != nil {
		t.Fatal(err)
	}
	if lastArgs[0] != "add-generic-password" {
		t.Errorf("set args = %v", lastArgs)
	}
}

func TestSecurityBackend_DeleteArgs(t *testing.T) {
	var lastArgs []string
	b := fakeSecurity(func(args ...string) (string, error) { lastArgs = args; return "", nil })
	if err := b.delete("acct"); err != nil {
		t.Fatal(err)
	}
	if lastArgs[0] != "delete-generic-password" || lastArgs[2] != "app.paulie.test" || lastArgs[4] != "acct" {
		t.Errorf("delete args = %v", lastArgs)
	}
}

func TestSecurityBackend_SetErrorIncludesDiagnostic(t *testing.T) {
	b := fakeSecurity(func(...string) (string, error) {
		return "security: SecKeychainItemCreateFromContent: write permission denied", errBoom
	})
	err := b.set("acct", "s")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"write permission denied", "acct", "app.paulie.test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
	if !errors.Is(err, errBoom) {
		t.Error("wrapped error should preserve the underlying cause")
	}
}

func TestSecurityBackend_DeleteAllLoopsUntilEmpty(t *testing.T) {
	calls := 0
	b := fakeSecurity(func(args ...string) (string, error) {
		if args[0] != "delete-generic-password" {
			t.Fatalf("unexpected call %v", args)
		}
		calls++
		if calls <= 3 { // 3 items, then security reports empty
			return "", nil
		}
		return "security: could not be found", errBoom
	})
	if err := b.deleteAll(); err != nil {
		t.Fatalf("deleteAll = %v", err)
	}
	if calls != 4 {
		t.Errorf("deleteAll made %d calls, want 4 (3 deletes + terminating miss)", calls)
	}
}
