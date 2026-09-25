package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// symlink is replaceable so tests can exercise the junction fallback.
var symlink = os.Symlink

const (
	blockBegin = "# BEGIN agent-harness: skill links, recreated by `harness link`"
	blockEnd   = "# END agent-harness"
)

// linkClaudeSkills links every skill into .claude/skills, which is the only
// project location Claude Code scans. A skill with an adapter in the payload
// keeps the adapter instead, so its Claude-specific frontmatter stays out of
// the shared skill.
func (in *Installer) linkClaudeSkills() error {
	entries, err := os.ReadDir(filepath.Join(in.Root, ".agents", "skills"))
	if err != nil {
		return err
	}
	var linked []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := fs.Stat(in.Payload, path.Join("template-clients/claude-code/.claude/skills", name)); err == nil {
			continue
		}
		rel := ".claude/skills/" + name
		link := filepath.Join(in.Root, ".claude", "skills", name)
		target := filepath.Join(in.Root, ".agents", "skills", name)
		kind, err := ensureLink(link, target)
		switch {
		case err != nil:
			in.warn("%s: %v", rel, err)
		case kind == "":
			fmt.Fprintf(in.Out, "OK     %s\n", rel)
		default:
			fmt.Fprintf(in.Out, "LINK   %s -> .agents/skills/%s (%s)\n", rel, name, kind)
		}
		linked = append(linked, "/"+rel)
	}
	return in.updateIgnoreBlock(linked)
}

// ensureLink makes link point at the target directory. It returns the kind of
// link created, or "" when a correct link already exists.
func ensureLink(link, target string) (string, error) {
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			return "", errors.New("exists and is not a link; move it aside and rerun")
		}
		if sameFile(link, target) {
			return "", nil
		}
		if err := os.Remove(link); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(filepath.Dir(link), target)
	if err != nil {
		return "", err
	}
	symlinkErr := symlink(relative, link)
	if symlinkErr == nil {
		return "symlink", nil
	}
	if runtime.GOOS != "windows" {
		return "", symlinkErr
	}
	// Directory junctions need no administrator rights or Developer Mode.
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		return "", fmt.Errorf("cannot create a symlink (%v) or a junction (%s)", symlinkErr, strings.TrimSpace(string(output)))
	}
	return "junction", nil
}

func sameFile(a, b string) bool {
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	return err == nil && os.SameFile(left, right)
}

// updateIgnoreBlock keeps generated links out of Git: a committed link breaks
// silently on checkouts without symlink support.
func (in *Installer) updateIgnoreBlock(entries []string) error {
	target := filepath.Join(in.Root, ".gitignore")
	raw, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	block := strings.Join(slices.Concat([]string{blockBegin}, entries, []string{blockEnd}), "\n") + "\n"
	var updated string
	begin, end := strings.Index(content, blockBegin), strings.Index(content, blockEnd)
	switch {
	case begin >= 0 && end > begin:
		updated = content[:begin] + block + strings.TrimPrefix(content[end+len(blockEnd):], "\n")
	case content == "":
		updated = block
	default:
		updated = strings.TrimRight(content, "\n") + "\n\n" + block
	}
	if updated == content {
		return nil
	}
	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(in.Out, "UPDATE .gitignore (skill links)")
	return nil
}
