//go:build !unix

package creds

// lockStore is a no-op where flock is unavailable.
//
// Update still gets its atomic write on these platforms, so a concurrent writer
// cannot produce a TORN file — but it can still lose an update, because nothing
// serializes the read-modify-write. That is a deliberate, documented gap rather
// than a silent one: the family ships darwin and linux binaries, and adding a
// Windows LockFileEx path would pull golang.org/x/sys from an indirect
// dependency to a direct one for a platform none of the release workflows
// currently build.
//
// If a Windows build is ever released, this is the file to fill in.
func lockStore(string) (func(), error) {
	return func() {}, nil
}
