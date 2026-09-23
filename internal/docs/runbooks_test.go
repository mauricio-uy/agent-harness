package docs

import (
	"os"
	"slices"
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

// tableRow returns the rendered text of the index row for identifier.
func tableRow(t *testing.T, index, identifier string) []string {
	t.Helper()
	for _, line := range strings.Split(index, "\n") {
		if !strings.HasPrefix(line, "| "+identifier+" |") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(line, "|"), " | ") {
			cells = append(cells, unescapePunctuation(strings.TrimSpace(c)))
		}
		return cells
	}
	t.Fatalf("missing index row %s in:\n%s", identifier, index)
	return nil
}

func TestRunbookPreviewApplyCheckAndRetirement(t *testing.T) {
	f := newFixture(t)
	path := f.runbook(1)
	f.write("docs/research/README.md", "Keep this index\n")
	expectStatus(t, f.sync("runbooks", false, false), 0)
	if f.exists("docs/runbooks/README.md") {
		t.Fatal("preview must not write")
	}
	expectStatus(t, f.sync("runbooks", false, true), 1)
	original, _ := os.ReadFile(path)
	expectStatus(t, f.sync("runbooks", true, false), 0)
	index := f.read("docs/runbooks/README.md")
	if !strings.Contains(index, "| Validation | Validated on |") {
		t.Fatal("validation columns missing")
	}
	if got := tableRow(t, index, "RUN-000001")[3:]; !slices.Equal(got, []string{"not-validated", "—", day}) {
		t.Fatalf("row %v", got)
	}
	if !strings.Contains(section(index, "Draft"), "RUN-000001") {
		t.Fatal("draft row missing")
	}
	expectStatus(t, f.sync("runbooks", false, true), 0)
	f.runbook(1, field{"status", "retired"}, field{"owner", "Operations"})
	expectStatus(t, f.sync("runbooks", false, true), 1)
	expectStatus(t, f.sync("runbooks", true, false), 0)
	if !strings.Contains(section(f.read("docs/runbooks/README.md"), "Retired"), "RUN-000001") {
		t.Fatal("retired row missing")
	}
	if f.read("docs/research/README.md") != "Keep this index\n" || f.exists("docs/decisions") {
		t.Fatal("runbook sync must not touch other indexes")
	}
	_ = original
}

func TestRunbookApprovalDoesNotRequireOrGrantValidation(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"status", "approved"}, field{"owner", "Operations"}, field{"approval", approval(1, day)})
	expectStatus(t, f.sync("runbooks", true, false), 0)
	if got := tableRow(t, f.read("docs/runbooks/README.md"), "RUN-000001")[3]; got != "not-validated" {
		t.Fatalf("validation %q", got)
	}
	f.runbook(1, field{"validation", validation("passed", 1, day, "staging")})
	expectStatus(t, f.sync("runbooks", true, false), 0)
	if got := tableRow(t, f.read("docs/runbooks/README.md"), "RUN-000001")[3:]; !slices.Equal(got, []string{"passed", day, day}) {
		t.Fatalf("row %v", got)
	}
}

func TestRunbookInvalidOwnerOrApprovalPreventsWrites(t *testing.T) {
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
		r := f.sync("runbooks", true, false)
		expectStatus(t, r, 1)
		r.contains(t, c.expected)
		if f.exists("docs/runbooks/README.md") {
			t.Fatal("validation errors must prevent writes")
		}
	}
}

func TestRunbookInvalidValidationMetadataPreventsWrites(t *testing.T) {
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
		r := f.sync("runbooks", true, false)
		if r.status != 1 || !strings.Contains(r.out, "validation") {
			t.Errorf("validation %v: status %d\n%s", v, r.status, r.out)
		}
		if f.exists("docs/runbooks/README.md") {
			t.Fatal("validation errors must prevent writes")
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
		expectStatus(t, f.sync("runbooks", true, false), 0)
		if after, _ := os.ReadFile(path); string(after) != string(original) {
			t.Fatal("record must not change")
		}
		if got := tableRow(t, f.read("docs/runbooks/README.md"), "RUN-000001")[3:]; !slices.Equal(got, []string{c.state, day, day}) {
			t.Fatalf("%s: row %v", c.state, got)
		}
	}
}

func TestRunbookInvalidIdentityAndDuplicateIDsLeaveIndexUnchanged(t *testing.T) {
	f := newFixture(t)
	path := f.runbook(1)
	expectStatus(t, f.sync("runbooks", true, false), 0)
	original := f.read("docs/runbooks/README.md")
	raw, _ := os.ReadFile(path)
	f.write("docs/runbooks/RUN-000001-copy.md", string(raw))
	f.runbook(2, field{"type", "research"}, field{"related", []string{"RES-999999"}})
	r := f.sync("runbooks", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate ID", "type must", "RES-999999")
	if f.read("docs/runbooks/README.md") != original {
		t.Fatal("index must not change")
	}
}

func TestRunbookReplacementRequiresApprovedSuccessorAndPreservesRetiredHistory(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"status", "superseded"}, field{"owner", "Ops"}, field{"approval", approval(1, day)})
	f.runbook(2, field{"supersedes", []string{"RUN-000001"}})
	r := f.sync("runbooks", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "approved replacement")
	f.runbook(2, field{"status", "approved"}, field{"owner", "Ops"}, field{"approval", approval(1, day)}, field{"supersedes", []string{"RUN-000001"}})
	expectStatus(t, f.sync("runbooks", true, false), 0)
	f.runbook(2, field{"status", "retired"}, field{"owner", "Ops"}, field{"approval", approval(1, day)}, field{"supersedes", []string{"RUN-000001"}})
	expectStatus(t, f.sync("runbooks", true, false), 0)
	if !strings.Contains(section(f.read("docs/runbooks/README.md"), "Superseded"), "RUN-000001") {
		t.Fatal("superseded row missing")
	}
	f.runbook(1, field{"supersedes", []string{"RUN-000002"}})
	r = f.sync("runbooks", false, true)
	expectStatus(t, r, 1)
	r.contains(t, "cycle")
}

func TestRunbookCrossDocumentRelationsDoNotExpandOwnership(t *testing.T) {
	f := newFixture(t)
	f.runbook(1, field{"related", []string{"RES-000001", "ADR-000001", "PLAN-000001"}})
	for rel, id := range map[string]string{
		"research/RES-000001-example.md": "RES-000001", "decisions/ADR-000001-example.md": "ADR-000001",
		"plans/draft/PLAN-000001-example.md": "PLAN-000001"} {
		f.write("docs/"+rel, "---\nid: "+id+"\nstatus: invalid\n---\n")
	}
	expectStatus(t, f.sync("runbooks", true, false), 0)
	for _, c := range []struct{ suite, kind string }{{"research", "research"}, {"specifications", "adr"}} {
		f.document(c.kind, 1, field{"related", []string{"RUN-000001"}})
		f.runbook(1, field{"status", "invalid"})
		before := f.read("docs/runbooks/README.md")
		expectStatus(t, f.sync(c.suite, true, false), 0)
		if f.read("docs/runbooks/README.md") != before {
			t.Fatalf("%s sync must not touch the runbook index", c.suite)
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
