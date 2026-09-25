package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// payload builds a small embedded payload from paths relative to template/.
func payload(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for rel, content := range files {
		fsys["template/"+rel] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func readState(t *testing.T, root string) state {
	t.Helper()
	var s state
	if err := json.Unmarshal([]byte(read(t, root, stateFile)), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func runInstaller(t *testing.T, root string, fsys fstest.MapFS, version string, do func(*Installer) error) (string, error) {
	t.Helper()
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: fsys, Out: report.Text{W: &out}, Version: version}
	err := do(in)
	return out.String(), err
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitRecordsVersionAndHashesOfInstalledFiles(t *testing.T) {
	root := t.TempDir()
	install(t, root, "claude-code", "opencode")
	s := readState(t, root)
	for _, rel := range []string{"AGENTS.md", ".agents/skills/write-plan/SKILL.md", ".claude/skills/write-plan/SKILL.md", ".githooks/pre-commit"} {
		if !strings.HasPrefix(s.Files[rel], "sha256:") {
			t.Errorf("%s must be recorded: %v", rel, s.Files[rel])
		}
	}
	for _, rel := range []string{"docs/plans/draft.md", "opencode.json"} {
		if _, ok := s.Files[rel]; ok {
			t.Errorf("%s is generated or merged and must not be recorded", rel)
		}
	}
}

func TestUpgradeReplacesOnlyWhatTheProjectLeftUntouched(t *testing.T) {
	root := t.TempDir()
	v1 := payload(map[string]string{
		"AGENTS.md":           "rules v1\n",
		"docs/overview.md":    "overview v1\n",
		"docs/old.md":         "old\n",
		"docs/plans/draft.md": "index v1\n",
	})
	v2 := payload(map[string]string{
		"AGENTS.md":           "rules v2\n",
		"docs/overview.md":    "overview v2\n",
		"docs/new.md":         "new\n",
		"docs/plans/draft.md": "index v2\n",
	})
	if out, err := runInstaller(t, root, v1, "v1.0.0", func(in *Installer) error { return in.Init(nil) }); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	write(t, root, "docs/overview.md", "our overview\n")
	write(t, root, "docs/plans/draft.md", "regenerated index\n")

	out, err := runInstaller(t, root, v2, "v2.0.0", func(in *Installer) error { return in.Upgrade(false) })
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, out)
	}
	for _, want := range []string{"UPDATE AGENTS.md", "CREATE docs/new.md", "SKIP   docs/overview.md (changed in this project", "SKIP   docs/old.md (no longer installed", "--apply"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "draft.md") {
		t.Errorf("generated indexes belong to sync:\n%s", out)
	}
	if read(t, root, "AGENTS.md") != "rules v1\n" || present(root, "docs/new.md") || readState(t, root).Version != "v1.0.0" {
		t.Fatal("a preview must not write")
	}

	if out, err = runInstaller(t, root, v2, "v2.0.0", func(in *Installer) error { return in.Upgrade(true) }); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	for rel, want := range map[string]string{"AGENTS.md": "rules v2\n", "docs/overview.md": "our overview\n", "docs/new.md": "new\n", "docs/old.md": "old\n", "docs/plans/draft.md": "regenerated index\n"} {
		if got := read(t, root, rel); got != want {
			t.Errorf("%s: got %q, want %q", rel, got, want)
		}
	}
	s := readState(t, root)
	if s.Version != "v2.0.0" || s.Files["AGENTS.md"] != digest([]byte("rules v2\n")) {
		t.Errorf("state must follow the new payload: %+v", s)
	}
	if _, kept := s.Files["docs/old.md"]; kept {
		t.Error("a file the harness no longer installs must leave the record")
	}
	if out, _ = runInstaller(t, root, v2, "v2.0.0", func(in *Installer) error { return in.Upgrade(false) }); !strings.Contains(out, "up to date") {
		t.Errorf("a second upgrade has nothing to do except report the kept change:\n%s", out)
	}
}

func TestUpgradeTreatsLineEndingsAsUnchanged(t *testing.T) {
	root := t.TempDir()
	v1 := payload(map[string]string{"AGENTS.md": "a\nb\n"})
	v2 := payload(map[string]string{"AGENTS.md": "a\nb\nc\n"})
	runInstaller(t, root, v1, "v1", func(in *Installer) error { return in.Init(nil) })
	write(t, root, "AGENTS.md", "a\r\nb\r\n")
	if out, err := runInstaller(t, root, v2, "v2", func(in *Installer) error { return in.Upgrade(true) }); err != nil || read(t, root, "AGENTS.md") != "a\nb\nc\n" {
		t.Fatalf("a checkout with CRLF endings is not a local change: %v\n%s", err, out)
	}
}

func TestUpgradeAdoptsAnInstallationWithoutHashes(t *testing.T) {
	root := t.TempDir()
	v1 := payload(map[string]string{"AGENTS.md": "rules\n", "docs/overview.md": "overview\n"})
	runInstaller(t, root, v1, "", func(in *Installer) error { return in.Init(nil) })
	write(t, root, stateFile, "{\n  \"clients\": []\n}\n")
	write(t, root, "docs/overview.md", "ours\n")
	v2 := payload(map[string]string{"AGENTS.md": "rules\n", "docs/overview.md": "overview v2\n"})
	out, err := runInstaller(t, root, v2, "v2", func(in *Installer) error { return in.Upgrade(true) })
	if err != nil || !strings.Contains(out, "SKIP   docs/overview.md") || read(t, root, "docs/overview.md") != "ours\n" {
		t.Fatalf("a file of unknown origin must be kept: %v\n%s", err, out)
	}
	if readState(t, root).Files["AGENTS.md"] != digest([]byte("rules\n")) {
		t.Fatal("a file equal to the payload must be adopted")
	}
}

func TestUpgradeNeedsAnInstallation(t *testing.T) {
	_, err := runInstaller(t, t.TempDir(), payload(nil), "v1", func(in *Installer) error { return in.Upgrade(false) })
	if err == nil || !strings.Contains(err.Error(), "harness init") {
		t.Fatalf("expected a hint to run init: %v", err)
	}
}
