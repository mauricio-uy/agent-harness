package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type specType struct{ prefix, folder string }

var specTypes = map[string]specType{
	"adr":                        {"ADR", "decisions"},
	"use-case":                   {"UC", "use-cases"},
	"functional-requirement":     {"FR", "requirements/functional"},
	"non-functional-requirement": {"NFR", "requirements/non-functional"},
	"research":                   {"RES", "research"},
}

var specKinds = []string{"adr", "use-case", "functional-requirement", "non-functional-requirement"}

func (f *fixture) document(kind string, n int, overrides ...field) string {
	f.t.Helper()
	spec := specTypes[kind]
	id := spec.prefix + "-" + number(n)
	fields := with([]field{
		{"id", id}, {"type", kind}, {"title", "Example"}, {"status", "draft"},
		{"created", "2026-09-20"}, {"updated", "2026-09-20"}, {"revision", 1},
		{"approval", approval(nil, nil)}, {"related", []string{}},
	}, overrides...)
	return f.write("docs/"+spec.folder+"/records/"+id+"-example.md", frontmatter(fields))
}

// indexes lists generated index files: Markdown outside records/ other than README.md.
func indexes(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() != "README.md" && filepath.Base(filepath.Dir(path)) != RecordsDir {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func TestSpecificationPreviewCheckAndStatusChangeMoveRowsOnly(t *testing.T) {
	f := newFixture(t)
	var paths []string
	for _, kind := range specKinds {
		paths = append(paths, f.document(kind, 1))
	}
	expectStatus(t, f.sync("specifications", false, false), 0)
	if len(indexes(t, f.root)) != 0 {
		t.Fatal("preview must not write indexes")
	}
	expectStatus(t, f.sync("specifications", false, true), 1)
	expectStatus(t, f.sync("specifications", true, false), 0)
	for _, path := range paths {
		index, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(path)), "draft.md"))
		if !strings.Contains(string(index), filepath.Base(path)) {
			t.Fatalf("index lacks %s", path)
		}
	}
	f.document("adr", 1, field{"status", "accepted"}, field{"approval", approval(1, "2026-09-20")})
	expectStatus(t, f.sync("specifications", false, true), 1)
	expectStatus(t, f.sync("specifications", true, false), 0)
	if strings.Contains(f.index("decisions", "draft"), "ADR-000001") || !strings.Contains(f.index("decisions", "accepted"), "ADR-000001") {
		t.Fatal("row must move to the accepted index")
	}
	expectStatus(t, f.sync("specifications", false, true), 0)
}

func TestSpecificationHistoricalDocumentsAreInSeparateTables(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1, field{"status", "superseded"}, field{"approval", approval(1, "2026-09-20")})
	f.document("adr", 2, field{"status", "accepted"}, field{"approval", approval(1, "2026-09-20")}, field{"supersedes", []string{"ADR-000001"}})
	f.document("adr", 3, field{"status", "rejected"})
	expectStatus(t, f.sync("specifications", true, false), 0)
	if !strings.Contains(f.index("decisions", "superseded"), "ADR-000001") || !strings.Contains(f.index("decisions", "rejected"), "ADR-000003") {
		t.Fatal("historical rows misplaced")
	}
}

func TestSpecificationInvalidMetadataAndDuplicateIDsAbortAllWrites(t *testing.T) {
	f := newFixture(t)
	path := f.document("adr", 1, field{"status", "approved"}, field{"approval", approval(true, "2026-09-20")})
	raw, _ := os.ReadFile(path)
	f.write("docs/decisions/records/ADR-000001-copy.md", string(raw))
	f.document("use-case", 1, field{"related", []string{"FR-999999"}})
	r := f.sync("specifications", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "status", "approval", "duplicate", "FR-999999")
	if len(indexes(t, f.root)) != 0 {
		t.Fatal("validation errors must prevent writes")
	}
}

func TestSpecificationReplacementCyclesAndCrossTypeReplacementsFail(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1, field{"supersedes", []string{"ADR-000002"}})
	f.document("adr", 2, field{"supersedes", []string{"ADR-000001"}})
	f.document("use-case", 1, field{"supersedes", []string{"ADR-000001"}})
	r := f.sync("specifications", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "cycle", "same type")
}

func TestSpecificationRelatedCanReferenceAPlanAndOtherTypes(t *testing.T) {
	f := newFixture(t)
	f.document("functional-requirement", 1)
	f.document("adr", 1, field{"related", []string{"FR-000001", "PLAN-000001"}})
	f.write("docs/plans/records/PLAN-000001-example.md", "---\nid: PLAN-000001\n---\n")
	expectStatus(t, f.sync("specifications", true, false), 0)
}

