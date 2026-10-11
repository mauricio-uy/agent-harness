package docs

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func mustList(t *testing.T, f *fixture, o QueryOptions) ListResult {
	t.Helper()
	r, err := List(f.root, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func idsOf(r ListResult) string {
	var ids []string
	for _, d := range r.Documents {
		ids = append(ids, d["id"].(string))
	}
	return strings.Join(ids, ",")
}

func TestQueriesCombineFiltersAndPreserveHistory(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1, field{"title", "Storage choice"}, field{"related", []string{"PLAN-000001"}})
	f.document("adr", 2, field{"status", "rejected"}, field{"supersedes", []string{"ADR-000001"}})
	path := f.document("research", 1, field{"title", "Different"}, field{"related", []string{"PLAN-000001"}})
	raw := rawBytes(t, path)
	os.WriteFile(path, append(raw, []byte("\nFinding: SQLITE needs no service.\n")...), 0o644)
	f.plan(1, "in-progress", "1", 1)
	o := DefaultQuery()
	if r := mustList(t, f, o); r.Total != 4 {
		t.Fatalf("historical documents must remain discoverable: %+v", r)
	}
	o.Type = "adr,research"
	o.Status = "draft,rejected"
	o.RelatedTo = "PLAN-000001"
	o.Query = "sqlite"
	if got := idsOf(mustList(t, f, o)); got != "RES-000001" {
		t.Fatal(got)
	}
	// Every word must appear, in any order, across title and body.
	for query, want := range map[string]string{"service  sqlite": "RES-000001", "different needs": "RES-000001", "sqlite storage": ""} {
		o.Query = query
		if got := idsOf(mustList(t, f, o)); got != want {
			t.Fatalf("%q: %q", query, got)
		}
	}
	o.Query = ""
	o.RelatedTo = "ADR-000001"
	if r := mustList(t, f, o); r.Total != 0 {
		t.Fatal("supersedes must not match related_to")
	}
}

func TestQueriesPaginationAndStableSorting(t *testing.T) {
	f := newFixture(t)
	for _, n := range []int{3, 1, 2} {
		f.document("adr", n, field{"title", "Same"})
	}
	o := DefaultQuery()
	o.Sort = "title"
	o.Limit = 2
	r := mustList(t, f, o)
	if r.Total != 3 || !r.HasMore || idsOf(r) != "ADR-000001,ADR-000002" {
		t.Fatalf("%+v", r)
	}
	o.Offset = 2
	r = mustList(t, f, o)
	if r.HasMore || idsOf(r) != "ADR-000003" {
		t.Fatalf("%+v", r)
	}
	o.Offset = 100
	r = mustList(t, f, o)
	if r.Documents == nil || len(r.Documents) != 0 || r.Total != 3 {
		t.Fatalf("%+v", r)
	}
	o.Offset = 0
	o.Order = "desc"
	if got := idsOf(mustList(t, f, o)); got != "ADR-000003,ADR-000002" {
		t.Fatal(got)
	}
}

func TestQueriesDatesProjectionAndGetBody(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	o := DefaultQuery()
	o.UpdatedSince = "2026-09-20"
	o.UpdatedUntil = "2026-09-20"
	o.Fields = "id,path,approval.revision,approval.date,validation.status"
	r := mustList(t, f, o)
	if r.Total != 1 || r.Documents[0]["approval.date"] != "2026-09-20" || r.Documents[0]["validation.status"] != nil {
		t.Fatalf("%+v", r)
	}
	data, err := Get(f.root, "ADR-000001", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := data["body"]; ok {
		t.Fatal("body must be opt-in")
	}
	if data["created"] != "2026-09-20" || data["path"] != "docs/decisions/ADR-000001-example.md" {
		t.Fatal(data)
	}
	data, err = Get(f.root, "ADR-000001", true, false)
	if err != nil || !strings.Contains(data["body"].(string), "# Example") {
		t.Fatalf("%v %v", data, err)
	}
	raw, err := json.Marshal(data)
	if err != nil || strings.Contains(string(raw), "T00:00") {
		t.Fatal("calendar dates must retain their representation")
	}
}

func TestQueriesExposeStaleAndMissingApproval(t *testing.T) {
	f := newFixture(t)
	f.plan(1, "approved", "1", 2)
	f.plan(2, "draft", "null", 1)
	f.plan(3, "completed", "1", 1)
	for _, c := range []struct{ approval, id string }{{"stale", "PLAN-000001"}, {"missing", "PLAN-000002"}, {"current", "PLAN-000003"}} {
		o := DefaultQuery()
		o.Approval = c.approval
		if got := idsOf(mustList(t, f, o)); got != c.id {
			t.Fatal(got)
		}
	}
	if len(Plans.Inspect(f.root).Errors) == 0 {
		t.Fatal("check still enforces current approval in approved states")
	}
}

func TestQueriesRejectMalformedApprovalWithoutHidingStaleApproval(t *testing.T) {
	for _, value := range []any{
		map[string]any{"revision": 1},
		approval(1, nil),
		approval(nil, "2026-09-20"),
		approval(1, "2026-02-30"),
		approval(2, "2026-09-20"),
		approval(1, "2026-09-19"),
	} {
		f := newFixture(t)
		f.document("adr", 1, field{"approval", value})
		o := DefaultQuery()
		o.Approval = "current"
		if r := mustList(t, f, o); r.Total != 0 || len(r.Issues) == 0 {
			t.Fatalf("malformed approval must be reported, not listed: %#v %+v", value, r)
		}
		if _, err := Get(f.root, "ADR-000001", false, false); err.(*QueryError).Code != "invalid_documents" {
			t.Fatalf("get must also reject malformed approval: %#v", value)
		}
	}
}

func TestQueriesRejectBadOptionsAndDoNotHideBrokenDocuments(t *testing.T) {
	f := newFixture(t)
	for _, mutate := range []func(*QueryOptions){func(o *QueryOptions) { o.Type = "unknown" }, func(o *QueryOptions) { o.Status = "accepted" }, func(o *QueryOptions) { o.Fields = "body" }, func(o *QueryOptions) { o.Sort = "revision" }, func(o *QueryOptions) { o.Order = "random" }, func(o *QueryOptions) { o.Limit = 0 }, func(o *QueryOptions) { o.Limit = 1001 }, func(o *QueryOptions) { o.Offset = -1 }, func(o *QueryOptions) { o.UpdatedSince = "2026-02-30" }, func(o *QueryOptions) { o.UpdatedSince = "2026-10-01"; o.UpdatedUntil = "2026-09-01" }, func(o *QueryOptions) { o.Approval = "any" }, func(o *QueryOptions) { o.RelatedTo = "not-an-id" }} {
		o := DefaultQuery()
		mutate(&o)
		if _, err := List(f.root, o); err == nil {
			t.Fatal("bad options must fail", o)
		}
	}
	f.document("adr", 1)
	f.write("docs/decisions/ADR-000002-broken.md", "broken")
	o := DefaultQuery()
	o.Query = "matches nothing"
	if r := mustList(t, f, o); r.Total != 0 || len(r.Issues) != 1 || r.Issues[0].File != "docs/decisions/ADR-000002-broken.md" {
		t.Fatal("filters must not conceal an unreadable document", r)
	}
	o.Query = ""
	if r := mustList(t, f, o); idsOf(r) != "ADR-000001" {
		t.Fatal("valid documents stay discoverable next to a broken one", r)
	}
	if _, err := Get(f.root, "ADR-000001", false, false); err != nil {
		t.Fatal(err)
	}
	f.document("research", 1)
	o.Type = "research"
	o.Query = ""
	if r := mustList(t, f, o); r.Total != 1 {
		t.Fatal(r)
	}
}

func TestGetDistinguishesNotFoundInvalidIDAndDuplicates(t *testing.T) {
	f := newFixture(t)
	for _, c := range []struct{ id, code string }{{"ADR-000001", "not_found"}, {"../overview.md", "invalid_arguments"}} {
		_, err := Get(f.root, c.id, true, false)
		e, ok := err.(*QueryError)
		if !ok || e.Code != c.code {
			t.Fatalf("%v", err)
		}
	}
	f.document("adr", 1)
	f.write("docs/decisions/ADR-000001-copy.md", f.read("docs/decisions/ADR-000001-example.md"))
	if _, err := Get(f.root, "ADR-000001", false, false); err == nil {
		t.Fatal("duplicates must fail")
	}
}

func TestGetReportsMetadataThatJSONCannotRepresent(t *testing.T) {
	f := newFixture(t)
	path := f.document("adr", 1)
	raw := string(rawBytes(t, path))
	raw = strings.Replace(raw, "related: []", "related: []\nextra: .nan", 1)
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Get(f.root, "ADR-000001", false, false)
	if e, ok := err.(*QueryError); !ok || e.Code != "invalid_documents" {
		t.Fatal(err)
	}
}

func TestQueriesReadFreshFilesWithoutWritingAndUseTheGitIndex(t *testing.T) {
	f := stagedFixture(t)
	path := f.document("adr", 1)
	f.git("add", ".")
	f.document("adr", 1, field{"title", "Unstaged"}, field{"status", "rejected"})
	before := string(rawBytes(t, path))
	o := DefaultQuery()
	o.Staged = true
	r := mustList(t, f, o)
	if r.Documents[0]["title"] != "Example" {
		t.Fatal(r)
	}
	d, err := Get(f.root, "ADR-000001", true, true)
	if err != nil || d["status"] != "draft" {
		t.Fatalf("%v %v", d, err)
	}
	o.Staged = false
	r = mustList(t, f, o)
	if r.Documents[0]["title"] != "Unstaged" {
		t.Fatal(r)
	}
	if string(rawBytes(t, path)) != before || f.exists("docs/decisions/rejected.md") {
		t.Fatal("queries must not modify project files")
	}
}

func TestQueriesRejectEscapingSymlinks(t *testing.T) {
	f := newFixture(t)
	external := newFixture(t)
	path := external.document("adr", 1)
	os.MkdirAll(f.path("docs/decisions"), 0o755)
	if err := os.Symlink(path, f.path("docs/decisions/ADR-000001-example.md")); err != nil {
		t.Skip(err)
	}
	if r := mustList(t, f, DefaultQuery()); r.Total != 0 || len(r.Issues) == 0 {
		t.Fatal("symlinked document must not be read", r)
	}
}
