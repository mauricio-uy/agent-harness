package docs

import (
	"os"
	"strings"
	"testing"
)

func (f *fixture) plan(n int, status, approvalRevision string, revision int) string {
	f.t.Helper()
	date := "null"
	if approvalRevision != "null" {
		date = "2026-09-20"
	}
	return f.write("docs/plans/records/PLAN-"+number(n)+"-example.md",
		"---\nid: PLAN-"+number(n)+"\ntitle: Example\nstatus: "+status+"\n"+
			"created: 2026-09-20\nupdated: 2026-09-20\nrevision: "+string(rune('0'+revision))+"\n"+
			"approval:\n  revision: "+approvalRevision+"\n  date: "+date+"\n---\n\n"+
			"# Example\n\n[Reference](../../guide.md)\n")
}

func TestPlanStatusChangeMovesOnlyTheIndexRow(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "awaiting-approval", "null", 1)
	original := f.read("docs/plans/records/PLAN-000001-example.md")

	expectStatus(t, f.sync("plans", false, false), 0)
	if f.exists("docs/plans/awaiting-approval.md") {
		t.Fatal("preview must not write")
	}
	expectStatus(t, f.sync("plans", false, true), 1)
	expectStatus(t, f.sync("plans", true, false), 0)
	for _, state := range planStates {
		if !f.exists("docs/plans/" + state + ".md") {
			t.Fatalf("every state needs an index, even when empty: %s", state)
		}
	}
	if !strings.Contains(f.read("docs/plans/awaiting-approval.md"), "(records/PLAN-000001-example.md)") {
		t.Fatal("the index must link the plan in records/")
	}
	expectStatus(t, f.sync("plans", false, true), 0)

	f.plan(1, "approved", "1", 1)
	approved := f.read("docs/plans/records/PLAN-000001-example.md")
	expectStatus(t, f.sync("plans", false, true), 1)
	expectStatus(t, f.sync("plans", true, false), 0)
	if !exists(path) || f.read("docs/plans/records/PLAN-000001-example.md") != approved || approved == original {
		t.Fatal("the plan must stay in place, unchanged by synchronization")
	}
	if strings.Contains(f.read("docs/plans/awaiting-approval.md"), "PLAN-000001") ||
		!strings.Contains(f.read("docs/plans/approved.md"), "PLAN-000001") {
		t.Fatal("the row must move to the approved index")
	}
	if !strings.Contains(f.read("docs/plans/completed.md"), "No documents in this state.") {
		t.Fatal("empty indexes say so")
	}
}

func TestPlanDuplicateIDsAbortAllWrites(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "awaiting-approval", "null", 1)
	f.write("docs/plans/records/PLAN-000001-copy.md", f.read("docs/plans/records/PLAN-000001-example.md"))
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
	if f.exists("docs/plans/draft.md") {
		t.Fatal("validation errors must prevent writes")
	}
}

func TestPlanInvalidApprovalAndDatesAreReportedTogether(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "approved", "1", 2)
	other := f.plan(2, "draft", "null", 1)
	content, _ := os.ReadFile(other)
	os.WriteFile(other, []byte(strings.Replace(string(content), "updated: 2026-09-20", "updated: 2026-01-01", 1)), 0o644)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "approval", "updated")
}

func TestPlanDuplicateYAMLKeysAreRejected(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(content), "status: draft", "status: draft\nstatus: approved", 1)), 0o644)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
}

func TestPlanMissingReplacementAndFilenameMismatch(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "superseded", "null", 1)
	content, _ := os.ReadFile(path)
	os.Remove(path)
	f.write("docs/plans/records/PLAN-000002-wrong.md", strings.Replace(string(content), "title: Example", "superseded_by: PLAN-999999\ntitle: Example", 1))
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "filename", "superseded_by")
}

func TestDocumentsOutsideRecordsAreRejected(t *testing.T) {
	f := newFixture(t)
	f.write("docs/plans/README.md", "# Plans\n")
	f.write("docs/plans/PLAN-000001-loose.md", "---\nid: PLAN-000001\n---\n")
	f.write("docs/plans/draft/PLAN-000002-old-layout.md", "---\nid: PLAN-000002\n---\n")
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "docs/plans/PLAN-000001-loose.md: documents belong in docs/plans/records/",
		"docs/plans/draft/PLAN-000002-old-layout.md: documents belong in docs/plans/records/")
}

func TestTypeWithoutStatesKeepsTheSameLayout(t *testing.T) {
	f := newFixture(t)
	suite := &Suite{Name: "notes", Types: []DocType{{"note", "RES", "notes", "Notes", nil, ""}},
		validate: func(record, DocType, *Suite) []string { return nil }}
	f.write("docs/notes/records/RES-000001-a.md", frontmatter([]field{{"id", "RES-000001"}, {"title", "A"}, {"revision", 1}, {"updated", "2026-09-20"}}))
	var out strings.Builder
	if status := suite.Sync(f.root, true, false, &out); status != 0 {
		t.Fatalf("status %d:\n%s", status, out.String())
	}
	if index := f.read("docs/notes/all.md"); !strings.Contains(index, "# Notes: All") || !strings.Contains(index, "RES-000001") {
		t.Fatalf("a type without states gets one all.md index:\n%s", index)
	}
}
