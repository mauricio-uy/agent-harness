package docs

import (
	"os"
	"strings"
	"testing"
)

const day = "2026-09-21"

func (f *fixture) runbook(n int, overrides ...field) string {
	f.t.Helper()
	id := "RUN-" + number(n)
	fields := with([]field{
		{"id", id}, {"type", "runbook"}, {"title", "Recover service"}, {"status", "draft"},
		{"created", day}, {"updated", day}, {"revision", 1}, {"owner", nil},
		{"approval", approval(nil, nil)}, {"related", []string{}},
		{"validation", validation("not-validated", nil, nil, nil)},
	}, overrides...)
	return f.write("docs/runbooks/"+id+"-recover.md", frontmatter(fields))
}

func validation(status, revision, date, environment any) map[string]any {
	return map[string]any{"status": status, "revision": revision, "date": date, "environment": environment}
}

func TestRunbookApprovalDoesNotRequireOrGrantValidation(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"status", "approved"}, field{"owner", "Operations"}, field{"approval", approval(1, day)})
	expectStatus(t, f.inspect("runbooks"), 0)
	f.runbook(1, field{"validation", validation("passed", 1, day, "staging")})
	expectStatus(t, f.inspect("runbooks"), 0)
}

func TestRunbookInvalidOwnerOrApprovalIsReported(t *testing.T) {
	cases := []struct {
		fields   []field
		expected string
	}{
		{[]field{{"status", "awaiting-approval"}}, "owner"},
		{[]field{{"owner", []string{}}}, "owner"},
		{[]field{{"owner", "   "}}, "owner"},
		{[]field{{"status", "approved"}, {"owner", "Ops"}}, "current revision"},
		{[]field{{"status", "approved"}, {"owner", "Ops"}, {"revision", 2}, {"approval", approval(1, day)}}, "current revision"},
	}
	for _, c := range cases {
		f := newFixture(t)
		f.runbook(1, c.fields...)
		r := f.inspect("runbooks")
		expectStatus(t, r, 1)
		r.contains(t, c.expected)
	}
}

func TestRunbookInvalidValidationMetadataIsReported(t *testing.T) {
	valid := func(overrides map[string]any) map[string]any {
		v := validation("passed", 1, day, "staging")
		for k, value := range overrides {
			v[k] = value
		}
		return v
	}
	cases := []any{nil, map[string]any{}, valid(map[string]any{"status": "unknown"}), valid(map[string]any{"status": []string{}}),
		valid(map[string]any{"status": "not-validated"}), valid(map[string]any{"revision": true}),
		valid(map[string]any{"revision": 0}), valid(map[string]any{"revision": 3}),
		valid(map[string]any{"date": "2026-02-30"}), valid(map[string]any{"date": "2026-09-20"}),
		valid(map[string]any{"date": "2026-09-22"}), valid(map[string]any{"environment": "  "}),
		valid(map[string]any{"environment": []string{"staging"}}),
		valid(map[string]any{"status": "passed"}), valid(map[string]any{"status": "partial"}), valid(map[string]any{"status": "failed"})}
	for _, v := range cases {
		f := newFixture(t)
		f.runbook(1, field{"revision", 2}, field{"validation", v})
		r := f.inspect("runbooks")
		if r.status != 1 || !strings.Contains(r.out, "validation") {
			t.Errorf("validation %v: status %d\n%s", v, r.status, r.out)
		}
	}
}

func TestRunbookValidOperationalStatesAndStaleHistory(t *testing.T) {
	for _, c := range []struct {
		state          string
		revision, from int
	}{{"partial", 1, 1}, {"passed", 1, 1}, {"failed", 1, 1}, {"stale", 2, 1}, {"stale", 1, 1}} {
		f := newFixture(t)
		path := f.runbook(1, field{"revision", c.revision}, field{"validation", validation(c.state, c.from, day, "staging")})
		original, _ := os.ReadFile(path)
		expectStatus(t, f.inspect("runbooks"), 0)
		if after, _ := os.ReadFile(path); string(after) != string(original) {
			t.Fatal("record must not change")
		}
	}
}

func TestRunbookInvalidIdentityAndDuplicateIDsAreReportedTogether(t *testing.T) {
	f := newFixture(t)
	path := f.runbook(1)
	raw, _ := os.ReadFile(path)
	f.write("docs/runbooks/RUN-000001-copy.md", string(raw))
	f.runbook(2, field{"type", "research"}, field{"related", []string{"RES-999999"}})
	r := f.inspect("runbooks")
	expectStatus(t, r, 1)
	r.contains(t, "duplicate ID", "type must", "RES-999999")
}

func TestRunbookReplacementRequiresApprovedSuccessorAndPreservesRetiredHistory(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"status", "superseded"}, field{"owner", "Ops"}, field{"approval", approval(1, day)})
	f.runbook(2, field{"supersedes", []string{"RUN-000001"}})
	r := f.inspect("runbooks")
	expectStatus(t, r, 1)
	r.contains(t, "approved replacement")
	f.runbook(2, field{"status", "approved"}, field{"owner", "Ops"}, field{"approval", approval(1, day)}, field{"supersedes", []string{"RUN-000001"}})
	expectStatus(t, f.inspect("runbooks"), 0)
	f.runbook(2, field{"status", "retired"}, field{"owner", "Ops"}, field{"approval", approval(1, day)}, field{"supersedes", []string{"RUN-000001"}})
	expectStatus(t, f.inspect("runbooks"), 0)
	f.runbook(1, field{"supersedes", []string{"RUN-000002"}})
	r = f.inspect("runbooks")
	expectStatus(t, r, 1)
	r.contains(t, "cycle")
}

func TestRunbookCrossDocumentRelationsDoNotExpandOwnership(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"related", []string{"RES-000001", "ADR-000001", "PLAN-000001"}})
	for rel, id := range map[string]string{
		"research/RES-000001-example.md": "RES-000001", "decisions/ADR-000001-example.md": "ADR-000001",
		"plans/PLAN-000001-example.md": "PLAN-000001"} {
		f.write("docs/"+rel, "---\nid: "+id+"\nstatus: invalid\n---\n")
	}
	expectStatus(t, f.inspect("runbooks"), 0)
	for _, c := range []struct{ suite, kind string }{{"research", "research"}, {"specifications", "adr"}} {
		f.document(c.kind, 1, field{"related", []string{"RUN-000001"}})
		f.runbook(1, field{"status", "invalid"})
		before := f.read("docs/runbooks/RUN-000001-recover.md")
		expectStatus(t, f.inspect(c.suite), 0)
		if f.read("docs/runbooks/RUN-000001-recover.md") != before {
			t.Fatalf("%s checks must not change the runbook document", c.suite)
		}
	}
}

func TestLinkCheckerSuggestsRunbookPath(t *testing.T) {
	f := newFixture(t)
	f.runbook(1)
	f.write("docs/guide.md", "[Runbook](old/RUN-000001-recover.md#recovery)\n")
	_, errors := f.links(LinkReport{})
	if len(errors) != 1 || errors[0].Suggestion != "runbooks/RUN-000001-recover.md#recovery" {
		t.Fatalf("errors %+v", errors)
	}
}
