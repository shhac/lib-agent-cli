package creds

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ErrKeychainUnavailable is returned by keychain mutations on platforms without
// a supported backend (currently anything other than macOS).
var ErrKeychainUnavailable = errors.New("keychain unavailable on this platform")

// Keychain stores secrets in the macOS login keychain via the `security` CLI,
// keyed by Service (e.g. "app.paulie.agent-foo") and an account name. On
// non-macOS platforms it reports Available() == false and mutations return
// ErrKeychainUnavailable, so callers can fall back to file storage.
type Keychain struct {
	Service string
	run     func(args ...string) (string, error) // overridable in tests
}

// NewKeychain returns a Keychain for the given service.
func NewKeychain(service string) *Keychain {
	return &Keychain{Service: service, run: runSecurity}
}

// runSecurity invokes the macOS `security` CLI. It uses CombinedOutput so the
// tool's diagnostic (which it writes to stderr) is captured and can be surfaced
// in error messages instead of being discarded.
func runSecurity(args ...string) (string, error) {
	out, err := exec.Command("security", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Available reports whether the keychain backend can be used (macOS only).
func (k *Keychain) Available() bool { return runtime.GOOS == "darwin" }

// Get returns the secret for account and whether it was found.
func (k *Keychain) Get(account string) (string, bool) {
	if !k.Available() {
		return "", false
	}
	v, err := k.run("find-generic-password", "-s", k.Service, "-a", account, "-w")
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

// Set stores secret for account, replacing any existing entry. On failure the
// error includes the `security` diagnostic and the service/account context.
func (k *Keychain) Set(account, secret string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	out, err := k.run("add-generic-password", "-s", k.Service, "-a", account, "-w", secret, "-U")
	if err != nil {
		return keychainErr("store secret", k.Service, account, out, err)
	}
	return nil
}

// Delete removes the secret for account. On failure the error includes the
// `security` diagnostic and the service/account context.
func (k *Keychain) Delete(account string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	out, err := k.run("delete-generic-password", "-s", k.Service, "-a", account)
	if err != nil {
		return keychainErr("delete secret", k.Service, account, out, err)
	}
	return nil
}

// keychainErr builds a descriptive error from a failed `security` call,
// folding in the tool's own diagnostic when it printed one.
func keychainErr(op, service, account, out string, err error) error {
	if out != "" {
		return fmt.Errorf("keychain: %s for %q (service %q): %w: %s", op, account, service, err, out)
	}
	return fmt.Errorf("keychain: %s for %q (service %q): %w", op, account, service, err)
}
