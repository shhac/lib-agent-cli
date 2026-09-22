// Package creds is the secret/credential plumbing shared by agent-first CLIs: a
// 0600 JSON store, a macOS keychain wrapper, and small value-resolution helpers
// (FirstNonEmpty / FirstNonZero / Getenv).
//
// Filesystem locations are a separate concern — the freedesktop base
// directories live in the sibling xdg package, e.g.
// creds.Store{Path: filepath.Join(xdg.ConfigDir("agent-foo"), "credentials.json")}.
//
// # Config files people read
//
// A credentials file is written wholly by its struct. A CONFIG file often is
// not: it carries comments explaining each setting, and it outlives the
// release that wrote it. Store.Overlay is for that second case — it preserves
// everything the struct cannot see and lays comments out beside the keys they
// document. It is off by default, so credential stores are unaffected.
//
// The "//" comment convention and the merge itself live in internal/jsondoc,
// which is pure: this package keeps the file, the lock and the permissions,
// and hands the document algebra maps rather than paths.
package creds
