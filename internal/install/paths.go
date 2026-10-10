package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// safePath prevents installation writes outside the project or through links.
func safePath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("installation path escapes project: %s", target)
	}
	path := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("installation requires regular paths: %s", path)
		}
	}
	return nil
}

// FindRoot returns the nearest directory, from dir upward, that holds an
// installation. Agents and clients may start in a subdirectory of the project.
func FindRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(stateFile))); err == nil && info.Mode().IsRegular() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
