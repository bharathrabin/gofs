package master

import "sync"

// Namespace is the master's in-memory mapping from full path names to
// file and directory metadata. Following the GFS design (§4.1),
// there is no tree of directory inode objects: a directory is just
// an entry in this table like any file, distinguished only by its
// isDir flag, and "directory listing" works by scanning for entries
// whose immediate parent is the directory in question. GFS never
// materializes a directory as something to be locked or traversed as
// an object in its own right; the only structural property a
// directory needs is that its full pathname is a string-prefix of
// the path names below it, which is exactly what the locking scheme
// in acquireAncestors relies on.
type Namespace struct {
	mu      sync.Mutex               // guards entries and locks below
	entries map[string]*entry        // full path -> metadata
	locks   map[string]*sync.RWMutex // full path -> per-path lock, created lazily
}

// entry is the metadata stored per full pathname. Files and
// directories share this one representation (see the Namespace doc
// comment); chunks is unused when isDir is true.
type entry struct {
	isDir  bool
	chunks []ChunkHandle
}

// NewNamespace returns an empty Namespace.
func NewNamespace() *Namespace {
	return &Namespace{
		entries: make(map[string]*entry),
		locks:   make(map[string]*sync.RWMutex),
	}
}

// lockFor returns the RWMutex for path, creating it if it does not
// already exist. The returned mutex is shared across all callers for
// the same path string. Lock entries are never removed, even after a
// Delete: safely reclaiming a mutex that another goroutine might
// still hold a reference to would require a refcounting or epoch
// scheme, which this increment deliberately defers. Leaking one small
// mutex per path ever created is an accepted tradeoff for an
// in-memory v1.
func (ns *Namespace) lockFor(path string) *sync.RWMutex {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	l, ok := ns.locks[path]
	if !ok {
		l = &sync.RWMutex{}
		ns.locks[path] = l
	}
	return l
}

// acquireAncestors read-locks every path in ancestors, shallow to
// deep, verifying as it goes that each one exists and is a
// directory. It fails fast — without attempting to lock anything
// deeper — the moment an ancestor is missing or is not a directory,
// which keeps the check race-free against a concurrent delete of an
// ancestor. On success it returns a function that releases the
// acquired locks in reverse (deepest-first) order; the caller must
// call it exactly once, typically via defer. On failure it returns a
// no-op release function alongside the error.
func (ns *Namespace) acquireAncestors(op string, ancestors []string) (release func(), err error) {
	locks := make([]*sync.RWMutex, 0, len(ancestors))
	unlock := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].RUnlock()
		}
	}

	for _, p := range ancestors {
		l := ns.lockFor(p)
		l.RLock()
		locks = append(locks, l)

		ns.mu.Lock()
		e, ok := ns.entries[p]
		ns.mu.Unlock()
		if !ok {
			unlock()
			return func() {}, &PathError{Op: op, Path: p, Err: ErrDoesNotExist}
		}
		if !e.isDir {
			unlock()
			return func() {}, &PathError{Op: op, Path: p, Err: ErrNotDir}
		}
	}
	return unlock, nil
}

