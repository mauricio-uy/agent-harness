package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	harness "github.com/mauricio-uy/agent-harness"
	"github.com/mauricio-uy/agent-harness/internal/docs"
)

var skills = []string{"implement-plan", "write-plan", "write-research-report", "write-runbook", "write-specification"}

func install(t *testing.T, root string, clients ...string) string {
	t.Helper()
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: &out}
	if err := in.Init(clients); err != nil {
		t.Fatalf("init: %v\n%s", err, out.String())
	}
	return out.String()
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func present(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func check(t *testing.T, root string) {
	t.Helper()
	var out bytes.Buffer
	if docs.Check(root, docs.LinkReport{}, &out) != 0 {
		t.Fatalf("installed project fails its checks:\n%s", out.String())
	}
}

func TestBaseInstallMatchesTemplateAndPassesChecks(t *testing.T) {
	root := t.TempDir()
	install(t, root)
	for _, rel := range []string{"AGENTS.md", "docs/overview.md", "docs/glossary.md", "docs/plans/draft/README.md",
		".agents/shared/README.md", ".githooks/pre-commit", ".agents/harness.json"} {
		if !present(root, rel) {
			t.Errorf("missing %s", rel)
		}
	}
	for _, rel := range []string{".claude", "opencode.json", "CLAUDE.md", ".agents/skills/write-plan/agents"} {
		if present(root, rel) {
			t.Errorf("base install must not add %s", rel)
		}
	}
	check(t, root)
}

func TestFullInstallWithEveryClientPassesChecks(t *testing.T) {
	root := t.TempDir()
	install(t, root, "claude-code", "codex", "opencode", "pi")
	for _, skill := range skills {
		link := ".claude/skills/" + skill
		if skill == "write-plan" {
			if !strings.Contains(read(t, root, link+"/SKILL.md"), "disable-model-invocation: true") {
				t.Error("write-plan must use the manual-only adapter")
			}
			continue
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(link)))
		if err != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			t.Errorf("%s must be a link: %v", link, err)
		}
		if !strings.Contains(read(t, root, link+"/SKILL.md"), "name: "+skill) {
			t.Errorf("%s must resolve to the canonical skill", link)
		}
		if !strings.Contains(read(t, root, ".gitignore"), "/"+link+"\n") {
			t.Errorf(".gitignore must list %s", link)
		}
		if !present(root, ".agents/skills/"+skill+"/agents/openai.yaml") {
			t.Errorf("codex policy missing for %s", skill)
		}
	}
	if !strings.Contains(read(t, root, "opencode.json"), `"write-plan": "deny"`) || !present(root, ".opencode/commands/write-plan.md") {
		t.Error("opencode files missing")
	}
	if present(root, "CLAUDE.md") {
		t.Error("CLAUDE.md must not be created")
	}
	if got := read(t, root, ".agents/harness.json"); !strings.Contains(got, `"claude-code"`) || !strings.Contains(got, `"pi"`) {
		t.Errorf("state %s", got)
	}
	check(t, root)
}

func TestInitIsIdempotentAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Our own rules\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)
	out := install(t, root, "claude-code")
	if !strings.Contains(out, "SKIP   AGENTS.md (exists)") || read(t, root, "AGENTS.md") != "# Our own rules\n" {
		t.Fatalf("existing file must be kept:\n%s", out)
	}
	ignore := read(t, root, ".gitignore")
	if !strings.HasPrefix(ignore, "node_modules/\n\n"+blockBegin) {
		t.Fatalf(".gitignore must keep its content:\n%s", ignore)
	}
	out = install(t, root, "claude-code")
	if strings.Contains(out, "CREATE") || strings.Contains(out, "LINK ") || strings.Contains(out, "UPDATE") {
		t.Fatalf("second run must change nothing:\n%s", out)
	}
	if read(t, root, ".gitignore") != ignore {
		t.Fatal(".gitignore must be stable")
	}
}

