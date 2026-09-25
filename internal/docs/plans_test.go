package docs

import (
	"os"
	"strings"
	"testing"
)

func (f *fixture) plan(n int, status, approvalRevision string, revision int, overrides ...field) string {
	f.t.Helper()
	approved := approval(nil, nil)
	if approvalRevision != "null" {
		approved = approval(int(approvalRevision[0]-'0'), "2026-09-20")
	}
	id := "PLAN-" + number(n)
	fields := with([]field{
		{"id", id}, {"type", "plan"}, {"title", "Example"}, {"status", status},
		{"created", "2026-09-20"}, {"updated", "2026-09-20"}, {"revision", revision},
		{"approval", approved}, {"related", []string{}},
	}, overrides...)
	return f.write("docs/plans/records/"+id+"-example.md", frontmatter(fields)+"\n[Reference](../../guide.md)\n")
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
	f.plan(2, "draft", "null", 1, field{"updated", "2026-01-01"})
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "approved requires approval of the current revision", "updated must not precede created")
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

func TestPlanFilenameMustMatchItsID(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	os.Remove(path)
	f.write("docs/plans/records/PLAN-000002-wrong.md", string(content))
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "filename")
}

func TestPlanUsesTheSharedDocumentFields(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	stripped := strings.Replace(strings.Replace(string(content), "type: plan\n", "", 1), "related: []\n", "", 1)
	os.WriteFile(path, []byte(stripped), 0o644)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "missing field: type", "missing field: related")
}

func TestLegacySupersededByIsExplained(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "draft", "null", 1, field{"superseded_by", "PLAN-000002"})
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "superseded_by is no longer used")
}

func TestSupersededPlanRequiresAnApprovedSuccessor(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "superseded", "1", 1)
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "PLAN-000001-example.md: superseded document requires an approved replacement")

	f.plan(2, "draft", "null", 1, field{"supersedes", []string{"PLAN-000001"}})
	expectStatus(t, f.sync("plans", true, false), 1)

	f.plan(2, "in-progress", "1", 1, field{"supersedes", []string{"PLAN-000001"}})
	expectStatus(t, f.sync("plans", true, false), 0)
	if !strings.Contains(f.index("plans", "superseded"), "PLAN-000001") {
		t.Fatal("the replaced plan must be listed as superseded")
	}
}

func TestPlanCannotSupersedeAnotherType(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1)
	f.plan(1, "draft", "null", 1, field{"supersedes", []string{"ADR-000001"}})
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "supersedes must reference the same type")
}

func TestPlansAndOtherDocumentsRelateByID(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "draft", "null", 1, field{"related", []string{"ADR-000001"}})
	f.document("adr", 1, field{"related", []string{"PLAN-000001"}})
	expectStatus(t, f.sync("plans", true, false), 0)
	expectStatus(t, f.sync("specifications", true, false), 0)

	f.plan(1, "draft", "null", 1, field{"related", []string{"ADR-000002"}})
	r := f.sync("plans", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "related must resolve to one document: ADR-000002")
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
	suite := &Suite{Name: "notes", Types: []DocType{{"note", "RES", "notes", "Notes", nil, nil}},
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
