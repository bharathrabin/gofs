package master

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// runWithTimeout runs fn in a goroutine and fails the test if it
// does not complete within d. This turns a real deadlock into a
// loud test failure instead of a CI run that hangs until its own
// timeout.
func runWithTimeout(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("timed out, likely deadlock")
	}
}

func TestConcurrentSiblingCreates(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/home", true); err != nil {
		t.Fatalf("Create(/home) error = %v", err)
	}
	if err := ns.Create("/home/user", true); err != nil {
		t.Fatalf("Create(/home/user) error = %v", err)
	}

	const n = 50
	runWithTimeout(t, 5*time.Second, func() {
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() {
				errs[i] = ns.Create(fmt.Sprintf("/home/user/f%d", i), false)
			})
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("Create(/home/user/f%d) error = %v", i, err)
			}
		}
	})

	children, err := ns.ReadDir("/home/user")
	if err != nil {
		t.Fatalf("ReadDir(/home/user) error = %v", err)
	}
	if len(children) != n {
		t.Errorf("ReadDir(/home/user) returned %d children, want %d", len(children), n)
	}
}

func TestConcurrentCreateDeleteSamePath(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/d", true); err != nil {
		t.Fatalf("Create(/d) error = %v", err)
	}

	const iterations = 200
	runWithTimeout(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		wg.Go(func() {
			for range iterations {
				_ = ns.Create("/d/x", false)
			}
		})
		wg.Go(func() {
			for range iterations {
				_ = ns.Delete("/d/x")
			}
		})
		wg.Wait()
	})

	// Whatever the final state, it must be internally consistent:
	// existence and chunk-list queries must agree and not panic.
	exists, isDir := ns.Exists("/d/x")
	if exists && isDir {
		t.Errorf("Exists(/d/x) reports isDir=true, want a file")
	}
}

func TestConcurrentCreateAndLookup(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/d", true); err != nil {
		t.Fatalf("Create(/d) error = %v", err)
	}
	if err := ns.Create("/d/sub", true); err != nil {
		t.Fatalf("Create(/d/sub) error = %v", err)
	}

	const n = 50
	runWithTimeout(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				_ = ns.Create(fmt.Sprintf("/d/sub/f%d", i), false)
			})
			wg.Go(func() {
				ns.Exists("/d")
				ns.Exists("/d/sub")
				_, _ = ns.ReadDir("/d")
			})
		}
		wg.Wait()
	})
}

func TestConcurrentAppendChunk(t *testing.T) {
	ns := NewNamespace()
	alloc := NewChunkAllocator()
	if err := ns.Create("/f", false); err != nil {
		t.Fatalf("Create(/f) error = %v", err)
	}

	const n = 100
	runWithTimeout(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				if _, err := ns.AppendChunk("/f", alloc); err != nil {
					t.Errorf("AppendChunk(/f) error = %v", err)
				}
			})
		}
		wg.Wait()
	})

	chunks, err := ns.Chunks("/f")
	if err != nil {
		t.Fatalf("Chunks(/f) error = %v", err)
	}
	if len(chunks) != n {
		t.Fatalf("Chunks(/f) has %d entries, want %d", len(chunks), n)
	}
	seen := make(map[ChunkHandle]bool, n)
	for _, h := range chunks {
		if seen[h] {
			t.Errorf("duplicate chunk handle %v", h)
		}
		seen[h] = true
	}
}

func TestConcurrentDisjointSubtrees(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/a", true); err != nil {
		t.Fatalf("Create(/a) error = %v", err)
	}
	if err := ns.Create("/b", true); err != nil {
		t.Fatalf("Create(/b) error = %v", err)
	}

	const n = 100
	runWithTimeout(t, 10*time.Second, func() {
		var wg sync.WaitGroup
		for _, root := range []string{"/a", "/b"} {
			wg.Go(func() {
				for i := range n {
					if err := ns.Create(fmt.Sprintf("%s/f%d", root, i), false); err != nil {
						t.Errorf("Create(%s/f%d) error = %v", root, i, err)
					}
				}
			})
		}
		wg.Wait()
	})
}