// Create adds a new file or directory at path. It fails with
// ErrExists if path already exists, or ErrDoesNotExist/ErrNotDir if some
// ancestor directory of path does not exist or is not a directory.
func (ns *Namespace) Create(path string, isDir bool) error {
	chain, err := splitPath(path)
	if err != nil {
		return &PathError{Op: "create", Path: path, Err: err}
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("create", chain[:len(chain)-1])
	if err != nil {
		return err
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.Lock()
	defer tlock.Unlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	if _, exists := ns.entries[target]; exists {
		return &PathError{Op: "create", Path: path, Err: ErrExists}
	}
	ns.entries[target] = &entry{isDir: isDir}
	return nil
}

// Delete removes the file or directory at path. It fails with
// ErrNotExist if path does not exist, or ErrNotEmpty if path is a
// non-empty directory.
//
// GFS's full design does not physically remove the entry here: it
// renames it to a hidden, timestamped name for later lazy garbage
// collection. That lifecycle belongs to a future increment; this
// Delete removes the namespace entry immediately, which is the right
// primitive for the metadata layer in isolation.
func (ns *Namespace) Delete(path string) error {
	chain, err := splitPath(path)
	if err != nil {
		return &PathError{Op: "delete", Path: path, Err: err}
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("delete", chain[:len(chain)-1])
	if err != nil {
		return err
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.Lock()
	defer tlock.Unlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	e, exists := ns.entries[target]
	if !exists {
		return &PathError{Op: "delete", Path: path, Err: ErrDoesNotExist}
	}
	if e.isDir && ns.hasChildrenLocked(target) {
		return &PathError{Op: "delete", Path: path, Err: ErrNotEmpty}
	}
	delete(ns.entries, target)
	return nil
}

// Exists reports whether path is present in the namespace, and
// whether it is a directory. An invalid path, or one whose ancestors
// don't exist, simply reports exists == false rather than returning
// an error, mirroring the two-value idiom of map lookups.
func (ns *Namespace) Exists(path string) (exists, isDir bool) {
	chain, err := splitPath(path)
	if err != nil {
		return false, false
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("exists", chain[:len(chain)-1])
	if err != nil {
		return false, false
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.RLock()
	defer tlock.RUnlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	e, ok := ns.entries[target]
	if !ok {
		return false, false
	}
	return true, e.isDir
}

// ReadDir returns the base names of path's immediate children. path
// must refer to an existing directory.
func (ns *Namespace) ReadDir(path string) ([]string, error) {
	chain, err := splitPath(path)
	if err != nil {
		return nil, &PathError{Op: "readdir", Path: path, Err: err}
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("readdir", chain[:len(chain)-1])
	if err != nil {
		return nil, err
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.RLock()
	defer tlock.RUnlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	e, ok := ns.entries[target]
	if !ok {
		return nil, &PathError{Op: "readdir", Path: path, Err: ErrDoesNotExist}
	}
	if !e.isDir {
		return nil, &PathError{Op: "readdir", Path: path, Err: ErrNotDir}
	}

	var children []string
	for p := range ns.entries {
		if p == target {
			continue
		}
		if dir, base := parent(p); dir == target {
			children = append(children, base)
		}
	}
	return children, nil
}

// hasChildrenLocked reports whether any entry's immediate parent is
// dir. Callers must hold ns.mu. This is a linear scan of the whole
// namespace table; acceptable for this increment's in-memory scope,
// and a candidate to revisit (e.g. with a maintained child count)
// if a later increment's workload makes it a hot path.
func (ns *Namespace) hasChildrenLocked(dir string) bool {
	for p := range ns.entries {
		if p == dir {
			continue
		}
		if d, _ := parent(p); d == dir {
			return true
		}
	}
	return false
}

// AppendChunk allocates a new chunk handle via alloc and appends it
// to path's chunk list, representing the file's growth by one more
// chunk. path must refer to an existing file.
//
// Allocation happens before path's write lock is taken, so a
// contended namespace never serializes behind chunk-handle
// allocation (and vice versa): the two are independent resources.
func (ns *Namespace) AppendChunk(path string, alloc *ChunkAllocator) (ChunkHandle, error) {
	h := alloc.AllocateChunk()

	chain, err := splitPath(path)
	if err != nil {
		return 0, &PathError{Op: "appendchunk", Path: path, Err: err}
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("appendchunk", chain[:len(chain)-1])
	if err != nil {
		return 0, err
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.Lock()
	defer tlock.Unlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	e, ok := ns.entries[target]
	if !ok {
		return 0, &PathError{Op: "appendchunk", Path: path, Err: ErrDoesNotExist}
	}
	if e.isDir {
		return 0, &PathError{Op: "appendchunk", Path: path, Err: ErrIsDir}
	}
	e.chunks = append(e.chunks, h)
	return h, nil
}

// Chunks returns the ordered list of chunk handles for the file at
// path, from first to last. The returned slice is a copy; callers
// must not assume it reflects later mutations.
func (ns *Namespace) Chunks(path string) ([]ChunkHandle, error) {
	chain, err := splitPath(path)
	if err != nil {
		return nil, &PathError{Op: "chunks", Path: path, Err: err}
	}
	target := chain[len(chain)-1]

	release, err := ns.acquireAncestors("chunks", chain[:len(chain)-1])
	if err != nil {
		return nil, err
	}
	defer release()

	tlock := ns.lockFor(target)
	tlock.RLock()
	defer tlock.RUnlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	e, ok := ns.entries[target]
	if !ok {
		return nil, &PathError{Op: "chunks", Path: path, Err: ErrDoesNotExist}
	}
	if e.isDir {
		return nil, &PathError{Op: "chunks", Path: path, Err: ErrIsDir}
	}
	cp := make([]ChunkHandle, len(e.chunks))
	copy(cp, e.chunks)
	return cp, nil
}
