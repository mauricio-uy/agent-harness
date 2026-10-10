package docs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinksReportAllErrorsAndSuggestMovedPlan(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "completed", "1", 1)
	f.write("docs/guide.md", "# Guide\n\n[Plan](plans/records/PLAN-000001-example.md)\n\n![Image](missing.png)\n")
	report := filepath.Join(f.root, "report")
	r, _ := f.links(LinkReport{ReportDir: report})
	expectStatus(t, r, 1)
	errors := readLinkReport(t, report)
	if len(errors) != 2 {
		t.Fatalf("want 2 errors, got %v", errors)
	}
	for _, e := range errors {
		if strings.Contains(e.Destination, "PLAN-") {
			if e.Line != 3 || e.Suggestion != "plans/PLAN-000001-example.md" {
				t.Fatalf("unexpected plan error %+v", e)
			}
		}
	}
}

func TestLinksMarkdownReferencesHTMLAndDuplicateAnchors(t *testing.T) {
	f := newFixture(t)
	f.write("docs/target file.md", "# Intro\n\n## Repeat\n\n## Repeat\n\n<a id=\"custom\"></a>\n")
	f.write("docs/index.md", "[A](target%20file.md#repeat-1)\n\n[B][target]\n\n"+
		"[target]: <target file.md#intro>\n\n<a href=\"target%20file.md#custom\">C</a>\n")
	r, _ := f.links(LinkReport{})
	expectStatus(t, r, 0)
	f.write("docs/bad.md", "[A](target%20file.md#absent)\n")
	r, _ = f.links(LinkReport{})
	expectStatus(t, r, 1)
	r.contains(t, "fragment")
}

func TestLinksIgnoreExamplesFrontmatterCommentsAndRemoteURLs(t *testing.T) {
	f := newFixture(t)
	f.write("docs/guide.md", "---\nexample: '[X](missing.md)'\n---\n\n"+
		"```md\n[X](missing.md)\n```\n\n`[X](missing.md)`\n\n"+
		"<!-- [X](missing.md) <a href=\"missing.md\">x</a> -->\n\n"+
		"[Remote](https://example.invalid/file)\n")
	r, _ := f.links(LinkReport{})
	expectStatus(t, r, 0)
}

func TestLinksCheckEveryProjectMarkdownFileButIgnoredOnes(t *testing.T) {
	for _, repository := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "repository"}[repository], func(t *testing.T) {
			f := newFixture(t)
			if repository {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git is not available")
				}
				f.git("init", "--quiet")
			}
			os.MkdirAll(f.path("docs"), 0o755)
			sources := []string{
				".agents/skills/implement-plan/SKILL.md",
				".claude/skills/write-plan/SKILL.md",
				".opencode/commands/write-plan.md",
				"AGENTS.md",
				"CLAUDE.md",
				"CONTRIBUTING.md",
				"README.md",
				"src/api/README.md",
			}
			skipped := []string{"vendor/lib/README.md", "notes/draft.md", "node_modules/pkg/README.md"}
			for _, source := range append(sources, skipped...) {
				f.write(source, "# Source\n\n[Missing](missing.md)\n")
			}
			f.write(".gitignore", "node_modules/\n")
			f.write(ignoreFile, "# Third-party code\nvendor/\n/notes/*.md\n")
			_, errors := f.links(LinkReport{})
			found := map[string]bool{}
			for _, e := range errors {
				found[e.Source] = true
			}
			for _, source := range sources {
				if !found[source] {
					t.Errorf("source not scanned: %s", source)
				}
			}
			for _, source := range skipped {
				if found[source] {
					t.Errorf("ignored source was scanned: %s", source)
				}
			}
		})
	}
}

