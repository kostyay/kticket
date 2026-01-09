package filelock

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := Acquire(lockPath)
	require.NoError(t, err)
	require.NotNil(t, lock)

	// Lock file should exist
	_, err = os.Stat(lockPath)
	require.NoError(t, err)

	// Release
	err = lock.Release()
	require.NoError(t, err)
}

func TestAcquireShared(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "shared.lock")

	// Multiple shared locks should work
	lock1, err := AcquireShared(lockPath)
	require.NoError(t, err)

	lock2, err := AcquireShared(lockPath)
	require.NoError(t, err)

	require.NoError(t, lock1.Release())
	require.NoError(t, lock2.Release())
}

func TestTryAcquire(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "try.lock")

	// First try should succeed
	lock1, err := TryAcquire(lockPath)
	require.NoError(t, err)
	require.NotNil(t, lock1)

	// Second try should return nil (lock held)
	lock2, err := TryAcquire(lockPath)
	require.NoError(t, err)
	assert.Nil(t, lock2)

	// After release, should succeed
	require.NoError(t, lock1.Release())

	lock3, err := TryAcquire(lockPath)
	require.NoError(t, err)
	require.NotNil(t, lock3)
	require.NoError(t, lock3.Release())
}

func TestTryAcquireShared(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "tryshared.lock")

	// Multiple shared locks should work
	lock1, err := TryAcquireShared(lockPath)
	require.NoError(t, err)
	require.NotNil(t, lock1)

	lock2, err := TryAcquireShared(lockPath)
	require.NoError(t, err)
	require.NotNil(t, lock2)

	require.NoError(t, lock1.Release())
	require.NoError(t, lock2.Release())
}

func TestExclusiveBlocksShared(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "exclusive.lock")

	// Get exclusive lock
	exclusive, err := TryAcquire(lockPath)
	require.NoError(t, err)
	require.NotNil(t, exclusive)

	// Shared lock should fail
	shared, err := TryAcquireShared(lockPath)
	require.NoError(t, err)
	assert.Nil(t, shared)

	require.NoError(t, exclusive.Release())
}

func TestAcquireContextTimeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "timeout.lock")

	// Hold exclusive lock
	lock1, err := Acquire(lockPath)
	require.NoError(t, err)

	// Try to acquire with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = AcquireContext(ctx, lockPath)
	require.Error(t, err)
	// Error can be "timeout" or "context deadline exceeded"
	assert.True(t, err != nil)

	require.NoError(t, lock1.Release())
}

func TestAcquireCreatesDir(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "nested", "deep", "test.lock")

	lock, err := Acquire(lockPath)
	require.NoError(t, err)

	// Verify nested dirs created
	_, err = os.Stat(filepath.Dir(lockPath))
	require.NoError(t, err)

	require.NoError(t, lock.Release())
}

func TestLockPath(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "path.lock")

	lock, err := Acquire(lockPath)
	require.NoError(t, err)

	assert.Equal(t, lockPath, lock.Path())
	require.NoError(t, lock.Release())
}

func TestNilLockSafe(t *testing.T) {
	var lock *Lock

	// Should not panic
	err := lock.Release()
	assert.NoError(t, err)
	assert.Empty(t, lock.Path())
}

func TestConcurrentExclusiveLocks(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "concurrent.lock")

	const goroutines = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	order := make([]int, 0, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			lock, err := Acquire(lockPath)
			require.NoError(t, err)

			// Record that we got the lock
			mu.Lock()
			order = append(order, id)
			mu.Unlock()

			// Small delay to ensure serialization is tested
			time.Sleep(10 * time.Millisecond)

			require.NoError(t, lock.Release())
		}(i)
	}

	wg.Wait()

	// All goroutines should have completed
	assert.Len(t, order, goroutines)
}
