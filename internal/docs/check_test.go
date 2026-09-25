package docs

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	output, err := exec.Command("git", append([]string{"-C", f.root}, args...)...).CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

// stagedFixture is a repository with valid, generated indexes, all staged.
func stagedFixture(t *testing.T) *fixture {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	f := newFixture(t)
	f.git("init", "--quiet")
	f.git("config", "core.autocrlf", "false")
	f.write(".agents/skills/example/SKILL.md", "---\nname: example\ndescription: An example skill for tests.\n---\n\n# Example\n")
	f.write("docs/plans/README.md", "# Documents\n")
	for _, name := range Suites {
		expectStatus(t, f.sync(name, true, false), 0)
	}
	f.write("README.md", "# Fixture\n")
	f.write("docs/README.md", "# Documents\n")
	f.git("add", ".")
	return f
}

func (f *fixture) checkStaged() result {
	f.t.Helper()
	before := f.git("diff", "--cached", "--binary")
	var out bytes.Buffer
	status := CheckStaged(f.root, LinkReport{}, report.Text{W: &out})
	if f.git("diff", "--cached", "--binary") != before {
		f.t.Fatal("staged check must not change the index")
	}
	return result{status, out.String()}
}

func TestStagedUnstagedFixDoesNotHideStagedBrokenLink(t *testing.T) {
	f := stagedFixture(t)
	f.write("README.md", "[Missing](missing.md)\n")
	f.git("add", "README.md")
	f.write("README.md", "# Fixed locally\n")
	r := f.checkStaged()
	expectStatus(t, r, 1)
	r.contains(t, "missing.md")
	if f.read("README.md") != "# Fixed locally\n" {
		t.Fatal("working tree must not change")
	}
}

func TestStagedUnstagedErrorDoesNotBlockValidIndex(t *testing.T) {
	f := stagedFixture(t)
	f.write("README.md", "[Missing](missing.md)\n")
	expectStatus(t, f.checkStaged(), 0)
}

func TestStagedStaleIndexAndLinksAreBothReported(t *testing.T) {
	f := stagedFixture(t)
	f.write("docs/research/draft.md", "stale\n")
	f.write("README.md", "[Missing](missing.md)\n")
	f.git("add", ".")
	r := f.checkStaged()
	expectStatus(t, r, 1)
	r.contains(t, "== research ==", "INDEX docs/research/draft.md", "missing.md")
}

func TestCheckRejectsNonStandardSkillFrontmatter(t *testing.T) {
	f := newFixture(t)
	f.write(".agents/skills/Bad_Name/SKILL.md", "---\nname: other\ndisable-model-invocation: true\n---\n")
	var out bytes.Buffer
	if CheckSkills(f.root, report.Text{W: &out}) != 1 {
		t.Fatal("invalid skill must fail")
	}
	for _, want := range []string{"disable-model-invocation", "name must", "description must"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestStagedLinksToCodeAndFoldersUseTheIndex(t *testing.T) {
	f := stagedFixture(t)
	f.write("internal/app/main.go", "package main\n")
	f.write("docs/guide.md", "[Code](../internal/app/main.go), [folder](../internal/app/) and [bare folder](../internal)\n")
	f.git("add", ".")
	expectStatus(t, f.checkStaged(), 0)

	f.write("internal/app/new.go", "package main\n")
	f.write("docs/guide.md", "[New](../internal/app/new.go)\n")
	f.git("add", "docs/guide.md")
	r := f.checkStaged()
	expectStatus(t, r, 1)
	r.contains(t, "internal/app/new.go", "missing target")
}

func TestStagedSnapshotHoldsOnlyMarkdown(t *testing.T) {
	f := stagedFixture(t)
	f.write("internal/app/main.go", "package main\n")
	f.git("add", ".")
	snapshot, known, err := stagedSnapshot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(snapshot)
	if exists(snapshot + "/internal/app/main.go") {
		t.Error("code must not be copied")
	}
	if !exists(snapshot+"/docs/README.md") || !known.exists(snapshot+"/internal/app/main.go") || !known.isDir(snapshot+"/internal/app") {
		t.Error("Markdown is copied and every staged path is known")
	}
}
