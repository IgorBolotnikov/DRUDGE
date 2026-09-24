package persistence

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// lockFileExtension is the suffix of every lock file this package writes.
const lockFileExtension = ".lock"

// What an update does when someone else holds the lock.
const (
	waitForLock  = true
	giveUpOnLock = false
)

// lockFile takes an exclusive lock on the lock file and returns the release
// callback. If the process dies, then lock is released by the kernel.
//
// With waitForLock it waits for a lock someone else holds and always comes
// back with it. With giveUpOnLock it comes back at once, and hasLock says
// whether the lock was free. A caller that did not get the lock gets a nil
// release callback.
func lockFile(path string, shouldWait bool) (unlock func(), hasLock bool, err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, common.DefaultFilePerm)
	if err != nil {
		return nil, false, fmt.Errorf("could not open the lock file %s: %w", path, err)
	}

	lockMode := syscall.LOCK_EX
	if !shouldWait {
		lockMode |= syscall.LOCK_NB
	}

	if err := syscall.Flock(int(file.Fd()), lockMode); err != nil {
		file.Close()
		if !shouldWait && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("could not lock %s: %w", path, err)
	}

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, true, nil
}
