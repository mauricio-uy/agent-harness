package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, input string, interactive bool, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	status := Run(args, IO{In: strings.NewReader(input), Out: &out, Err: &out, Interactive: interactive})
	return status, out.String()
}

func TestInitPromptsForClientsInATerminal(t *testing.T) {
	root := t.TempDir()
	status, out := run(t, "2, 4\n", true, "init", "--root", root)
	if status != 0 {
		t.Fatalf("status %d:\n%s", status, out)
	}
	state, _ := os.ReadFile(filepath.Join(root, ".agents", "harness.json"))
	if !strings.Contains(string(state), `"codex"`) || !strings.Contains(string(state), `"pi"`) || strings.Contains(string(state), "claude") {
		t.Fatalf("state %s", state)
	}
	if status, out = run(t, "", false, "check", "--root", root); status != 0 {
		t.Fatalf("check failed:\n%s", out)
	}
}

func TestInitWithoutTerminalInstallsOnlyTheBase(t *testing.T) {
	root := t.TempDir()
	status, out := run(t, "", false, "init", "--root", root)
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
		if status, out := run(t, "", false, args...); status != 1 {
			t.Errorf("%v: status %d:\n%s", args, status, out)
		}
	}
	if status, _ := run(t, "", false, "unknown"); status != 2 {
		t.Error("unknown command must exit 2")
	}
	if status, _ := run(t, "9\n", true, "init", "--root", root); status != 1 {
		t.Error("invalid prompt choice must fail")
	}
}
