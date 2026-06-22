//go:build darwin

package creds

import (
	"os/exec"
	"strings"
)

// newBackend selects the macOS backend: the `security` CLI, unchanged from the
// pre-seam implementation so existing keychain items (raw, un-encoded values)
// stay readable.
func newBackend(service string) backend {
	return &securityBackend{service: service, run: runSecurity}
}

// securityBackend stores secrets in the macOS login keychain via the `security`
// CLI, keyed by service + account. run is overridable in tests.
type securityBackend struct {
	service string
	run     func(args ...string) (string, error)
}

func (b *securityBackend) available() bool { return true }

func (b *securityBackend) get(account string) (string, bool) {
	v, err := b.run("find-generic-password", "-s", b.service, "-a", account, "-w")
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (b *securityBackend) set(account, secret string) error {
	out, err := b.run("add-generic-password", "-s", b.service, "-a", account, "-w", secret, "-U")
	if err != nil {
		return keychainErr("store secret", b.service, account, out, err)
	}
	return nil
}

func (b *securityBackend) delete(account string) error {
	out, err := b.run("delete-generic-password", "-s", b.service, "-a", account)
	if err != nil {
		return keychainErr("delete secret", b.service, account, out, err)
	}
	return nil
}

// deleteAll removes every item under the service. The `security` CLI deletes one
// matching item per call, so this loops until none remain (it reports an error
// once the service is empty, the expected terminator).
func (b *securityBackend) deleteAll() error {
	for {
		if _, err := b.run("delete-generic-password", "-s", b.service); err != nil {
			return nil
		}
	}
}

// runSecurity invokes the macOS `security` CLI. It uses CombinedOutput so the
// tool's diagnostic (written to stderr) is captured and can be surfaced in error
// messages instead of being discarded.
func runSecurity(args ...string) (string, error) {
	out, err := exec.Command("security", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
