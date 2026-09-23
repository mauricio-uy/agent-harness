package docs

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
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
	status := CheckStaged(f.root, LinkReport{}, &out)
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
	f.write("docs/research/README.md", "stale\n")
	f.write("README.md", "[Missing](missing.md)\n")
	f.git("add", ".")
	r := f.checkStaged()
	expectStatus(t, r, 1)
	r.contains(t, "== research ==", "INDEX docs/research/README.md", "missing.md")
}

func TestCheckRejectsNonStandardSkillFrontmatter(t *testing.T) {
	f := newFixture(t)
	f.write(".agents/skills/Bad_Name/SKILL.md", "---\nname: other\ndisable-model-invocation: true\n---\n")
	var out bytes.Buffer
	if CheckSkills(f.root, &out) != 1 {
		t.Fatal("invalid skill must fail")
	}
	for _, want := range []string{"disable-model-invocation", "name must", "description must"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
