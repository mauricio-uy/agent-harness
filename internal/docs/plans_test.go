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
	return f.write("docs/plans/"+id+"-example.md", frontmatter(fields)+"\n[Reference](../guide.md)\n")
}

func TestPlanDuplicateIDsAreReported(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "awaiting-approval", "null", 1)
	f.write("docs/plans/PLAN-000001-copy.md", f.read("docs/plans/PLAN-000001-example.md"))
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
}

func TestPlanInvalidApprovalAndDatesAreReportedTogether(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "approved", "1", 2)
	f.plan(2, "draft", "null", 1, field{"updated", "2026-01-01"})
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "approved requires approval of the current revision", "updated must not precede created")
}

func TestPlanDuplicateYAMLKeysAreRejected(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(content), "status: draft", "status: draft\nstatus: approved", 1)), 0o644)
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "duplicate")
}

func TestPlanFilenameMustMatchItsID(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	os.Remove(path)
	f.write("docs/plans/PLAN-000002-wrong.md", string(content))
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "filename")
}

func TestPlanUsesTheSharedDocumentFields(t *testing.T) {
	f := newFixture(t)
	path := f.plan(1, "draft", "null", 1)
	content, _ := os.ReadFile(path)
	stripped := strings.Replace(strings.Replace(string(content), "type: plan\n", "", 1), "related: []\n", "", 1)
	os.WriteFile(path, []byte(stripped), 0o644)
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "missing field: type", "missing field: related")
}

func TestSupersededPlanRequiresAnApprovedSuccessor(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "superseded", "1", 1)
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "PLAN-000001-example.md: superseded document requires an approved replacement")

	f.plan(2, "draft", "null", 1, field{"supersedes", []string{"PLAN-000001"}})
	expectStatus(t, f.inspect("plans"), 1)

	f.plan(2, "in-progress", "1", 1, field{"supersedes", []string{"PLAN-000001"}})
	expectStatus(t, f.inspect("plans"), 0)
}

func TestPlanCannotSupersedeAnotherType(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1)
	f.plan(1, "draft", "null", 1, field{"supersedes", []string{"ADR-000001"}})
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "supersedes must reference the same type")
}

func TestPlansAndOtherDocumentsRelateByID(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "draft", "null", 1, field{"related", []string{"ADR-000001"}})
	f.document("adr", 1, field{"related", []string{"PLAN-000001"}})
	expectStatus(t, f.inspect("plans"), 0)
	expectStatus(t, f.inspect("specifications"), 0)

	f.plan(1, "draft", "null", 1, field{"related", []string{"ADR-000002"}})
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "related must resolve to one document: ADR-000002")
}

func TestNestedDocumentsAreRejected(t *testing.T) {
	f := newFixture(t)
	f.write("docs/plans/records/PLAN-000001-old.md", "---\nid: PLAN-000001\n---\n")
	f.write("docs/plans/draft/PLAN-000002-old.md", "---\nid: PLAN-000002\n---\n")
	r := f.inspect("plans")
	expectStatus(t, r, 1)
	r.contains(t, "documents belong directly in docs/plans/")
}
