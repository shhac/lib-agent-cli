package creds

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/shhac/lib-agent-cli/env"
)

// NoKeychainKey is the env-namespace key that opts out of the keychain. Setting
// the resolved variable to a truthy value makes Keychain.Available() report
// false, so callers fall back to the 0600 file store and the macOS `security`
// CLI (with its GUI prompt) is never invoked — which is what makes the
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

// ErrKeychainUnavailable is returned by keychain mutations on platforms without
// a supported backend (currently anything other than macOS).
var ErrKeychainUnavailable = errors.New("keychain unavailable on this platform")

// Keychain stores secrets in the macOS login keychain via the `security` CLI,
// keyed by Service (e.g. "app.paulie.agent-foo") and an account name. On
// non-macOS platforms it reports Available() == false and mutations return
// ErrKeychainUnavailable, so callers can fall back to file storage.
type Keychain struct {
	Service string
	env     env.Namespace                        // resolves the NO_KEYCHAIN opt-out
	run     func(args ...string) (string, error) // overridable in tests
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
	return &Keychain{Service: service, env: env.Namespace{Prefix: prefix}, run: runSecurity}
}

// lastSegment returns the substring after the final "." in s (s itself if there
// is none) — the binary name in a "app.paulie.<name>" service id.
func lastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// runSecurity invokes the macOS `security` CLI. It uses CombinedOutput so the
// tool's diagnostic (which it writes to stderr) is captured and can be surfaced
// in error messages instead of being discarded.
func runSecurity(args ...string) (string, error) {
	out, err := exec.Command("security", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Available reports whether the keychain backend can be used: macOS only, and
// not opted out via the NO_KEYCHAIN env var (per-CLI AGENT_<NAME>_NO_KEYCHAIN,
// or the family-wide LIB_AGENT_NO_KEYCHAIN). When opted out, callers fall back to
// the file store and the `security` CLI (and its GUI prompt) is never reached —
// which is what makes the credential-write path testable headlessly.
func (k *Keychain) Available() bool {
	return runtime.GOOS == "darwin" && !k.env.Flag(NoKeychainKey)
}

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

// DeleteAll removes every secret stored under the service, including accounts
// the caller doesn't track (orphans). The `security` CLI deletes one matching
// item per call, so this loops until none remain (it reports an error once the
// service is empty, which is the expected terminator). macOS only; returns
// ErrKeychainUnavailable elsewhere.
func (k *Keychain) DeleteAll() error {
	if !k.Available() {
		return ErrKeychainUnavailable
	}
	for {
		if _, err := k.run("delete-generic-password", "-s", k.Service); err != nil {
			return nil
		}
	}
}

// keychainErr builds a descriptive error from a failed `security` call,
// folding in the tool's own diagnostic when it printed one.
func keychainErr(op, service, account, out string, err error) error {
	if out != "" {
		return fmt.Errorf("keychain: %s for %q (service %q): %w: %s", op, account, service, err, out)
	}
	return fmt.Errorf("keychain: %s for %q (service %q): %w", op, account, service, err)
}