func TestSpecificationDuplicateYAMLKeysAndStaleApprovalFail(t *testing.T) {
	f := newFixture(t)
	path := f.document("adr", 1)
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(raw), "status: draft", "status: draft\nstatus: accepted", 1)), 0o644)
	f.document("functional-requirement", 1, field{"revision", 2}, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	r := f.sync("specifications", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "duplicate YAML", "current revision")
}

func TestLinkSuggestionsRecognizeAllSpecificationIDs(t *testing.T) {
	f := newFixture(t)
	var lines []string
	for kind, spec := range specTypes {
		f.document(kind, 1)
		lines = append(lines, "[Old](old/"+spec.prefix+"-000001-example.md)")
	}
	f.write("docs/guide.md", strings.Join(lines, "\n\n"))
	_, errors := f.links(LinkReport{})
	if len(errors) != len(specTypes) {
		t.Fatalf("want %d errors, got %v", len(specTypes), errors)
	}
	for _, e := range errors {
		if e.Suggestion == "" {
			t.Fatalf("missing suggestion: %+v", e)
		}
	}
}

func TestSpecificationSyncDoesNotManageUnrelatedResearch(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1)
	f.document("research", 1, field{"status", "invalid"})
	f.write("docs/research/draft.md", "Keep research index unchanged")
	expectStatus(t, f.sync("specifications", true, false), 0)
	if f.read("docs/research/draft.md") != "Keep research index unchanged" {
		t.Fatal("research index must be untouched")
	}
	expectStatus(t, f.sync("specifications", false, true), 0)
}

func TestResearchSyncOnlyWritesItsIndexAndIgnoresUnrelatedMetadata(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1)
	f.document("adr", 1, field{"status", "invalid"})
	f.write("docs/decisions/draft.md", "Keep specification index unchanged")
	expectStatus(t, f.sync("research", false, false), 0)
	if f.exists("docs/research/draft.md") {
		t.Fatal("preview must not write")
	}
	expectStatus(t, f.sync("research", true, false), 0)
	if f.read("docs/decisions/draft.md") != "Keep specification index unchanged" || f.exists("docs/use-cases") {
		t.Fatal("research sync must not touch specifications")
	}
	expectStatus(t, f.sync("research", false, true), 0)
}

func TestCrossSkillRelationsVerifyIdentityWithoutOwningTargetLifecycle(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"related", []string{"ADR-000001"}})
	f.document("adr", 1, field{"status", "invalid"})
	expectStatus(t, f.sync("research", true, false), 0)
	f.document("research", 1, field{"status", "invalid"})
	f.document("adr", 1, field{"related", []string{"RES-000001"}})
	expectStatus(t, f.sync("specifications", true, false), 0)
}

func TestResearchApprovalMovesOnlyTheIndexRow(t *testing.T) {
	f := newFixture(t)
	path := f.document("research", 1, field{"related", []string{"ADR-000001"}})
	f.document("adr", 1, field{"related", []string{"RES-000001"}})
	expectStatus(t, f.sync("research", true, false), 0)
	if !strings.Contains(f.index("research", "draft"), "RES-000001") {
		t.Fatal("draft row missing")
	}
	f.document("research", 1, field{"related", []string{"ADR-000001"}}, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	original, _ := os.ReadFile(path)
	expectStatus(t, f.sync("research", false, true), 1)
	expectStatus(t, f.sync("research", true, false), 0)
	if strings.Contains(f.index("research", "draft"), "RES-000001") || !strings.Contains(f.index("research", "approved"), "RES-000001") {
		t.Fatal("row must move to the approved index")
	}
	if after, _ := os.ReadFile(path); string(after) != string(original) {
		t.Fatal("record must not change")
	}
	expectStatus(t, f.sync("research", false, true), 0)
}

func TestResearchRequiresCurrentApprovalAndExistingRelations(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"revision", 2}, field{"status", "approved"},
		field{"approval", approval(1, "2026-09-20")}, field{"related", []string{"FR-999999"}})
	r := f.sync("research", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "current revision", "FR-999999")
	if len(indexes(t, f.root)) != 0 {
		t.Fatal("validation errors must prevent writes")
	}
}

func TestSupersededResearchRequiresAnApprovedResearchSuccessor(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"status", "superseded"}, field{"approval", approval(1, "2026-09-20")})
	f.document("research", 2, field{"supersedes", []string{"RES-000001"}})
	r := f.sync("research", true, false)
	expectStatus(t, r, 1)
	r.contains(t, "approved replacement")
	f.document("research", 2, field{"supersedes", []string{"RES-000001"}}, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	expectStatus(t, f.sync("research", true, false), 0)
	if !strings.Contains(f.index("research", "superseded"), "RES-000001") {
		t.Fatal("superseded row missing")
	}
	f.document("adr", 1)
	f.document("research", 3, field{"supersedes", []string{"ADR-000001"}})
	r = f.sync("research", false, true)
	expectStatus(t, r, 1)
	r.contains(t, "same type")
}
