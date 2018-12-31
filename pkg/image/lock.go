package image

import (
	"fmt"
	"sync"

	"github.com/alexflint/go-filemutex"
)

var iplocksmu = &sync.Mutex{}
var iplocks = make(map[string]*sync.Mutex)

// InterProcessLock provides a mutex that works across the current process and
// across all other processes. It works by first acquiring a local lock and
// then a file lock.
//
// The reason that a local process lock is used first, is due to toe limits
// of interprocess locking in Linux -> we have to avoid reusing the same lock
// file multiple times in the same process or closing one of the locks will
// unlock all the others. See: http://0pointer.de/blog/projects/locking.html
type InterProcessLock struct {
	Path  string
	flock *filemutex.FileMutex
}

// Lock the lock, blocking until the lock has been acquired
func (l *InterProcessLock) Lock() {
	iplocksmu.Lock()
	if iplocks[l.Path] == nil {
		iplocks[l.Path] = &sync.Mutex{}
	}
	iplocksmu.Unlock()

	iplocks[l.Path].Lock()

	if l.flock != nil {
		panic(fmt.Errorf("expected flock to be nil"))
	}

	var err error
	l.flock, err = filemutex.New(l.Path)

	if err != nil {
		panic(fmt.Errorf("could not acquire lock: %v", err))
	}

	err = l.flock.Lock()

	if err != nil {
		panic(fmt.Errorf("could not acquire file lock: %v", err))
	}
}

// Unlock the lock
func (l *InterProcessLock) Unlock() {
	if err := l.flock.Unlock(); err != nil {
		panic(fmt.Errorf("could not unlock file lock: %v", err))
	}
	iplocks[l.Path].Unlock()
}
