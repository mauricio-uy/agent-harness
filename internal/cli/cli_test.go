package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/mauricio-uy/agent-harness/internal/ui"
)

func run(t *testing.T, selectClients func() ([]string, error), args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	status := Run(args, IO{Out: &out, Err: &out, SelectClients: selectClients})
	return status, out.String()
}

func choose(ids ...string) func() ([]string, error) {
	return func() ([]string, error) { return ids, nil }
}

func TestInitUsesTheInteractiveSelection(t *testing.T) {
	root := t.TempDir()
	status, out := run(t, choose("codex", "pi"), "init", "--root", root)
	if status != 0 {
		t.Fatalf("status %d:\n%s", status, out)
	}
	state, _ := os.ReadFile(filepath.Join(root, ".agents", "harness.json"))
	if !strings.Contains(string(state), `"codex"`) || !strings.Contains(string(state), `"pi"`) || strings.Contains(string(state), "claude") {
		t.Fatalf("state %s", state)
	}
	if !strings.Contains(out, "created") || strings.Contains(out, "\x1b[") {
		t.Fatalf("output must summarize without escape codes when not a terminal:\n%s", out)
	}
	if status, out = run(t, nil, "check", "--root", root); status != 0 {
		t.Fatalf("check failed:\n%s", out)
	}
}

func TestClientsFlagSkipsTheSelection(t *testing.T) {
	root := t.TempDir()
	prompted := false
	selectClients := func() ([]string, error) { prompted = true; return nil, nil }
	if status, out := run(t, selectClients, "init", "--root", root, "--clients", "none"); status != 0 || prompted {
		t.Fatalf("status %d, prompted %v:\n%s", status, prompted, out)
	}
}

func TestCancelledSelectionWritesNothing(t *testing.T) {
	root := t.TempDir()
	status, out := run(t, func() ([]string, error) { return nil, ui.ErrCancelled }, "init", "--root", root)
	entries, _ := os.ReadDir(root)
	if status != 1 || len(entries) != 0 || !strings.Contains(out, "nothing was written") {
		t.Fatalf("status %d, %d entries:\n%s", status, len(entries), out)
	}
}

func TestInitWithoutTerminalInstallsOnlyTheBase(t *testing.T) {
	root := t.TempDir()
	status, out := run(t, nil, "init", "--root", root)
	if status != 0 || !strings.Contains(out, "Pass --clients") {
		t.Fatalf("status %d:\n%s", status, out)
	}
}

func TestUsageErrors(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "--root", root, "--clients", "cursor"},
		{"docs", "list", "--root", root, "--limit", "-1"},
		{"docs", "get", "--root", root, "unknown"},
		{"check", "--root", root, "--format", "xml"},
		{"init", "--root", filepath.Join(root, "missing")},
	} {
		if status, out := run(t, nil, args...); status != 1 {
			t.Errorf("%v: status %d:\n%s", args, status, out)
		}
	}
	if status, _ := run(t, nil, "unknown"); status != 2 {
		t.Error("unknown command must exit 2")
	}
	if status, _ := run(t, func() ([]string, error) { return nil, errors.New("boom") }, "init", "--root", root); status != 1 {
		t.Error("a failed selection must fail")
	}
}

// runJSON runs check --format json with stdout and stderr kept apart, as an
// agent reading the JSON would.
func runJSON(t *testing.T, args ...string) (int, map[string]any) {
	t.Helper()
	var out, errOut bytes.Buffer
	status := Run(append([]string{"check", "--format", "json"}, args...), IO{Out: &out, Err: &errOut})
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("stdout must hold only JSON: %v\n%s\nstderr: %s", err, out.String(), errOut.String())
	}
	return status, result
}

func TestCheckJSONReportsEveryCheck(t *testing.T) {
	root := t.TempDir()
	if status, out := run(t, nil, "init", "--root", root, "--clients", "none"); status != 0 {
		t.Fatalf("init: %s", out)
	}
	status, result := runJSON(t, "--root", root)
	if status != 0 || result["ok"] != true {
		t.Fatalf("a clean install must pass: %d %v", status, result)
	}
	for _, key := range []string{"suites", "links"} {
		if result[key] == nil {
			t.Errorf("missing %s: %v", key, result)
		}
	}
	plans := result["suites"].(map[string]any)["plans"].(map[string]any)
	if errs, ok := plans["errors"].([]any); !ok || len(errs) != 0 {
		t.Errorf("errors must be an empty list, not null: %v", plans)
	}

	os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "plans", "records.md"), []byte("[x](missing.md)\n"), 0o644)
	status, result = runJSON(t, "--root", root)
	if status != 1 || result["ok"] != false {
		t.Fatalf("a broken link must fail: %d %v", status, result)
	}
	links := result["links"].(map[string]any)["errors"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["source"] != "docs/plans/records.md" {
		t.Errorf("unexpected link errors: %v", links)
	}
	suiteErrors := result["suites"].(map[string]any)["plans"].(map[string]any)["errors"].([]any)
	if len(suiteErrors) != 1 || suiteErrors[0].(map[string]any)["file"] != "docs/plans/records.md" {
		t.Errorf("suite errors must name their file: %v", suiteErrors)
	}
}

