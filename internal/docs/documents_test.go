package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

func (f *fixture) document(kind string, n int, overrides ...field) string {
	f.t.Helper()
	spec := specTypes[kind]
	id := spec.prefix + "-" + number(n)
	fields := with([]field{
		{"id", id}, {"type", kind}, {"title", "Example"}, {"status", "draft"},
		{"created", "2026-09-20"}, {"updated", "2026-09-20"}, {"revision", 1},
		{"approval", approval(nil, nil)}, {"related", []string{}},
	}, overrides...)
	return f.write("docs/"+spec.folder+"/"+id+"-example.md", frontmatter(fields))
}

func TestSpecificationInvalidMetadataAndDuplicateIDsAreReportedTogether(t *testing.T) {
	f := newFixture(t)
	path := f.document("adr", 1, field{"status", "invalid"}, field{"approval", approval(true, "2026-09-20")})
	raw, _ := os.ReadFile(path)
	f.write("docs/decisions/ADR-000001-copy.md", string(raw))
	f.document("use-case", 1, field{"related", []string{"FR-999999"}})
	r := f.inspect("specifications")
	expectStatus(t, r, 1)
	r.contains(t, "status", "approval", "duplicate", "FR-999999")
}

func TestSpecificationReplacementCyclesAndCrossTypeReplacementsFail(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1, field{"supersedes", []string{"ADR-000002"}})
	f.document("adr", 2, field{"supersedes", []string{"ADR-000001"}})
	f.document("use-case", 1, field{"supersedes", []string{"ADR-000001"}})
	r := f.inspect("specifications")
	expectStatus(t, r, 1)
	r.contains(t, "cycle", "same type")
}

func TestSpecificationRelatedCanReferenceAPlanAndOtherTypes(t *testing.T) {
	f := newFixture(t)
	f.document("functional-requirement", 1)
	f.document("adr", 1, field{"related", []string{"FR-000001", "PLAN-000001"}})
	f.write("docs/plans/PLAN-000001-example.md", "---\nid: PLAN-000001\n---\n")
	expectStatus(t, f.inspect("specifications"), 0)
}

func TestSpecificationDuplicateYAMLKeysAndStaleApprovalFail(t *testing.T) {
	f := newFixture(t)
	path := f.document("adr", 1)
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(raw), "status: draft", "status: draft\nstatus: accepted", 1)), 0o644)
	f.document("functional-requirement", 1, field{"revision", 2}, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	r := f.inspect("specifications")
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

func TestSpecificationValidationIgnoresUnrelatedResearch(t *testing.T) {
	f := newFixture(t)
	f.document("adr", 1)
	f.document("research", 1, field{"status", "invalid"})
	f.write("docs/research/notes.md", "Unrelated research notes")
	expectStatus(t, f.inspect("specifications"), 0)
}

func TestResearchValidationIgnoresUnrelatedMetadata(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1)
	f.document("adr", 1, field{"status", "invalid"})
	f.write("docs/decisions/notes.md", "Unrelated specification notes")
	expectStatus(t, f.inspect("research"), 0)
}

func TestValidationLeavesAllFilesAndDirectoriesUnchanged(t *testing.T) {
	for _, status := range []string{"draft", "invalid"} {
		for _, name := range Suites {
			t.Run(name+"/"+status, func(t *testing.T) {
				f := newFixture(t)
				f.plan(1, status, "null", 1)
				f.document("adr", 1, field{"status", status})
				f.document("research", 1, field{"status", status})
				f.runbook(1, field{"status", status})
				f.write("docs/notes.md", "Project notes\n")
				snapshot := func() map[string]string {
					files := map[string]string{}
					if err := filepath.WalkDir(f.root, func(path string, entry fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if entry.IsDir() {
							files[path+"/"] = ""
							return nil
						}
						raw, err := os.ReadFile(path)
						files[path] = string(raw)
						return err
					}); err != nil {
						t.Fatal(err)
					}
					return files
				}
				before := snapshot()
				want := 0
				if status == "invalid" {
					want = 1
				}
				expectStatus(t, f.inspect(name), want)
				if !reflect.DeepEqual(before, snapshot()) {
					t.Fatal("validation changed project files or directories")
				}
			})
		}
	}
}

func TestCrossSkillRelationsVerifyIdentityWithoutOwningTargetLifecycle(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"related", []string{"ADR-000001"}})
	f.document("adr", 1, field{"status", "invalid"})
	expectStatus(t, f.inspect("research"), 0)
	f.document("research", 1, field{"status", "invalid"})
	f.document("adr", 1, field{"related", []string{"RES-000001"}})
	expectStatus(t, f.inspect("specifications"), 0)
}

func TestResearchRequiresCurrentApprovalAndExistingRelations(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"revision", 2}, field{"status", "approved"},
		field{"approval", approval(1, "2026-09-20")}, field{"related", []string{"FR-999999"}})
	r := f.inspect("research")
	expectStatus(t, r, 1)
	r.contains(t, "current revision", "FR-999999")
}

func TestSupersededResearchRequiresAnApprovedResearchSuccessor(t *testing.T) {
	f := newFixture(t)
	f.document("research", 1, field{"status", "superseded"}, field{"approval", approval(1, "2026-09-20")})
	f.document("research", 2, field{"supersedes", []string{"RES-000001"}})
	r := f.inspect("research")
	expectStatus(t, r, 1)
	r.contains(t, "approved replacement")
	f.document("research", 2, field{"supersedes", []string{"RES-000001"}}, field{"status", "approved"}, field{"approval", approval(1, "2026-09-20")})
	expectStatus(t, f.inspect("research"), 0)
	f.document("adr", 1)
	f.document("research", 3, field{"supersedes", []string{"ADR-000001"}})
	r = f.inspect("research")
	expectStatus(t, r, 1)
	r.contains(t, "same type")
}
