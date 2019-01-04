// Package lock provides interprocess locking using a combination of flock
// and process-local mutex-es
package lock

import (
	"fmt"
	"sync"

	"github.com/alexflint/go-filemutex"
)

var (
	locksmu = &sync.Mutex{}
	locks   = make(map[string]*sync.Mutex)
)

// InterProcessLock provides a mutex that works across the current process and
// across all other processes. It works by first acquiring a local lock and
// then a file lock.
//
// The reason that a local process lock is used first, is due to the limits
// of interprocess locking in Linux -> we have to avoid reusing the same lock
// file multiple times in the same process or closing one of the locks will
// unlock all the others. See: http://0pointer.de/blog/projects/locking.html
type InterProcessLock struct {
	Path     string
	filelock *filemutex.FileMutex
}

func (l *InterProcessLock) localMutex() *sync.Mutex {
	locksmu.Lock()
	defer locksmu.Unlock()

	if locks[l.Path] == nil {
		locks[l.Path] = &sync.Mutex{}
	}

	return locks[l.Path]
}

// Lock the lock, blocking until the lock has been acquired
func (l *InterProcessLock) Lock() error {
	local := l.localMutex()
	local.Lock()

	if l.filelock != nil {
		return fmt.Errorf("expected filelock to be nil")
	}

	var err error

	if l.filelock, err = filemutex.New(l.Path); err != nil {
		return fmt.Errorf("could not acquire lock: %v", err)
	}

	if err = l.filelock.Lock(); err != nil {
		return fmt.Errorf("could not acquire file lock: %v", err)
	}

	return nil
}

// Unlock the lock
func (l *InterProcessLock) Unlock() error {
	if err := l.filelock.Unlock(); err != nil {
		return fmt.Errorf("could not unlock file lock: %v", err)
	}

	l.localMutex().Unlock()
	return nil
}

// MustLock engages the lock and panics if that fails (it will still block
// if the lock is already locked, since that is not an error)
func (l *InterProcessLock) MustLock() {
	if err := l.Lock(); err != nil {
		panic(err)
	}
}

// MustUnlock removes the lock and panics if that fails
func (l *InterProcessLock) MustUnlock() {
	if err := l.Unlock(); err != nil {
		panic(err)
	}
}
