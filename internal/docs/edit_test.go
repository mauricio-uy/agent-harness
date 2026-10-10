package docs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var today = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

// editor installs the skills' templates from the payload, as init would.
func (f *fixture) editor() Editor {
	f.t.Helper()
	for _, rel := range templates {
		raw, err := os.ReadFile(filepath.Join("..", "..", "template", filepath.FromSlash(rel)))
		if err != nil {
			f.t.Fatal(err)
		}
		f.write(rel, string(raw))
	}
	return Editor{Root: f.root, Today: today}
}

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *QueryError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want error %s, got %v", code, err)
	}
}

func mustEdit(t *testing.T) func(map[string]any, error) map[string]any {
	return func(data map[string]any, err error) map[string]any {
		t.Helper()
		if err != nil {
			var e *QueryError
			if errors.As(err, &e) {
				t.Fatalf("%s: %s %v", e.Code, e.Message, e.Issues)
			}
			t.Fatal(err)
		}
		return data
	}
}

func TestCreateFillsTheTemplateAndPassesValidation(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	must := mustEdit(t)
	for kind := range templates {
		must(e.Create(NewDocument{Type: kind, Title: "Migración: cache layer"}))
	}
	plan := must(e.Create(NewDocument{Type: "PLAN", Title: "Second plan", Related: []string{"ADR-000001"}}))
	if plan["id"] != "PLAN-000002" || plan["path"] != "docs/plans/PLAN-000002-second-plan.md" || plan["created"] != "2026-10-07" {
		t.Fatalf("%v", plan)
	}
	if body := strings.ReplaceAll(f.read("docs/plans/PLAN-000002-second-plan.md"), "\r\n", "\n"); !strings.Contains(body, "\n# Second plan\n") || !strings.Contains(body, "related: [ADR-000001]") {
		t.Fatal(body)
	}
	if !f.exists("docs/decisions/ADR-000001-migracion-cache-layer.md") {
		t.Fatal("accents fold into the slug")
	}
	for _, name := range Suites {
		expectStatus(t, f.inspect(name), 0)
	}
	_, err := e.Create(NewDocument{Type: "plan", Title: "Bad", Related: []string{"ADR-000009"}})
	expectCode(t, err, "invalid_arguments")
	_, err = e.Create(NewDocument{Type: "adr", Title: "Bad", Supersedes: []string{"PLAN-000001"}})
	expectCode(t, err, "invalid_arguments")
	_, err = e.Create(NewDocument{Type: "plan", Title: "Bad", Slug: "Not A Slug"})
	expectCode(t, err, "invalid_arguments")
}

