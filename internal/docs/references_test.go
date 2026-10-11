package docs

import (
	"strings"
	"testing"
)

// referenceFixture is a repository with one plan and a guide.
func referenceFixture(t *testing.T) *fixture {
	f := stagedFixture(t)
	f.plan(1, "draft", "null", 1)
	f.write("docs/guide.md", "# Guide\n")
	expectStatus(t, f.inspect("plans"), 0)
	f.git("add", ".")
	return f
}

func referenceErrors(t *testing.T, f *fixture, staged bool) []LinkError {
	t.Helper()
	var result CheckResult
	if staged {
		var err error
		if result, err = InspectStaged(f.root); err != nil {
			t.Fatal(err)
		}
	} else {
		result = Inspect(f.root)
	}
	var found []LinkError
	for _, e := range result.Links.Errors {
		if !strings.HasSuffix(e.Source, ".md") {
			found = append(found, e)
		}
	}
	return found
}

func TestCodeReferencesToDocumentsMustResolve(t *testing.T) {
	f := referenceFixture(t)
	f.write("src/app.go", strings.Join([]string{
		"package app",
		"// Implements PLAN-000001; see docs/plans/PLAN-000001-example.md and docs/guide.md.",
		"// Folder: ./docs/plans/ and ../docs/guide.md",
		"// Not references: https://example.com/docs/x, mydocs/y, XPLAN-000001, PLAN-0000012",
		"// Unknown: ADR-000009",
		"// Moved: docs/plans/records/PLAN-000001-example.md",
		"// Missing: docs/nowhere.md.",
	}, "\n")+"\n")
	f.write("assets/logo.bin", "\x00\x01ADR-000009 docs/nowhere.md")
	// Paths into docs/ that are not documents belong to other tools.
	f.write(".gitignore", "docs/_build/\n")
	f.write("package.json", `{"scripts": {"api": "typedoc --out docs/api"}}`+"\n")
	f.git("add", ".")
	errors := referenceErrors(t, f, false)
	want := map[string]LinkError{
		"ADR-000009": {Source: "src/app.go", Line: 5, Reason: "no document has this ID"},
		"docs/plans/records/PLAN-000001-example.md": {Source: "src/app.go", Line: 6, Reason: "missing target", Suggestion: "docs/plans/PLAN-000001-example.md"},
		"docs/nowhere.md": {Source: "src/app.go", Line: 7, Reason: "missing target"},
	}
	if len(errors) != len(want) {
		t.Fatalf("got %d errors, want %d: %+v", len(errors), len(want), errors)
	}
	for _, e := range errors {
		w, ok := want[e.Destination]
		if !ok || e.Source != w.Source || e.Line != w.Line || e.Reason != w.Reason || e.Suggestion != w.Suggestion {
			t.Errorf("unexpected error %+v", e)
		}
	}
}

func TestCodeReferenceToADuplicatedIDFails(t *testing.T) {
	f := referenceFixture(t)
	f.write("docs/plans/PLAN-000001-copy.md", f.read("docs/plans/PLAN-000001-example.md"))
	f.write("src/app.go", "// PLAN-000001\n")
	f.git("add", ".")
	errors := referenceErrors(t, f, false)
	if len(errors) != 1 || errors[0].Reason != "several documents have this ID" {
		t.Fatalf("%+v", errors)
	}
}

func TestStagedCodeReferencesUseTheIndex(t *testing.T) {
	f := referenceFixture(t)
	f.write("src/app.go", "// ADR-000009\n")
	f.git("add", ".")
	f.write("src/app.go", "// PLAN-000001\n")
	f.write("src/untracked.go", "// ADR-000008\n")
	if errors := referenceErrors(t, f, true); len(errors) != 1 || errors[0].Destination != "ADR-000009" {
		t.Fatalf("the staged content is checked: %+v", errors)
	}
	if errors := referenceErrors(t, f, false); len(errors) != 0 {
		t.Fatalf("the working tree is fixed and untracked files are ignored: %+v", errors)
	}
}

func TestIgnoreFileExcludesFilesFromReferenceChecks(t *testing.T) {
	f := referenceFixture(t)
	f.write(ignoreFile, "tests/\n")
	f.write("tests/parse_test.go", "// parse(\"ADR-000123\")\n")
	f.write("src/app.go", "// ADR-000124\n")
	f.git("add", ".")
	for _, staged := range []bool{false, true} {
		errors := referenceErrors(t, f, staged)
		if len(errors) != 1 || errors[0].Destination != "ADR-000124" {
			t.Errorf("staged=%v: only files the ignore file does not list are checked: %+v", staged, errors)
		}
	}
}

func TestReferenceChecksIgnoreDocumentPathsInsideExternalURLs(t *testing.T) {
	f := referenceFixture(t)
	f.write("src/urls.go", strings.Join([]string{
		`// https://example.org/docs/decisions/ADR-000999-external.md`,
		`// https://example.org/?doc=docs/decisions/accepted.md`,
		`// https://example.org/#docs/decisions/accepted.md`,
		`// https://example.org/(docs/decisions/accepted.md)`,
		`// URL=https://example.org/?doc=docs/decisions/accepted.md`,
		`// //example.org/docs/decisions/ADR-000998-external.md`,
		`// Local: ./docs/missing.md`,
	}, "\n"))
	f.git("add", ".")
	for _, staged := range []bool{false, true} {
		errors := referenceErrors(t, f, staged)
		if len(errors) != 1 || errors[0].Destination != "./docs/missing.md" {
			t.Fatalf("staged=%v: external URLs must not be project references: %+v", staged, errors)
		}
	}
}
