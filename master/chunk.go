package master

import (
	"sync"
	"sync/atomic"
)

// ChunkHandle is a globally unique identifier for a chunk, assigned
// by the master at allocation time. GFS chunk handles are 64-bit and
// never reused, so a simple monotonically increasing counter is
// sufficient and matches the paper; there is no need for anything
// like a UUID.
type ChunkHandle uint64

// chunkInfo is the master's metadata for a single chunk. Replica
// locations and lease state are owned by later increments
// (chunk server and lease management) and are not tracked here.
type chunkInfo struct {
	version uint64
}

// ChunkAllocator hands out globally unique, monotonically increasing
// chunk handles and tracks their version numbers. It is deliberately
// independent of any Namespace instance and its path locks: handle
// allocation is a pure counter operation with no relationship to
// which file a chunk will belong to, so it needs no path lock, only
// its own internal synchronization.
type ChunkAllocator struct {
	next atomic.Uint64

	mu   sync.Mutex
	info map[ChunkHandle]*chunkInfo
}

// NewChunkAllocator returns an empty ChunkAllocator.
func NewChunkAllocator() *ChunkAllocator {
	return &ChunkAllocator{info: make(map[ChunkHandle]*chunkInfo)}
}

// AllocateChunk reserves a new, never-before-used chunk handle and
// initializes its version to 0, meaning "never leased". It does not
// associate the handle with any file; callers combine this with
// Namespace.AppendChunk to do so.
func (a *ChunkAllocator) AllocateChunk() ChunkHandle {
	h := ChunkHandle(a.next.Add(1))

	a.mu.Lock()
	defer a.mu.Unlock()
	a.info[h] = &chunkInfo{}
	return h
}

// Version returns the current version number for h.
func (a *ChunkAllocator) Version(h ChunkHandle) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	info, ok := a.info[h]
	if !ok {
		return 0, &PathError{Op: "version", Path: "", Err: ErrNoSuchChunk}
	}
	return info.version, nil
}

// BumpVersion atomically increments and returns h's version number.
// A future lease-management increment calls this whenever a new
// lease is granted, per the paper's scheme for detecting stale
// replicas after a chunk server misses a mutation.
func (a *ChunkAllocator) BumpVersion(h ChunkHandle) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	info, ok := a.info[h]
	if !ok {
		return 0, &PathError{Op: "bumpversion", Path: "", Err: ErrNoSuchChunk}
	}
	info.version++
	return info.version, nil
}
