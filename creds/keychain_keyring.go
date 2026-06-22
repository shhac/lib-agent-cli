//go:build linux || windows

package creds

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keyringStore is the get/set/delete/deleteAll shared by the Linux Secret Service
// and Windows Credential Manager backends — both go through go-keyring, keyed by
// the reverse-DNS service id. Each platform wraps it with its own availability
// check (see keychain_linux.go / keychain_windows.go).
type keyringStore struct{ service string }

func (s keyringStore) get(account string) (string, bool) {
	v, err := keyring.Get(s.service, account)
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (s keyringStore) set(account, secret string) error {
	if err := keyring.Set(s.service, account, secret); err != nil {
		return keychainErr("store secret", s.service, account, "", err)
	}
	return nil
}

func (s keyringStore) delete(account string) error {
	if err := keyring.Delete(s.service, account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return keychainErr("delete secret", s.service, account, "", err)
	}
	return nil
}

func (s keyringStore) deleteAll() error {
	if err := keyring.DeleteAll(s.service); err != nil {
		return keychainErr("delete all secrets", s.service, "", "", err)
	}
	return nil
}
