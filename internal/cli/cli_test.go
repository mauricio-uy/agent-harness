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
		{"sync", "--root", root, "--apply", "--check"},
		{"sync", "--root", root, "unknown"},
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
	for _, key := range []string{"suites", "skills", "links"} {
		if result[key] == nil {
			t.Errorf("missing %s: %v", key, result)
		}
	}
	plans := result["suites"].(map[string]any)["plans"].(map[string]any)
	if errs, ok := plans["errors"].([]any); !ok || len(errs) != 0 {
		t.Errorf("errors must be an empty list, not null: %v", plans)
	}

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

func TestIDAndDatePrintOneBareLine(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "plans", "records"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "plans", "records", "PLAN-000003-x.md"), nil, 0o644)
	if status, out := run(t, nil, "id", "--root", root, "plan"); status != 0 || out != "PLAN-000004\n" {
		t.Fatalf("id: %d %q", status, out)
	}
	now = func() time.Time { return time.Date(2026, 3, 7, 23, 59, 0, 0, time.Local) }
	t.Cleanup(func() { now = time.Now })
	if status, out := run(t, nil, "date"); status != 0 || out != "2026-03-07\n" {
		t.Fatalf("date: %d %q", status, out)
	}
	for _, args := range [][]string{{"id", "--root", root}, {"id", "--root", root, "memo"}, {"id", "--root", root, "plan", "adr"}, {"date", "extra"}} {
		if status, _ := run(t, nil, args...); status != 1 {
			t.Errorf("%v must fail", args)
		}
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

func TestUpgradePreviewsThenApplies(t *testing.T) {
	root := t.TempDir()
	if status, out := run(t, nil, "upgrade", "--root", root); status != 1 || !strings.Contains(out, "harness init") {
		t.Fatalf("upgrade needs an installation: %d\n%s", status, out)
	}
	run(t, nil, "init", "--root", root, "--clients", "none")
	agents := filepath.Join(root, "AGENTS.md")
	os.Remove(agents)
	if status, out := run(t, nil, "upgrade", "--root", root); status != 0 || !strings.Contains(out, "CREATE AGENTS.md") {
		t.Fatalf("preview: %d\n%s", status, out)
	}
	if _, err := os.Stat(agents); err == nil {
		t.Fatal("the preview must not write")
	}
	if status, out := run(t, nil, "upgrade", "--root", root, "--apply"); status != 0 {
		t.Fatalf("apply: %d\n%s", status, out)
	}
	if _, err := os.Stat(agents); err != nil {
		t.Fatal("apply must restore the file")
	}
}

func TestDoctorFailsOnlyOnProblems(t *testing.T) {
	root := t.TempDir()
	if status, out := run(t, nil, "doctor", "--root", root); status != 1 || !strings.Contains(out, "run harness init") {
		t.Fatalf("no installation: %d\n%s", status, out)
	}
	run(t, nil, "init", "--root", root, "--clients", "none")
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
