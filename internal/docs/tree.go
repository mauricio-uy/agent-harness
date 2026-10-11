package docs

import (
	"io/fs"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// tree answers whether paths exist. The staged check copies only Markdown out
// of the Git index, so every other staged path exists only in its listing.
type tree interface {
	exists(path string) bool
	isDir(path string) bool
	// grep lists the lines of the files other than Markdown under root that match pattern.
	grep(root, pattern string) ([]grepLine, error)
	// markdown lists the Markdown files under root whose links are checked.
	markdown(root string) []string
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

func (diskTree) markdown(root string) []string { return projectMarkdown(root) }

// The snapshot holds exactly the staged Markdown.
func (stagedTree) markdown(root string) []string { return markdownFiles(root) }

// projectMarkdown lists the Markdown files under root that Git would not
// ignore: tracked and untracked ones inside a repository, the ones outside
// .gitignore elsewhere. Without Git it walks every folder but .git.
func projectMarkdown(root string) []string {
	if _, err := exec.LookPath("git"); err != nil {
		return walkMarkdown(root)
	}
	args := []string{"-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.md"}
	if exec.Command("git", "-C", root, "rev-parse", "--git-dir").Run() != nil {
		// Outside a repository git grep is the command that honors .gitignore.
		// The empty pattern matches every line, and an empty file has no links.
		args = []string{"-C", root, "grep", "-z", "-l", "--no-index", "--exclude-standard", "-e", "", "--", "*.md"}
	}
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return walkMarkdown(root)
	}
	var files []string
	seen := map[string]bool{}
	for _, name := range strings.Split(string(output), "\x00") {
		path := filepath.Join(root, filepath.FromSlash(name))
		// A tracked file deleted from the working tree has no links to check.
		if name != "" && !seen[name] && isFile(path) {
			seen[name] = true
			files = append(files, path)
		}
	}
	return files
}

func walkMarkdown(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case entry.IsDir() && entry.Name() == ".git":
			return filepath.SkipDir
		case !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md"):
			files = append(files, path)
		}
		return nil
	})
	return files
}
