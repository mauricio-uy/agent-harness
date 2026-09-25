package docs

import (
	"path"
)

// tree answers whether paths exist. The staged check copies only Markdown out
// of the Git index, so every other staged path exists only in its listing.
type tree interface {
	exists(path string) bool
	isDir(path string) bool
	// grep lists the lines of the files other than Markdown under root that match pattern.
	grep(root, pattern string) ([]grepLine, error)
}

// diskTree is the file system.
type diskTree struct{}

func (diskTree) exists(p string) bool { return exists(p) }
func (diskTree) isDir(p string) bool  { return isDir(p) }

// stagedTree knows the staged paths, relative to root with forward slashes,
// and falls back to the Markdown copied under root.
type stagedTree struct {
	repo  string
	root  string
	files map[string]bool
	dirs  map[string]bool
}

// add records a staged file and every folder above it.
func (t stagedTree) add(name string) {
	t.files[name] = true
	for dir := path.Dir(name); dir != "." && !t.dirs[dir]; dir = path.Dir(dir) {
		t.dirs[dir] = true
	}
}

func (t stagedTree) exists(p string) bool {
	rel := relSlash(t.root, p)
	return t.files[rel] || t.dirs[rel] || exists(p)
}

func (t stagedTree) isDir(p string) bool { return t.dirs[relSlash(t.root, p)] || isDir(p) }

func (diskTree) grep(root, pattern string) ([]grepLine, error) {
	return grepFiles(root, pattern, false)
}

// grep searches the staged content of the repository the snapshot came from.
func (t stagedTree) grep(_, pattern string) ([]grepLine, error) {
	return grepFiles(t.repo, pattern, true)
}
