package master

import (
	"errors"
	"testing"
)

func TestAllocateChunkDistinctIncreasing(t *testing.T) {
	a := NewChunkAllocator()
	h1 := a.AllocateChunk()
	h2 := a.AllocateChunk()
	if h1 == h2 {
		t.Fatalf("AllocateChunk returned duplicate handle %v", h1)
	}
	if h2 <= h1 {
		t.Errorf("AllocateChunk handles not increasing: h1=%v h2=%v", h1, h2)
	}
}

func TestChunkVersion(t *testing.T) {
	a := NewChunkAllocator()
	h := a.AllocateChunk()

	v, err := a.Version(h)
	if err != nil {
		t.Fatalf("Version(%v) error = %v", h, err)
	}
	if v != 0 {
		t.Errorf("Version(%v) = %d, want 0", h, v)
	}

	v, err = a.BumpVersion(h)
	if err != nil {
		t.Fatalf("BumpVersion(%v) error = %v", h, err)
	}
	if v != 1 {
		t.Errorf("BumpVersion(%v) = %d, want 1", h, v)
	}

	v, err = a.BumpVersion(h)
	if err != nil {
		t.Fatalf("BumpVersion(%v) error = %v", h, err)
	}
	if v != 2 {
		t.Errorf("BumpVersion(%v) = %d, want 2", h, v)
	}
}

func TestChunkVersionErrNoSuchChunk(t *testing.T) {
	a := NewChunkAllocator()
	if _, err := a.Version(ChunkHandle(999)); !errors.Is(err, ErrNoSuchChunk) {
		t.Errorf("Version(999) error = %v, want ErrNoSuchChunk", err)
	}
	if _, err := a.BumpVersion(ChunkHandle(999)); !errors.Is(err, ErrNoSuchChunk) {
		t.Errorf("BumpVersion(999) error = %v, want ErrNoSuchChunk", err)
	}
}

func TestAppendChunkAndChunks(t *testing.T) {
	ns := NewNamespace()
	alloc := NewChunkAllocator()
	if err := ns.Create("/f", false); err != nil {
		t.Fatalf("Create(/f) error = %v", err)
	}

	h1, err := ns.AppendChunk("/f", alloc)
	if err != nil {
		t.Fatalf("AppendChunk(/f) error = %v", err)
	}
	h2, err := ns.AppendChunk("/f", alloc)
	if err != nil {
		t.Fatalf("AppendChunk(/f) error = %v", err)
	}

	chunks, err := ns.Chunks("/f")
	if err != nil {
		t.Fatalf("Chunks(/f) error = %v", err)
	}
	want := []ChunkHandle{h1, h2}
	if len(chunks) != len(want) || chunks[0] != want[0] || chunks[1] != want[1] {
		t.Errorf("Chunks(/f) = %v, want %v", chunks, want)
	}

	// Mutating the returned slice must not affect internal state.
	chunks[0] = ChunkHandle(0)
	chunks2, err := ns.Chunks("/f")
	if err != nil {
		t.Fatalf("Chunks(/f) error = %v", err)
	}
	if chunks2[0] != h1 {
		t.Errorf("Chunks(/f) returned a non-defensive copy: got %v, want first handle %v", chunks2[0], h1)
	}
}

func TestAppendChunkErrIsDir(t *testing.T) {
	ns := NewNamespace()
	alloc := NewChunkAllocator()
	if err := ns.Create("/d", true); err != nil {
		t.Fatalf("Create(/d) error = %v", err)
	}
	if _, err := ns.AppendChunk("/d", alloc); !errors.Is(err, ErrIsDir) {
		t.Errorf("AppendChunk(/d) error = %v, want ErrIsDir", err)
	}
}

func TestAppendChunkErrNotExist(t *testing.T) {
	ns := NewNamespace()
	alloc := NewChunkAllocator()
	if _, err := ns.AppendChunk("/missing", alloc); !errors.Is(err, ErrDoesNotExist) {
		t.Errorf("AppendChunk(/missing) error = %v, want ErrNotExist", err)
	}
}
