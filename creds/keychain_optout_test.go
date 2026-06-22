package creds

import "testing"

// fakeBackend is an in-memory backend so the Keychain WRAPPER logic (the
// availability/opt-out decision and method delegation) is testable on every
// platform — the OS backends themselves are exercised in their build-tagged
// tests.
type fakeBackend struct {
	avail bool
	store map[string]string
}

func newFake(avail bool) *fakeBackend { return &fakeBackend{avail: avail, store: map[string]string{}} }

func (b *fakeBackend) available() bool { return b.avail }
func (b *fakeBackend) get(a string) (string, bool) {
	v, ok := b.store[a]
	return v, ok
}
func (b *fakeBackend) set(a, s string) error { b.store[a] = s; return nil }
func (b *fakeBackend) delete(a string) error { delete(b.store, a); return nil }
func (b *fakeBackend) deleteAll() error      { b.store = map[string]string{}; return nil }

// withFake builds a Keychain wired to an in-memory backend with the given OS
// availability, preserving NewKeychain's env-prefix derivation.
func withFake(service string, avail bool) (*Keychain, *fakeBackend) {
	k := NewKeychain(service)
	fb := newFake(avail)
	k.backend = fb
	return k, fb
}

func TestKeychainAvailable_OptOutEnv(t *testing.T) {
	k, _ := withFake("app.test.optout", true)

	t.Run("family-wide opt-out forces unavailable", func(t *testing.T) {
		t.Setenv(NoKeychainEnv, "1")
		if k.Available() {
			t.Fatalf("Available() must be false when %s is set", NoKeychainEnv)
		}
	})

	t.Run("falsey values do not opt out (backend stays available)", func(t *testing.T) {
		for _, v := range []string{"", "0", "false", "FALSE"} {
			t.Setenv(NoKeychainEnv, v)
			if !k.Available() {
				t.Errorf("%s=%q: Available()=false, want true", NoKeychainEnv, v)
			}
		}
	})

	t.Run("unavailable OS backend is unavailable regardless of env", func(t *testing.T) {
		down, _ := withFake("app.test.optout", false)
		if down.Available() {
			t.Error("Available() must be false when the OS backend reports unavailable")
		}
	})
}

// TestKeychainAvailable_PerCLIPrecedence — the per-CLI var derived from the
// service wins over the family-wide one, and can re-enable a family-wide opt-out.
func TestKeychainAvailable_PerCLIPrecedence(t *testing.T) {
	k, _ := withFake("app.paulie.agent-foo", true)
	const perCLI = "AGENT_FOO_NO_KEYCHAIN"

	t.Run("per-CLI var alone opts out", func(t *testing.T) {
		t.Setenv(perCLI, "1")
		if k.Available() {
			t.Fatalf("Available() must be false when %s is set", perCLI)
		}
	})

	t.Run("falsey per-CLI var overrides truthy family var", func(t *testing.T) {
		t.Setenv(NoKeychainEnv, "1")
		t.Setenv(perCLI, "0")
		if !k.Available() {
			t.Errorf("per-CLI false should override family true; Available()=false")
		}
	})
}

func TestNewKeychainWithEnvPrefix_Explicit(t *testing.T) {
	k := NewKeychainWithEnvPrefix("whatever.service.id", "CUSTOM_TOOL")
	k.backend = newFake(true)
	t.Setenv("CUSTOM_TOOL_NO_KEYCHAIN", "1")
	if k.Available() {
		t.Fatal("Available() must be false when the explicit-prefix opt-out is set")
	}
}

// TestKeychain_Delegation — when available, the wrapper round-trips through the
// backend; when opted out, mutations return ErrKeychainUnavailable and reads miss.
func TestKeychain_Delegation(t *testing.T) {
	k, fb := withFake("app.paulie.agent-foo", true)

	if err := k.Set("acct", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if fb.store["acct"] != "secret" {
		t.Errorf("Set did not reach backend: %v", fb.store)
	}
	if v, ok := k.Get("acct"); !ok || v != "secret" {
		t.Errorf("Get = %q,%v", v, ok)
	}
	if err := k.Delete("acct"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := k.Get("acct"); ok {
		t.Error("Get should miss after Delete")
	}

	t.Setenv(NoKeychainEnv, "1") // opt out
	if _, ok := k.Get("x"); ok {
		t.Error("Get must miss when opted out")
	}
	if err := k.Set("x", "y"); err != ErrKeychainUnavailable {
		t.Errorf("Set when opted out = %v, want ErrKeychainUnavailable", err)
	}
	if err := k.Delete("x"); err != ErrKeychainUnavailable {
		t.Errorf("Delete when opted out = %v, want ErrKeychainUnavailable", err)
	}
	if err := k.DeleteAll(); err != ErrKeychainUnavailable {
		t.Errorf("DeleteAll when opted out = %v, want ErrKeychainUnavailable", err)
	}
}
