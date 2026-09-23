package docs

import (
	"os"
	"strings"
	"testing"
)

func (f *fixture) plan(n int, folder, status, approvalRevision string, revision int) string {
	f.t.Helper()
	date := "null"
	if approvalRevision != "null" {
		date = "2026-09-20"
	}
	return f.write("docs/plans/"+folder+"/PLAN-"+number(n)+"-example.md",
		"---\nid: PLAN-"+number(n)+"\ntitle: Example\nstatus: "+status+"\n"+
			"created: 2026-09-20\nupdated: 2026-09-20\nrevision: "+string(rune('0'+revision))+"\n"+
			"approval:\n  revision: "+approvalRevision+"\n  date: "+date+"\n---\n\n"+
			"# Example\n\n[Reference](../../guide.md)\n")
}

func TestPlanSyncPreviewApplyIdempotenceAndPreservedDocuments(t *testing.T) {
	f := newFixture(t)
	source := f.plan(1, "draft", "awaiting-approval", "null", 1)
	original, _ := os.ReadFile(source)
	f.write("docs/guide.md", "[Plan](plans/draft/PLAN-000001-example.md)\n")
	before := f.read("docs/guide.md")

	expectStatus(t, f.sync("plans", false, false), 0)
	if !exists(source) || f.exists("docs/plans/draft/README.md") {
		t.Fatal("preview must not write")
	}
	expectStatus(t, f.sync("plans", false, true), 1)
	expectStatus(t, f.sync("plans", true, false), 0)
	target := "docs/plans/awaiting-approval/PLAN-000001-example.md"
	if exists(source) || f.read(target) != string(original) {
		t.Fatal("plan must move unchanged")
	}
	if f.read("docs/guide.md") != before {
		t.Fatal("references must not be rewritten")
	}
	if !strings.Contains(f.read("docs/plans/awaiting-approval/README.md"), "PLAN-000001-example.md") {
		t.Fatal("index must list the plan")
	}
	expectStatus(t, f.sync("plans", false, true), 0)
}

func TestPlanDuplicateIDsAbortAllWrites(t *testing.T) {
	f := newFixture(t)
	first := f.plan(1, "draft", "awaiting-approval", "null", 1)
	f.plan(1, "cancelled", "cancelled", "null", 1)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
	if !exists(first) || f.exists("docs/plans/draft/README.md") {
		t.Fatal("validation errors must prevent writes")
	}
}

func TestPlanInvalidApprovalAndDatesAreReportedTogether(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "draft", "approved", "1", 2)
	other := f.plan(2, "draft", "draft", "null", 1)
	content, _ := os.ReadFile(other)
	os.WriteFile(other, []byte(strings.Replace(string(content), "updated: 2026-09-20", "updated: 2026-01-01", 1)), 0o644)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "approval", "updated")
}

func TestPlanDuplicateYAMLKeysAreRejected(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "draft", "null", 1)
	content, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(content), "status: draft", "status: draft\nstatus: approved", 1)), 0o644)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
}

func TestPlanMissingReplacementAndFilenameMismatch(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "superseded", "null", 1)
	content, _ := os.ReadFile(path)
	os.Remove(path)
	f.write("docs/plans/draft/PLAN-000002-wrong.md", strings.Replace(string(content), "title: Example", "superseded_by: PLAN-999999\ntitle: Example", 1))
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "filename", "superseded_by")
}
