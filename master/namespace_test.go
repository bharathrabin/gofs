package master

import (
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestSplitPath(t *testing.T) {
	cases := []struct {
		path    string
		want    []string
		wantErr bool
	}{
		{path: "/a", want: []string{"/a"}},
		{path: "/a/b/c", want: []string{"/a", "/a/b", "/a/b/c"}},
		{path: "", wantErr: true},
		{path: "/", wantErr: true},
		{path: "a/b", wantErr: true},
		{path: "/a//b", wantErr: true},
		{path: "/a/b/", wantErr: true},
		{path: "/a/./b", wantErr: true},
		{path: "/a/../b", wantErr: true},
	}
	for _, c := range cases {
		got, err := splitPath(c.path)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidPath) {
				t.Errorf("splitPath(%q) error = %v, want ErrInvalidPath", c.path, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitPath(%q) unexpected error: %v", c.path, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestParent(t *testing.T) {
	if dir, base := parent("/a/b/c"); dir != "/a/b" || base != "c" {
		t.Errorf("parent(/a/b/c) = (%q, %q), want (/a/b, c)", dir, base)
	}
	if dir, base := parent("/a"); dir != "" || base != "a" {
		t.Errorf("parent(/a) = (%q, %q), want (\"\", a)", dir, base)
	}
}

func TestCreateAndExists(t *testing.T) {
	ns := NewNamespace()

	if err := ns.Create("/a", true); err != nil {
		t.Fatalf("Create(/a, dir) error = %v", err)
	}
	if exists, isDir := ns.Exists("/a"); !exists || !isDir {
		t.Errorf("Exists(/a) = (%v, %v), want (true, true)", exists, isDir)
	}

	if err := ns.Create("/a/f", false); err != nil {
		t.Fatalf("Create(/a/f, file) error = %v", err)
	}
	if exists, isDir := ns.Exists("/a/f"); !exists || isDir {
		t.Errorf("Exists(/a/f) = (%v, %v), want (true, false)", exists, isDir)
	}

	if exists, _ := ns.Exists("/nope"); exists {
		t.Errorf("Exists(/nope) = true, want false")
	}
}

func TestCreateErrExists(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/a", false); err != nil {
		t.Fatalf("Create(/a) error = %v", err)
	}
	err := ns.Create("/a", false)
	if !errors.Is(err, ErrExists) {
		t.Errorf("Create(/a) again: error = %v, want ErrExists", err)
	}
}

func TestCreateErrNotExist(t *testing.T) {
	ns := NewNamespace()
	err := ns.Create("/missing/f", false)
	if !errors.Is(err, ErrDoesNotExist) {
		t.Errorf("Create(/missing/f) error = %v, want ErrNotExist", err)
	}
}

func TestCreateErrNotDir(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/a", false); err != nil {
		t.Fatalf("Create(/a) error = %v", err)
	}
	err := ns.Create("/a/b", false)
	if !errors.Is(err, ErrNotDir) {
		t.Errorf("Create(/a/b) error = %v, want ErrNotDir", err)
	}
}

func TestDelete(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/a", false); err != nil {
		t.Fatalf("Create(/a) error = %v", err)
	}
	if err := ns.Delete("/a"); err != nil {
		t.Fatalf("Delete(/a) error = %v", err)
	}
	if exists, _ := ns.Exists("/a"); exists {
		t.Errorf("Exists(/a) after delete = true, want false")
	}
}

func TestDeleteErrNotExist(t *testing.T) {
	ns := NewNamespace()
	err := ns.Delete("/missing")
	if !errors.Is(err, ErrDoesNotExist) {
		t.Errorf("Delete(/missing) error = %v, want ErrNotExist", err)
	}
}

func TestDeleteErrNotEmpty(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/d", true); err != nil {
		t.Fatalf("Create(/d) error = %v", err)
	}
	if err := ns.Create("/d/f", false); err != nil {
		t.Fatalf("Create(/d/f) error = %v", err)
	}
	if err := ns.Delete("/d"); !errors.Is(err, ErrNotEmpty) {
		t.Errorf("Delete(/d) error = %v, want ErrNotEmpty", err)
	}
	if err := ns.Delete("/d/f"); err != nil {
		t.Fatalf("Delete(/d/f) error = %v", err)
	}
	if err := ns.Delete("/d"); err != nil {
		t.Errorf("Delete(/d) after emptying error = %v, want nil", err)
	}
}

func TestReadDir(t *testing.T) {
	ns := NewNamespace()
	if err := ns.Create("/d", true); err != nil {
		t.Fatalf("Create(/d) error = %v", err)
	}
	for _, name := range []string{"/d/a", "/d/b"} {
		if err := ns.Create(name, false); err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
	}
	if err := ns.Create("/d/sub", true); err != nil {
		t.Fatalf("Create(/d/sub) error = %v", err)
	}
	if err := ns.Create("/d/sub/grandchild", false); err != nil {
		t.Fatalf("Create(/d/sub/grandchild) error = %v", err)
	}

	children, err := ns.ReadDir("/d")
	if err != nil {
		t.Fatalf("ReadDir(/d) error = %v", err)
	}
	sort.Strings(children)
	want := []string{"a", "b", "sub"}
	if !reflect.DeepEqual(children, want) {
		t.Errorf("ReadDir(/d) = %v, want %v", children, want)
	}
}

func TestReadDirErrors(t *testing.T) {
	ns := NewNamespace()
	if _, err := ns.ReadDir("/missing"); !errors.Is(err, ErrDoesNotExist) {
		t.Errorf("ReadDir(/missing) error = %v, want ErrNotExist", err)
	}
	if err := ns.Create("/f", false); err != nil {
		t.Fatalf("Create(/f) error = %v", err)
	}
	if _, err := ns.ReadDir("/f"); !errors.Is(err, ErrNotDir) {
		t.Errorf("ReadDir(/f) error = %v, want ErrNotDir", err)
	}
}