func TestLinkRecreatesMissingAndBrokenLinks(t *testing.T) {
	root := t.TempDir()
	install(t, root, "claude-code")
	missing := filepath.Join(root, ".claude", "skills", "implement-plan")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: &out}
	if err := in.Link(); err != nil {
		t.Fatalf("link: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "LINK   .claude/skills/implement-plan") {
		t.Fatalf("missing link must be recreated:\n%s", out.String())
	}
	check(t, root)
}

func TestLinkRefusesToReplaceARealDirectory(t *testing.T) {
	root := t.TempDir()
	install(t, root)
	os.MkdirAll(filepath.Join(root, ".claude", "skills", "implement-plan"), 0o755)
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: &out}
	if err := in.Init([]string{"claude-code"}); err == nil {
		t.Fatalf("a real directory must not be replaced:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "is not a link") {
		t.Fatalf("warning missing:\n%s", out.String())
	}
}

func TestClaudeMDGetsAnImportOnlyWhenItExists(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Team notes\n"), 0o644)
	install(t, root, "claude-code")
	if got := read(t, root, "CLAUDE.md"); got != "@AGENTS.md\n\n# Team notes\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	install(t, root, "claude-code")
	if got := read(t, root, "CLAUDE.md"); strings.Count(got, "@AGENTS.md") != 1 {
		t.Fatalf("import must not repeat: %q", got)
	}
}

func TestOpenCodeConfigIsMergedPreservingProjectSettings(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "opencode.json"), []byte(`{
  "model": "some/model",
  "permission": {
    "skill": {
      "write-runbook": "deny"
    },
    "bash": "ask"
  }
}
`), 0o644)
	install(t, root, "opencode")
	got := read(t, root, "opencode.json")
	for _, want := range []string{`"model": "some/model"`, `"write-runbook": "deny"`, `"write-plan": "deny"`, `"implement-plan": "allow"`, `"bash": "ask"`} {
		if !strings.Contains(got, want) {
			t.Errorf("opencode.json lacks %s:\n%s", want, got)
		}
	}
	if strings.Index(got, `"model"`) > strings.Index(got, `"permission"`) {
		t.Errorf("key order must be preserved:\n%s", got)
	}
}

func TestOpenCodeConfigWithCommentsIsReportedNotRewritten(t *testing.T) {
	root := t.TempDir()
	original := "{\n  // comment\n  \"model\": \"x\"\n}\n"
	os.WriteFile(filepath.Join(root, "opencode.json"), []byte(original), 0o644)
	var out bytes.Buffer
	in := &Installer{Root: root, Payload: harness.Payload, Out: &out}
	if err := in.Init([]string{"opencode"}); err == nil || !strings.Contains(out.String(), "not plain JSON") {
		t.Fatalf("expected a warning:\n%s", out.String())
	}
	if read(t, root, "opencode.json") != original {
		t.Fatal("opencode.json must not be rewritten")
	}
}

func TestParseClients(t *testing.T) {
	if got, err := ParseClients("codex, claude-code,codex"); err != nil || strings.Join(got, ",") != "codex,claude-code" {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := ParseClients("none"); err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ParseClients("cursor"); err == nil {
		t.Fatal("unknown clients must be rejected")
	}
}

func TestWindowsFallsBackToJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions exist only on Windows")
	}
	symlink = func(string, string) error { return errors.New("symlink privilege not held") }
	t.Cleanup(func() { symlink = os.Symlink })
	root := t.TempDir()
	out := install(t, root, "claude-code")
	if !strings.Contains(out, "(junction)") {
		t.Fatalf("expected junctions:\n%s", out)
	}
	if out = install(t, root, "claude-code"); !strings.Contains(out, "OK     .claude/skills/implement-plan") {
		t.Fatalf("existing junctions must be recognized:\n%s", out)
	}
	check(t, root)
}