func TestIgnoreFileFollowsGitignorePatterns(t *testing.T) {
	f := newFixture(t)
	f.write(ignoreFile, strings.Join([]string{
		"# comment", "", "build", "/top.md", "docs/generated/", "**/fixtures/*.md", "a/**/z.md", "draft-?.md", "[xy].md",
	}, "\n"))
	rules, errors := loadIgnore(f.root)
	if len(errors) != 0 {
		t.Fatalf("valid patterns must load: %+v", errors)
	}
	for rel, want := range map[string]bool{
		"build/out.md":          true,
		"src/build/out.md":      true,
		"src/builder.md":        false,
		"top.md":                true,
		"sub/top.md":            false,
		"docs/generated/api.md": true,
		"docs/generated.md":     false,
		"x/y/fixtures/case.md":  true,
		"fixtures/case.md":      true,
		"a/z.md":                true,
		"a/b/c/z.md":            true,
		"b/a/z.md":              false,
		"notes/draft-1.md":      true,
		"notes/draft-10.md":     false,
		"x.md":                  true,
		"z.md":                  false,
	} {
		if got := rules.ignores(rel); got != want {
			t.Errorf("%s: ignored=%v, want %v", rel, got, want)
		}
	}
}

func TestIgnoreFileReportsPatternsItCannotApply(t *testing.T) {
	f := newFixture(t)
	f.write(ignoreFile, "vendor/\n!vendor/keep.md\n/\n")
	rules, errors := loadIgnore(f.root)
	if len(rules) != 1 || len(errors) != 2 || errors[0].Line != 2 || errors[1].Line != 3 {
		t.Fatalf("negation and empty patterns must be reported: %d rules, %+v", len(rules), errors)
	}
	os.MkdirAll(f.path("docs"), 0o755)
	if r, _ := f.links(LinkReport{}); r.status != 1 || !strings.Contains(r.out, "negated patterns are not supported") {
		t.Fatalf("check must fail on an unusable pattern:\n%s", r.out)
	}
}

func TestLinksCompleteReportsAndBoundedGitHubSummary(t *testing.T) {
	f := newFixture(t)
	var lines []string
	for i := range 120 {
		lines = append(lines, fmt.Sprintf("[Missing %d](absent-%d.md)", i, i))
	}
	f.write("docs/bad.md", strings.Join(lines, "\n\n"))
	summary := filepath.Join(f.root, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	report := filepath.Join(f.root, "report")
	r, _ := f.links(LinkReport{GitHub: true, ReportDir: report})
	expectStatus(t, r, 1)
	r.contains(t, "::error file=docs/bad.md,line=1")
	if n := len(readLinkReport(t, report)); n != 120 {
		t.Fatalf("report has %d errors", n)
	}
	if !strings.Contains(f.read("report/links.md"), "absent-119.md") {
		t.Fatal("markdown report must be complete")
	}
	if info, _ := os.Stat(summary); info.Size() >= 65536 {
		t.Fatal("summary must stay bounded")
	}
}

func TestLinksRejectBackslashesAndTargetsOutsideRoot(t *testing.T) {
	f := newFixture(t)
	f.write("docs/guide.md", "[A](sub\\file.md)\n\n[B](../../outside.md)\n")
	r, _ := f.links(LinkReport{})
	expectStatus(t, r, 1)
	r.contains(t, "forward slashes", "outside repository")
}

func TestHeadingSlugs(t *testing.T) {
	for heading, want := range map[string]string{
		"Claude Code setup":               "claude-code-setup",
		"Write Plan: explicit invocation": "write-plan-explicit-invocation",
		"`code` and *emphasis*":           "code-and-emphasis",
		"Ünïcode — dashes":                "ünïcode--dashes",
		"Project memory (AGENTS.md)":      "project-memory-agentsmd",
		"Link [inside](b.md) heading":     "link-inside-heading",
	} {
		f := newFixture(t)
		f.write("docs/a.md", "# "+heading+"\n")
		f.write("docs/b.md", "[x](a.md#"+want+")\n")
		if r, _ := f.links(LinkReport{}); r.status != 0 {
			t.Errorf("heading %q: want slug %q\n%s", heading, want, r.out)
		}
	}
}
