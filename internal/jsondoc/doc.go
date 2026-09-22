// Package jsondoc is the document algebra behind creds.Store's Overlay: what
// a JSON config holds that its struct cannot see, how a write keeps it, and
// how it is laid out for a person to read.
//
// It is pure, maps and types in and maps out, so that creds stays the one
// audited place for the filesystem, the lock, and 0600 permissions.
//
// # Comments in JSON
//
// JSON has no comments, so a config a human is meant to read has nowhere to
// explain itself. The family convention is the one npm uses in package.json:
// a comment is an ordinary key whose name begins with "//".
//
//	{
//	  "//model_note": "Empty means the engine's own default.",
//	  "model": "opus"
//	}
//
// A key of exactly "//" would allow only one comment per object, since JSON
// object keys are unique, so a note names what it documents: "//<key>" or
// "//<key>_note" is PINNED to the sibling key <key>. A "//" key naming no
// sibling is commentary on the object itself.
//
// # Why a plain Save destroys them
//
// creds.Store.Save marshals whatever it is handed. A config struct has no
// field for a comment, so a load-modify-save round trip silently drops every
// note -- and every key written by a NEWER version of the tool, which the
// struct cannot see either. Changing one unrelated setting would strip a
// config's whole documentation, and quietly discard a setting the next
// release knows about.
//
// creds.Store.Overlay fixes that: the struct's view is laid over the stored
// document instead of replacing it. See creds.Store.Overlay and overlay for
// the write rules, and orderedKeys for the layout.
//
// # Mechanism, not policy
//
// Nothing here knows any particular schema. Which keys exist comes from the
// struct's own json tags, and what a note says is the CLI's business.
package jsondoc
