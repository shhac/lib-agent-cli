package creds

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shhac/lib-agent-cli/env"
)

// NoKeychainKey is the env-namespace key that opts out of the OS keychain.
// Setting the resolved variable to a truthy value makes Keychain.Available()
// report false, so callers fall back to the 0600 file store and the OS secret
// store (and any GUI prompt) is never reached — which is what makes the
// credential-write path testable in CI and other non-interactive contexts.
//
// It resolves through the keychain's env.Namespace, so for a service
// "app.paulie.agent-slack" the opt-out var is AGENT_SLACK_NO_KEYCHAIN, with
// LIB_AGENT_NO_KEYCHAIN as the family-wide fallback (set it once to flip every
// agent-* CLI headless).
const NoKeychainKey = "NO_KEYCHAIN"

// NoKeychainEnv is the family-wide opt-out variable: the fallback consulted when
// no per-CLI AGENT_<NAME>_NO_KEYCHAIN is set. Retained for reference and tests.
const NoKeychainEnv = env.FamilyPrefix + "_" + NoKeychainKey

// ErrKeychainUnavailable is returned by keychain mutations when no OS secret
// store is available (an unsupported platform, or a host with no usable backend).
var ErrKeychainUnavailable = errors.New("keychain unavailable on this platform")

// backend is the OS-specific secret store behind Keychain. It is selected per-OS
// by newBackend, which is defined once per platform in a build-tagged file
// (keychain_darwin.go uses the macOS `security` CLI; keychain_linux.go and
// keychain_windows.go use the system keyring; keychain_other.go is a no-op). So
// supporting a new OS is a new file, not a conditional sprinkled through methods.
// Methods take the account; the backend closes over the service at construction.
type backend interface {
	// available reports whether this OS backend can be used right now (the store
	// exists and is reachable — e.g. a D-Bus session on Linux). The env opt-out is
	// applied separately, by the Keychain wrapper.
	available() bool
	get(account string) (string, bool)
	set(account, secret string) error
	delete(account string) error
	deleteAll() error
}

// Keychain stores secrets in the host's secret store — the macOS login keychain,
// the Linux Secret Service, or the Windows Credential Manager — keyed by Service
// (e.g. "app.paulie.agent-foo") and an account name. Where no store is available
// it reports Available() == false and mutations return ErrKeychainUnavailable, so
// callers fall back to file storage.
type Keychain struct {
	Service string
	env     env.Namespace // resolves the NO_KEYCHAIN opt-out
	backend backend       // OS-specific store, selected by newBackend
}

// NewKeychain returns a Keychain for the given service, deriving the env
// namespace from the service's last dotted segment — so "app.paulie.agent-slack"
// opts out via AGENT_SLACK_NO_KEYCHAIN (or the family-wide LIB_AGENT_NO_KEYCHAIN).
func NewKeychain(service string) *Keychain {
	return NewKeychainWithEnvPrefix(service, env.PrefixFromName(lastSegment(service)))
}

// NewKeychainWithEnvPrefix is NewKeychain with an explicit env prefix (e.g.
// "AGENT_SLACK") instead of one derived from the service. Use it when the
// service id and the desired env namespace don't line up.
func NewKeychainWithEnvPrefix(service, prefix string) *Keychain {
	return &Keychain{Service: service, env: env.Namespace{Prefix: prefix}, backend: newBackend(service)}
}

// lastSegment returns the substring after the final "." in s (s itself if there
// is none) — the binary name in a "app.paulie.<name>" service id.
func lastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// Available reports whether the keychain can be used: the OS backend is reachable
// and the NO_KEYCHAIN opt-out (per-CLI AGENT_<NAME>_NO_KEYCHAIN, or the
// family-wide LIB_AGENT_NO_KEYCHAIN) is not set. When unavailable, callers fall
// back to the file store — which is what makes the credential-write path testable
// headlessly and keeps non-interactive hosts from blocking on a GUI prompt.
func (k *Keychain) Available() bool {
	return k.backend.available() && !k.env.Flag(NoKeychainKey)
}

// Get returns the secret for account and whether it was found.
func (k *Keychain) Get(account string) (string, bool) {
	if !k.Available() {
		return "", false
	}
	return k.backend.get(account)
}

// Set stores secret for account, replacing any existing entry.
func (k *Keychain) Set(account, secret string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	return k.backend.set(account, secret)
}

// Delete removes the secret for account.
func (k *Keychain) Delete(account string) error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	return k.backend.delete(account)
}

// DeleteAll removes every secret stored under the service, including accounts the
// caller doesn't track (orphans).
func (k *Keychain) DeleteAll() error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	return k.backend.deleteAll()
}

// keychainErr builds a descriptive error from a failed backend call, folding in
// the store's own diagnostic when it printed one.
func keychainErr(op, service, account, out string, err error) error {
	if out != "" {
		return fmt.Errorf("keychain: %s for %q (service %q): %w: %s", op, account, service, err, out)
	}
	return fmt.Errorf("keychain: %s for %q (service %q): %w", op, account, service, err)
}
