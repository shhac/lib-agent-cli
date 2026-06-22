//go:build !darwin && !linux && !windows

package creds

// newBackend selects the fallback backend on platforms without a supported
// secret store (e.g. the BSDs without a configured Secret Service): a no-op that
// reports unavailable, so callers use the 0600 file store.
func newBackend(string) backend { return unavailableBackend{} }

type unavailableBackend struct{}

func (unavailableBackend) available() bool           { return false }
func (unavailableBackend) get(string) (string, bool) { return "", false }
func (unavailableBackend) set(_, _ string) error     { return ErrKeychainUnavailable }
func (unavailableBackend) delete(string) error       { return ErrKeychainUnavailable }
func (unavailableBackend) deleteAll() error          { return ErrKeychainUnavailable }
