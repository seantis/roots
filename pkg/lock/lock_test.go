package lock

import (
    "io/ioutil"
    "path"
    "testing"

    "github.com/stretchr/testify/assert"
)

// TestLockSimple tests the interprocess locking using a single process
func TestLockSimple(t *testing.T) {
    dir, _ := ioutil.TempDir("", "locks")

    foo := &InterProcessLock{Path: path.Join(dir, "foo")}
    bar := &InterProcessLock{Path: path.Join(dir, "bar")}

    assert.NoError(t, foo.Lock(), "error locking foo")
    assert.NoError(t, bar.Lock(), "error locking bar")

    assert.NoError(t, foo.Unlock(), "error unlocking foo")
    assert.NoError(t, bar.Unlock(), "error unlocking bar")
}
