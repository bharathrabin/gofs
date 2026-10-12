package master

import "strings"

// splitPath validates that p is a clean, absolute, slash-separated
// pathname and returns the chain of paths from p's shallowest
// ancestor down to p itself, e.g.
//
//	splitPath("/d1/d2/leaf") -> ["/d1", "/d1/d2", "/d1/d2/leaf"]
//
// There is no entry for "/" itself: gofs, like GFS, never
// materializes a root directory object, so top-level paths such as
// "/d1" have an empty ancestor chain. splitPath rejects relative
// paths, ".", or ".." segments, and empty segments (consecutive or
// trailing slashes), as well as the bare root path "/".
func splitPath(p string) ([]string, error) {
	if !strings.HasPrefix(p, "/") || p == "/" {
		return nil, ErrInvalidPath
	}

	segments := strings.Split(strings.TrimPrefix(p, "/"), "/")
	chain := make([]string, 0, len(segments))
	var current strings.Builder
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil, ErrInvalidPath
		}
		current.WriteByte('/')
		current.WriteString(seg)
		chain = append(chain, current.String())
	}
	return chain, nil
}

// parent splits a full path, as returned by splitPath, into its
// immediate parent path and its base name:
//
//	parent("/a/b/c") -> ("/a/b", "c")
//	parent("/a")      -> ("", "a")
//
// A top-level path's parent is "".
func parent(p string) (dir, base string) {
	idx := strings.LastIndex(p, "/")
	return p[:idx], p[idx+1:]
}
