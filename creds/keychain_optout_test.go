package creds

import (
	"runtime"
	"testing"
)

func TestKeychainAvailable_OptOutEnv(t *testing.T) {
	k := NewKeychain("app.test.optout")

	t.Run("opt-out forces unavailable (no security CLI, no GUI)", func(t *testing.T) {
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
