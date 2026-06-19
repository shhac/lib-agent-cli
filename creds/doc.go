// Package creds is the secret/credential plumbing shared by agent-first CLIs: a
// 0600 JSON store, a macOS keychain wrapper, and small value-resolution helpers
// (FirstNonEmpty / FirstNonZero / Getenv).
//
// Filesystem locations are a separate concern — the freedesktop base
// directories live in the sibling xdg package, e.g.
// creds.Store{Path: filepath.Join(xdg.ConfigDir("agent-foo"), "credentials.json")}.
package creds