func TestCreateNeverHandsOutTheSameIDTwice(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.Create(NewDocument{Type: "research", Title: "Concurrent"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(f.path("docs/research"))
	if len(entries) != 8 || f.exists(".agents/.docs-new.lock") {
		t.Fatalf("%d documents", len(entries))
	}
	expectStatus(t, f.inspect("research"), 0)
}

func TestLifecycleCommandsFollowTheApprovalRules(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	must := mustEdit(t)
	must(e.Create(NewDocument{Type: "plan", Title: "Work"}))
	_, err := e.SetStatus("PLAN-000001", "in-progress")
	expectCode(t, err, "approval_required")
	_, err = e.SetStatus("PLAN-000001", "accepted")
	expectCode(t, err, "invalid_arguments")
	must(e.SetStatus("PLAN-000001", "awaiting-approval"))
	plan := must(e.Approve("PLAN-000001"))
	if plan["status"] != "approved" || plan["approval"].(map[string]any)["revision"] != int64(1) {
		t.Fatalf("%v", plan)
	}
	must(e.SetStatus("PLAN-000001", "in-progress"))
	_, err = e.Approve("PLAN-000001")
	expectCode(t, err, "invalid_state")
	plan = must(e.Revise("PLAN-000001"))
	if plan["status"] != "draft" || plan["revision"] != int64(2) || plan["approval"].(map[string]any)["revision"] != int64(1) {
		t.Fatalf("revising keeps the earlier approval as history: %v", plan)
	}
	if approvalStatus(plan) != "stale" {
		t.Fatal("the earlier approval no longer covers the new revision")
	}
	must(e.Approve("PLAN-000001"))
	plan = must(e.Revoke("PLAN-000001"))
	if plan["status"] != "awaiting-approval" || plan["approval"].(map[string]any)["revision"] != nil {
		t.Fatalf("%v", plan)
	}
	expectStatus(t, f.inspect("plans"), 0)
}

func TestSupersedingRequiresAnApprovedReplacement(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	must := mustEdit(t)
	must(e.Create(NewDocument{Type: "adr", Title: "Old"}))
	must(e.Approve("ADR-000001"))
	must(e.Create(NewDocument{Type: "adr", Title: "New", Supersedes: []string{"ADR-000001"}}))
	_, err := e.SetStatus("ADR-000001", "superseded")
	expectCode(t, err, "approval_required")
	must(e.Approve("ADR-000002"))
	must(e.SetStatus("ADR-000001", "superseded"))
	_, err = e.Revise("ADR-000001")
	expectCode(t, err, "invalid_state")
	g, err := RelationGraph(f.root, "ADR-000001")
	if err != nil || len(g.SupersededBy) != 1 || g.SupersededBy[0].ID != "ADR-000002" || g.Status != "superseded" {
		t.Fatalf("%+v %v", g, err)
	}
	expectStatus(t, f.inspect("specifications"), 0)
}

func TestRelateAndGraphShowBothDirections(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	must := mustEdit(t)
	must(e.Create(NewDocument{Type: "functional-requirement", Title: "Export"}))
	must(e.Create(NewDocument{Type: "plan", Title: "Build export"}))
	must(e.Relate("PLAN-000001", "related", []string{"FR-000001"}, false))
	_, err := e.Relate("PLAN-000001", "related", []string{"FR-000001"}, false)
	expectCode(t, err, "invalid_arguments")
	_, err = e.Relate("PLAN-000001", "related", []string{"PLAN-000001"}, false)
	expectCode(t, err, "invalid_arguments")
	g, err := RelationGraph(f.root, "FR-000001")
	if err != nil || len(g.ReferencedBy) != 1 || g.ReferencedBy[0].ID != "PLAN-000001" || len(g.Related) != 0 {
		t.Fatalf("%+v %v", g, err)
	}
	plan := must(e.Relate("PLAN-000001", "related", []string{"FR-000001"}, true))
	if len(plan["related"].([]any)) != 0 {
		t.Fatalf("%v", plan)
	}
	_, err = RelationGraph(f.root, "FR-000009")
	expectCode(t, err, "not_found")
}

func TestRunbookValidationAndOwner(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	must := mustEdit(t)
	must(e.Create(NewDocument{Type: "runbook", Title: "Restore backups"}))
	_, err := e.RecordValidation("RUN-000001", "passed", "")
	expectCode(t, err, "invalid_arguments")
	_, err = e.RecordValidation("RUN-000001", "stale", "")
	expectCode(t, err, "invalid_state")
	must(e.SetField("RUN-000001", "owner", "Platform team"))
	runbook := must(e.RecordValidation("RUN-000001", "passed", "staging"))
	validation := runbook["validation"].(map[string]any)
	if validation["revision"] != int64(1) || validation["date"] != "2026-10-07" || validation["environment"] != "staging" {
		t.Fatalf("%v", runbook)
	}
	runbook = must(e.Revise("RUN-000001"))
	if runbook["validation"].(map[string]any)["status"] != "stale" {
		t.Fatalf("a new revision makes earlier evidence stale: %v", runbook)
	}
	_, err = e.SetField("RUN-000001", "status", "approved")
	expectCode(t, err, "invalid_arguments")
	_, err = e.RecordValidation("PLAN-000001", "passed", "x")
	expectCode(t, err, "not_found")
	expectStatus(t, f.inspect("runbooks"), 0)
}

func TestEditsPreserveFormattingAndRefuseInvalidResults(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	content := "---\r\n# Kept comment\r\nid: RES-000001\r\ntype: research\r\ntitle: \"Quoted: title\"\r\nstatus: draft\r\ncreated: 2026-09-20\r\nupdated: 2026-09-20\r\nrevision: 1\r\napproval: {revision: null, date: null} # inline\r\nrelated: []\r\nextra: kept\r\n---\r\n\r\n# Body\r\n\r\n---\r\n\r\nText after a rule.\r\n"
	f.write("docs/research/RES-000001-quoted.md", content)
	mustEdit(t)(e.Touch("RES-000001"))
	got := f.read("docs/research/RES-000001-quoted.md")
	want := strings.Replace(content, "updated: 2026-09-20", "updated: 2026-10-07", 1)
	if got != want {
		t.Fatalf("only updated may change:\n%q\n%q", got, want)
	}
	// A document created after today cannot be edited today.
	_, err := Editor{Root: f.root, Today: today.AddDate(-1, 0, 0)}.Touch("RES-000001")
	expectCode(t, err, "invalid_date")
	if f.read("docs/research/RES-000001-quoted.md") != want {
		t.Fatal("a refused change must not write")
	}
}

func TestEditsDoNotAddValidationErrors(t *testing.T) {
	f := newFixture(t)
	e := f.editor()
	mustEdit(t)(e.Create(NewDocument{Type: "use-case", Title: "Sign in"}))
	rel := "docs/use-cases/UC-000001-sign-in.md"
	// An existing problem does not block an unrelated change.
	f.write(rel, strings.Replace(f.read(rel), "related: []", "related: [FR-999999]", 1))
	mustEdit(t)(e.SetField("UC-000001", "title", "Sign in again"))
	before := f.read(rel)
	_, err := e.update("UC-000001", func(d *editable, _ map[string]any, _ DocType) error {
		d.set("status", stringNode("bogus"))
		return nil
	})
	expectCode(t, err, "invalid_change")
	if f.read(rel) != before {
		t.Fatal("an invalid change must be rolled back")
	}
}
