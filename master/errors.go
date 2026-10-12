package master

import "errors"

// Sentinel errors returned (wrapped in a *PathError, where
// applicable) by Namespace and ChunkAllocator operations. Callers
// should use errors.Is to test for these rather than comparing
// *PathError values directly.
var (
	// ErrDoesNotExist indicates the named path, or one of its ancestor
	// directories, does not exist.
	ErrDoesNotExist = errors.New("no such file or directory")

	// ErrExists indicates the named path already exists.
	ErrExists = errors.New("file or directory already exists")

	// ErrNotDir indicates an operation that requires a directory was
	// given a path that names a file, or that names a file somewhere
	// among the path's ancestor components.
	ErrNotDir = errors.New("not a directory")

	// ErrIsDir indicates an operation that requires a file was given
	// a path that names a directory.
	ErrIsDir = errors.New("is a directory")

	// ErrNotEmpty indicates Delete was called on a non-empty
	// directory.
	ErrNotEmpty = errors.New("directory not empty")

	// ErrInvalidPath indicates a path argument was not a clean,
	// absolute, slash-separated pathname.
	ErrInvalidPath = errors.New("invalid path")

	// ErrNoSuchChunk indicates a chunk handle unknown to the
	// ChunkAllocator it was presented to.
	ErrNoSuchChunk = errors.New("no such chunk")
)

// PathError records an error and the operation and path that caused
// it, in the style of os.PathError.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error { return e.Err }
