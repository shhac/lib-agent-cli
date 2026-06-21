package creds

import (
	"runtime"
	"testing"
)

func TestKeychainAvailable_OptOutEnv(t *testing.T) {
	k := NewKeychain("app.test.optout")

	t.Run("family-wide opt-out forces unavailable (no security CLI, no GUI)", func(t *testing.T) {
		t.Setenv(NoKeychainEnv, "1")
		if k.Available() {
			t.Fatalf("Available() must be false when %s is set", NoKeychainEnv)
		}
	})

	t.Run("falsey values do not opt out", func(t *testing.T) {
		for _, v := range []string{"", "0", "false", "FALSE"} {
			t.Setenv(NoKeychainEnv, v)
			if k.Available() != (runtime.GOOS == "darwin") {
				t.Errorf("%s=%q: Available()=%v, want %v", NoKeychainEnv, v, k.Available(), runtime.GOOS == "darwin")
			}
		}
	})
}

// TestKeychainAvailable_PerCLIPrecedence — the per-CLI var derived from the
// service (here AGENT_FOO_NO_KEYCHAIN for "app.paulie.agent-foo") is consulted
// before the family-wide one, and wins on presence: it can both opt out on its
// own and *re-enable* (override) a truthy family-wide opt-out.
func TestKeychainAvailable_PerCLIPrecedence(t *testing.T) {
	k := NewKeychain("app.paulie.agent-foo")
	const perCLI = "AGENT_FOO_NO_KEYCHAIN"

	t.Run("per-CLI var alone opts out", func(t *testing.T) {
		t.Setenv(perCLI, "1")
		if k.Available() {
			t.Fatalf("Available() must be false when %s is set", perCLI)
		}
	})

	t.Run("falsey per-CLI var overrides truthy family var", func(t *testing.T) {
		t.Setenv(NoKeychainEnv, "1") // family-wide says opt out…
		t.Setenv(perCLI, "0")        // …but this CLI explicitly re-enables
		if k.Available() != (runtime.GOOS == "darwin") {
			t.Errorf("per-CLI false should override family true; Available()=%v", k.Available())
		}
	})
}

// TestNewKeychainWithEnvPrefix_Explicit — an explicit prefix is honoured instead
// of one derived from the service id.
func TestNewKeychainWithEnvPrefix_Explicit(t *testing.T) {
	k := NewKeychainWithEnvPrefix("whatever.service.id", "CUSTOM_TOOL")
	t.Setenv("CUSTOM_TOOL_NO_KEYCHAIN", "1")
	if k.Available() {
		t.Fatal("Available() must be false when the explicit-prefix opt-out is set")
	}
}
