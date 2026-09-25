package install

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	harness "github.com/mauricio-uy/agent-harness"
	"github.com/mauricio-uy/agent-harness/internal/report"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	gitIn(t, root, "init", "--quiet")
	return root
}

func gitIn(t *testing.T, root string, args ...string) {
	t.Helper()
	if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func doctor(t *testing.T, root, version string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: report.Text{W: &out}, Version: version}
	err := in.Doctor()
	return out.String(), err
}

func expect(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(out, part) {
			t.Errorf("output lacks %q:\n%s", part, out)
		}
	}
}

func TestDoctorReportsAHealthyInstallation(t *testing.T) {
	root := gitRepo(t)
	install(t, root, "claude-code")
	gitIn(t, root, "config", "core.hooksPath", ".githooks")
	out, err := doctor(t, root, "dev")
	if err != nil || strings.Contains(out, "WARN") || strings.Contains(out, "ERROR") {
		t.Fatalf("a fresh installation is healthy: %v\n%s", err, out)
	}
	expect(t, out, "OK     .agents/harness.json", "OK     pre-commit hook", "OK     .claude/skills")
}

func TestDoctorFindsProblemsAndWarnings(t *testing.T) {
	root := gitRepo(t)
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: report.Text{W: &out}, Version: "v1.0.0"}
	if err := in.Init([]string{"claude-code"}); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, ".claude", "skills", "implement-plan"))
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Notes\n"), 0o644)
	got, err := doctor(t, root, "v2.0.0")
	if err == nil {
		t.Fatalf("a missing link is a problem:\n%s", got)
	}
	expect(t, got,
		"ERROR: .claude/skills/implement-plan is missing; run harness link",
		"WARN   CLAUDE.md exists",
		"WARN   installed with v1.0.0",
		"WARN   the pre-commit check is off",
	)
}

func TestDoctorWithoutInstallation(t *testing.T) {
	out, err := doctor(t, t.TempDir(), "dev")
	if err == nil {
		t.Fatal("no installation is a problem")
	}
	expect(t, out, "ERROR: .agents/harness.json is missing; run harness init")
}

func TestOtherHookManagersAreDetected(t *testing.T) {
	root := gitRepo(t)
	install(t, root)
	os.WriteFile(filepath.Join(root, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644)
	os.MkdirAll(filepath.Join(root, ".husky"), 0o755)
	out, _ := doctor(t, root, "dev")
	expect(t, out, "WARN   found husky, pre-commit", "harness check --staged")
	if strings.Contains(out, "the pre-commit check is off") {
		t.Errorf("with another hook manager, do not suggest core.hooksPath:\n%s", out)
	}

	gitIn(t, root, "config", "core.hooksPath", ".githooks")
	out, _ = doctor(t, root, "dev")
	expect(t, out, "no longer run")

	var initOut bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: report.Text{W: &initOut}}
	gitIn(t, root, "config", "--unset", "core.hooksPath")
	if err := in.Init(nil); err != nil {
		t.Fatal(err)
	}
	expect(t, initOut.String(), "WARN   found husky, pre-commit")
	if strings.Contains(initOut.String(), "git config core.hooksPath") {
		t.Errorf("init must not suggest core.hooksPath next to another hook manager:\n%s", initOut.String())
	}
}

func TestPreCommitHookWithoutTheCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook runs under a POSIX shell")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not available")
	}
	hook := filepath.Join(t.TempDir(), "pre-commit")
	content, _ := harness.Payload.ReadFile("template/.githooks/pre-commit")
	os.WriteFile(hook, content, 0o755)
	for strict, wantStatus := range map[string]int{"": 0, "1": 1} {
		command := exec.Command(sh, hook)
		command.Env = []string{"PATH=" + t.TempDir(), "HARNESS_HOOK_STRICT=" + strict}
		output, err := command.CombinedOutput()
		status := 0
		if exit, ok := err.(*exec.ExitError); ok {
			status = exit.ExitCode()
		}
		if status != wantStatus || !strings.Contains(string(output), "harness CLI is not on PATH") {
			t.Errorf("strict=%q: status %d, want %d:\n%s", strict, status, wantStatus, output)
		}
	}
}
