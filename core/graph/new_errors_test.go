package graph

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// infoDir is a directory file info (the Stat of the fake root).
type infoDir struct{}

func (infoDir) Name() string       { return "." }
func (infoDir) Size() int64        { return 0 }
func (infoDir) Mode() fs.FileMode  { return fs.ModeDir }
func (infoDir) ModTime() time.Time { return time.Time{} }
func (infoDir) IsDir() bool        { return true }
func (infoDir) Sys() any           { return nil }

// infoFile is a regular file info (the Info of the fake file entries).
type infoFile struct{ name string }

func (i infoFile) Name() string     { return i.name }
func (infoFile) Size() int64        { return 0 }
func (infoFile) Mode() fs.FileMode  { return 0 }
func (infoFile) ModTime() time.Time { return time.Time{} }
func (infoFile) IsDir() bool        { return false }
func (infoFile) Sys() any           { return nil }

// entryFile is a fixed directory entry (one file).
type entryFile struct{ name string }

func (e entryFile) Name() string               { return e.name }
func (entryFile) IsDir() bool                  { return false }
func (entryFile) Type() fs.FileMode            { return 0 }
func (entryFile) Size() int64                  { return 0 }
func (entryFile) ModTime() time.Time           { return time.Time{} }
func (entryFile) Mode() fs.FileMode            { return 0 }
func (entryFile) Sys() any                     { return nil }
func (e entryFile) Info() (fs.FileInfo, error) { return infoFile{name: e.name}, nil }

// rootFile is the file of the fake root (the Stat of the "." Open).
type rootFile struct{}

func (rootFile) Stat() (fs.FileInfo, error)         { return infoDir{}, nil }
func (rootFile) Close() error                       { return nil }
func (rootFile) Read([]byte) (int, error)           { return 0, nil }
func (rootFile) Write([]byte) (int, error)          { return 0, nil }
func (rootFile) Seek(int64, int) (int64, error)     { return 0, nil }
func (rootFile) Readdirnames(int) ([]string, error) { return nil, nil }

// fakeFS is the failing FS: ReadDir lists the fixed entries or fails (the
// walk error seam), the Open of any non-root name errors (the read error
// seam).
type fakeFS struct {
	entries []fs.DirEntry
	dirErr  error
	fileErr error
}

func (f fakeFS) Open(name string) (fs.File, error) {
	if name == "." {
		return rootFile{}, nil
	}
	return nil, f.fileErr
}
func (f fakeFS) ReadDir(name string) ([]os.DirEntry, error) { return f.entries, f.dirErr }
func (fakeFS) Stat(name string) (fs.FileInfo, error)        { return infoDir{}, nil }
func (fakeFS) Close() error                                 { return nil }
func (fakeFS) Read([]byte) (int, error)                     { return 0, nil }
func (fakeFS) Write([]byte) (int, error)                    { return 0, nil }
func (fakeFS) Seek(int64, int) (int64, error)               { return 0, nil }
func (fakeFS) Readdirnames(int) ([]string, error)           { return nil, nil }

// New reports the walk failure when the FS cannot list the tree.
func TestNewWalkError(t *testing.T) {
	fsys := fakeFS{dirErr: errors.New("listing failed")}
	if _, err := New(fsys); err == nil || err.Error() != "listing failed" {
		t.Errorf("New(walk error): got %v, want the listing failure", err)
	}
}

// New reports the read failure when the FS cannot read a .go file.
func TestNewReadError(t *testing.T) {
	fsys := fakeFS{
		entries: []fs.DirEntry{entryFile{"bad.go"}},
		fileErr: errors.New("read failed"),
	}
	_, err := New(fsys)
	if err == nil {
		t.Fatal("New(read error): got nil, want the read failure")
	}
}

// New reports the parse failure when a .go file is not valid Go source.
func TestNewParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package p\nnot go\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := New(os.DirFS(dir)); err == nil {
		t.Fatal("New(parse error): got nil, want the parse failure")
	}
}
