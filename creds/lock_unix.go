//go:build unix

package creds

import (
	"os"
	"syscall"
)

// lockStore takes an exclusive advisory lock covering path and returns the
// release function.
//
// The lock is held on a sidecar ".lock" file rather than on the store itself,
// because writeFileAtomic replaces the store by renaming a new file over it: a
// lock held on the store's own descriptor would refer to the OLD inode the
// moment the first writer finished, and would guard nothing. The sidecar's
// inode is stable, so every process ends up contending on the same object.
//
// flock is advisory and released automatically when the descriptor closes —
// including on process death — so a crashed writer cannot wedge the store the
// way a lock file created with O_EXCL would.
func lockStore(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
