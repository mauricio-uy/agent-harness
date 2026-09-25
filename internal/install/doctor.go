package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// hookGuide points to the README section on running the check from other hook tools.
const hookGuide = `see "Git hook" in the harness README`

// Doctor reports how the installation is set up in this clone. Warnings do
// not fail; it returns an error only when something is broken.
func (in *Installer) Doctor() error {
	problems := 0
	problem := func(format string, args ...any) {
		problems++
		report.Emitf(in.Out, report.Error, format, args...)
	}
	recorded, err := in.readState()
	switch {
	case err != nil:
		problem("%v", err)
	case !exists(filepath.Join(in.Root, filepath.FromSlash(stateFile))):
		problem("%s is missing; run harness init", stateFile)
	default:
		clients := strings.Join(recorded.Clients, ", ")
		if clients == "" {
			clients = "none"
		}
		report.Emitf(in.Out, report.OK, "%s (clients: %s)", stateFile, clients)
		in.checkVersion(recorded.Version)
	}
	problems += in.checkHook()
	if slices.Contains(recorded.Clients, "claude-code") {
		problems += in.checkClaude()
	}
	if problems > 0 {
		return fmt.Errorf("%d problem(s) need attention", problems)
	}
	return nil
}

func (in *Installer) checkVersion(installed string) {
	unknown := func(v string) bool { return v == "" || v == "dev" }
	switch {
	case unknown(installed) || unknown(in.Version):
	case installed != in.Version:
		report.Emitf(in.Out, report.Warn, "installed with %s, but this CLI is %s; run harness upgrade, or update the CLI if it is the older one", installed, in.Version)
	default:
		report.Emitf(in.Out, report.OK, "version %s", in.Version)
	}
}

// checkHook reports whether the pre-commit check runs; it returns the number of problems.
func (in *Installer) checkHook() int {
	hook := filepath.Join(in.Root, ".githooks", "pre-commit")
	info, err := os.Stat(hook)
	if err != nil {
		report.Emitf(in.Out, report.Error, ".githooks/pre-commit is missing; run harness upgrade --apply")
		return 1
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		report.Emitf(in.Out, report.Error, ".githooks/pre-commit is not executable; run chmod +x .githooks/pre-commit")
		return 1
	}
	setup := hookSetup(in.Root)
	switch {
	case !setup.repo:
		in.Out.Emit(report.Warn, "not a Git repository, so the pre-commit check cannot run")
	case setup.hooksPath == ".githooks" && len(setup.managers) > 0:
		report.Emitf(in.Out, report.Warn, "core.hooksPath points to .githooks, so the hooks of %s no longer run; %s", strings.Join(setup.managers, ", "), hookGuide)
	case setup.hooksPath == ".githooks":
		in.Out.Emit(report.OK, "pre-commit hook (core.hooksPath=.githooks)")
	case len(setup.others()) > 0:
		in.Out.Emit(report.Warn, setup.managersWarning())
	default:
		in.Out.Emit(report.Warn, "the pre-commit check is off; run git config core.hooksPath .githooks")
	}
	return 0
}

// checkClaude reports the state of the Claude Code links; it returns the number of problems.
func (in *Installer) checkClaude() int {
	problems := 0
	skills, err := in.claudeLinkedSkills()
	if err != nil {
		report.Emitf(in.Out, report.Error, "%v", err)
		return 1
	}
	for _, name := range skills {
		rel := ".claude/skills/" + name
		link := filepath.Join(in.Root, ".claude", "skills", name)
		switch {
		case !exists(link) && !isLink(link):
			report.Emitf(in.Out, report.Error, "%s is missing; run harness link", rel)
			problems++
		case !sameFile(link, filepath.Join(in.Root, ".agents", "skills", name)):
			report.Emitf(in.Out, report.Error, "%s does not point to .agents/skills/%s; run harness link", rel, name)
			problems++
		}
	}
	if problems == 0 {
		report.Emitf(in.Out, report.OK, ".claude/skills (%d links)", len(skills))
	}
	if ignore, err := os.ReadFile(filepath.Join(in.Root, ".gitignore")); err != nil || !strings.Contains(string(ignore), blockBegin) {
		in.Out.Emit(report.Warn, ".gitignore lacks the skill links block; run harness link")
	}
	for _, name := range []string{"CLAUDE.md", "CLAUDE.local.md"} {
		if exists(filepath.Join(in.Root, name)) {
			report.Emitf(in.Out, report.Warn, `%s exists, so Claude Code reads it instead of AGENTS.md; include AGENTS.md in "Project instructions" with /config`, name)
		}
	}
	return problems
}

func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// hooks describes how Git runs hooks in a clone.
type hooks struct {
	repo      bool
	hooksPath string
	// managers lists other tools that run hooks, such as husky or pre-commit.
	managers []string
}

// others lists every other source of hooks, including a foreign core.hooksPath.
func (h hooks) others() []string {
	if h.hooksPath != "" && h.hooksPath != ".githooks" {
		return append(slices.Clone(h.managers), "core.hooksPath="+h.hooksPath)
	}
	return h.managers
}

func (h hooks) managersWarning() string {
	return fmt.Sprintf("found %s; run harness check --staged from there instead of setting core.hooksPath (%s)", strings.Join(h.others(), ", "), hookGuide)
}

// hookSetup inspects the clone at root. Without Git it reports no repository.
func hookSetup(root string) hooks {
	var h hooks
	git := func(args ...string) (string, bool) {
		output, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
		return strings.TrimSpace(string(output)), err == nil
	}
	common, ok := git("rev-parse", "--git-common-dir")
	if !ok {
		return h
	}
	h.repo = true
	h.hooksPath, _ = git("config", "--get", "core.hooksPath")
	if exists(filepath.Join(root, ".husky")) {
		h.managers = append(h.managers, "husky")
	}
	for _, name := range []string{"lefthook.yml", "lefthook.yaml", "lefthook.json", ".lefthook.yml", ".lefthook.yaml"} {
		if exists(filepath.Join(root, name)) {
			h.managers = append(h.managers, "lefthook")
			break
		}
	}
	if exists(filepath.Join(root, ".pre-commit-config.yaml")) {
		h.managers = append(h.managers, "pre-commit")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	if exists(filepath.Join(common, "hooks", "pre-commit")) {
		h.managers = append(h.managers, "a pre-commit hook in .git/hooks")
	}
	return h
}
