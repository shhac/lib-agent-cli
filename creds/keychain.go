package creds

import (
	"errors"
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

func runSecurity(args ...string) (string, error) {
	out, err := exec.Command("security", args...).Output()
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

// Set stores secret for account, replacing any existing entry.
func (k *Keychain) Set(account, secret string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	_, err := k.run("add-generic-password", "-s", k.Service, "-a", account, "-w", secret, "-U")
	return err
}

// Delete removes the secret for account.
func (k *Keychain) Delete(account string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	_, err := k.run("delete-generic-password", "-s", k.Service, "-a", account)
	return err
}
