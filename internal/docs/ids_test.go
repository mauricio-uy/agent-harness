package docs

import (
	"strings"
	"testing"
)

func TestNextIDFollowsTheHighestNumberInAnyState(t *testing.T) {
	f := newFixture(t)
	if id, err := NextID(f.root, "plan"); err != nil || id != "PLAN-000001" {
		t.Fatalf("an empty project starts at 1: %q %v", id, err)
	}
	f.write("docs/plans/records/PLAN-000001-a.md", "x")
	f.write("docs/plans/records/PLAN-000007-b.md", "x")
	f.write("docs/plans/PLAN-000009-misplaced.md", "x")
	f.write("docs/decisions/records/ADR-000050-c.md", "x")
	f.write("docs/plans/records/PLAN-0000123-too-long.md", "x")
	for name, want := range map[string]string{"plan": "PLAN-000010", "PLAN": "PLAN-000010", "Plan": "PLAN-000010", "adr": "ADR-000051", "nfr": "NFR-000001", "use-case": "UC-000001"} {
		if id, err := NextID(f.root, name); err != nil || id != want {
			t.Errorf("%s: got %q %v, want %s", name, id, err, want)
		}
	}
}

func TestNextIDRejectsUnknownTypesAndExhaustedNumbers(t *testing.T) {
	f := newFixture(t)
	if _, err := NextID(f.root, "memo"); err == nil || !strings.Contains(err.Error(), "plan, adr") {
		t.Fatalf("an unknown type must list the known ones: %v", err)
	}
	f.write("docs/research/records/RES-999999-last.md", "x")
	if _, err := NextID(f.root, "research"); err == nil {
		t.Fatal("RES-999999 leaves no free number")
	}
}