func TestCheckFindsTheProjectFromASubdirectory(t *testing.T) {
	for _, mode := range []string{"working-tree", "staged"} {
		t.Run(mode, func(t *testing.T) {
			var args []string
			root := t.TempDir()
			if mode == "staged" {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git is not available")
				}
				if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v\n%s", err, output)
				}
				args = append(args, "--staged")
			}
			if status, out := run(t, nil, "init", "--root", root, "--clients", "none"); status != 0 {
				t.Fatalf("init: %s", out)
			}
			if err := os.WriteFile(filepath.Join(root, "docs", "broken.md"), []byte("[x](missing.md)\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if mode == "staged" {
				if output, err := exec.Command("git", "-C", root, "add", ".").CombinedOutput(); err != nil {
					t.Fatalf("git add: %v\n%s", err, output)
				}
			}
			t.Chdir(filepath.Join(root, "docs"))
			status, result := runJSON(t, args...)
			links := result["links"].(map[string]any)["errors"].([]any)
			if status != 1 || len(links) != 1 || links[0].(map[string]any)["source"] != "docs/broken.md" {
				t.Fatalf("check must find the broken link in the project: %d %v", status, result)
			}
			project := result["links"].(map[string]any)["scanned"].(float64)
			status, result = runJSON(t, append(args, "--root", ".")...)
			if status != 1 || result["links"].(map[string]any)["scanned"].(float64) >= project {
				t.Fatalf("an explicit root must be used as given: %d %v", status, result)
			}
		})
	}
}

func TestLinkAndDoctorFindTheProjectFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	if status, out := run(t, nil, "init", "--root", root, "--clients", "claude-code"); status != 0 {
		t.Fatalf("init: %s", out)
	}
	link := filepath.Join(root, ".claude", "skills", "implement-plan")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "docs"))
	if status, out := run(t, nil, "link"); status != 0 {
		t.Fatalf("link: %s", out)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("link must recreate the skill link in the project: %v", err)
	}
	if _, out := run(t, nil, "doctor"); !strings.Contains(out, "clients: claude-code") {
		t.Fatalf("doctor must read the project's installation record:\n%s", out)
	}
}

func TestDatePrintsOneBareLine(t *testing.T) {
	now = func() time.Time { return time.Date(2026, 3, 7, 23, 59, 0, 0, time.Local) }
	t.Cleanup(func() { now = time.Now })
	if status, out := run(t, nil, "date"); status != 0 || out != "2026-03-07\n" {
		t.Fatalf("date: %d %q", status, out)
	}
	if status, _ := run(t, nil, "date", "extra"); status != 1 {
		t.Error("date must refuse arguments")
	}
}

func TestVersionPrefersLinkerThenModuleThenDev(t *testing.T) {
	module := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	for _, c := range []struct {
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"v1.2.3", module("v0.9.0"), "v1.2.3"},
		{"", module("v0.9.0"), "v0.9.0"},
		{"", module("(devel)"), "dev"},
		{"", nil, "dev"},
	} {
		if got := resolveVersion(c.linked, c.info); got != c.want {
			t.Errorf("resolveVersion(%q, %v) = %q, want %q", c.linked, c.info, got, c.want)
		}
	}
}

func TestDoctorFailsOnlyOnProblems(t *testing.T) {
	root := t.TempDir()
	if status, out := run(t, nil, "doctor", "--root", root); status != 1 || !strings.Contains(out, "run harness init") {
		t.Fatalf("no installation: %d\n%s", status, out)
	}
	run(t, nil, "init", "--root", root, "--clients", "none")
	// Another hook manager requires manual setup regardless of core.hooksPath.
	if err := os.MkdirAll(filepath.Join(root, ".husky"), 0o755); err != nil {
		t.Fatal(err)
	}
	if status, out := run(t, nil, "doctor", "--root", root); status != 0 || !strings.Contains(out, "WARN") {
		t.Fatalf("warnings alone must not fail: %d\n%s", status, out)
	}
}

func TestDoctorSummarizesAHealthySetup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "core.hooksPath", ".githooks"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	run(t, nil, "init", "--root", root, "--clients", "none")
	status, out := run(t, nil, "doctor", "--root", root)
	if status != 0 || !strings.Contains(out, "everything is set up") || strings.Contains(out, "no changes") {
		t.Fatalf("status %d:\n%s", status, out)
	}
}
