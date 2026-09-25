package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinksReportAllErrorsAndSuggestMovedPlan(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "completed", "1", 1)
	f.write("docs/guide.md", "# Guide\n\n[Plan](plans/PLAN-000001-example.md)\n\n![Image](missing.png)\n")
	report := filepath.Join(f.root, "report")
	r, _ := f.links(LinkReport{ReportDir: report})
	expectStatus(t, r, 1)
	errors := readLinkReport(t, report)
	if len(errors) != 2 {
		t.Fatalf("want 2 errors, got %v", errors)
	}
	for _, e := range errors {
		if strings.Contains(e.Destination, "PLAN-") {
			if e.Line != 3 || e.Suggestion != "plans/records/PLAN-000001-example.md" {
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

func TestLinksCheckSkillsSharedReferencesAndClientFiles(t *testing.T) {
	f := newFixture(t)
	os.MkdirAll(f.path("docs"), 0o755)
	sources := []string{
		".agents/skills/implement-plan/SKILL.md",
		".agents/shared/references/example-format.md",
		".claude/commands/implement-plan.md",
		".claude/skills/write-plan/SKILL.md",
		".opencode/commands/write-plan.md",
	}
	for _, source := range sources {
		f.write(source, "# Source\n\n[Missing](missing.md)\n")
	}
	report := filepath.Join(f.root, "report")
	r, _ := f.links(LinkReport{ReportDir: report})
	expectStatus(t, r, 1)
	found := map[string]bool{}
	for _, e := range readLinkReport(t, report) {
		found[e.Source] = true
	}
	for _, source := range sources {
		if !found[source] {
			t.Errorf("source not scanned: %s", source)
		}
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
