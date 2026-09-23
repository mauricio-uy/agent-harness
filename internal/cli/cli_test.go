package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
