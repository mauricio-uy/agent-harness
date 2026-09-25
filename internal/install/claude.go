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

	"github.com/mauricio-uy/agent-harness/internal/report"
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
	skills, err := in.claudeLinkedSkills()
	if err != nil {
		return err
	}
	var linked []string
	for _, name := range skills {
		rel := ".claude/skills/" + name
		link := filepath.Join(in.Root, ".claude", "skills", name)
		target := filepath.Join(in.Root, ".agents", "skills", name)
		kind, err := ensureLink(link, target)
		switch {
		case err != nil:
			in.warn("%s: %v", rel, err)
		case kind == "":
			in.Out.Emit(report.OK, rel)
		default:
			report.Emitf(in.Out, report.Link, "%s -> .agents/skills/%s (%s)", rel, name, kind)
		}
		linked = append(linked, "/"+rel)
	}
	return in.updateIgnoreBlock(linked)
}

// claudeLinkedSkills lists the skills Claude Code reaches through a link:
// every skill in .agents/skills without a Claude Code adapter in the payload.
func (in *Installer) claudeLinkedSkills() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(in.Root, ".agents", "skills"))
	if err != nil {
		return nil, err
	}
	var skills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := fs.Stat(in.Payload, path.Join("template-clients/claude-code/.claude/skills", entry.Name())); err == nil {
			continue
		}
		skills = append(skills, entry.Name())
	}
	return skills, nil
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
	in.Out.Emit(report.Update, ".gitignore (skill links)")
	return nil
}
