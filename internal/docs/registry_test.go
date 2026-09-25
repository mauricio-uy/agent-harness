package docs

import "testing"

// Every pattern that recognizes document IDs derives from the registry, so a
// type added to a suite is recognized everywhere without editing a pattern.
func TestEveryRegisteredTypeIsRecognizedByEveryIDPattern(t *testing.T) {
	types := documentTypes()
	if len(types) != 7 {
		t.Fatalf("expected the 7 harness types, got %d", len(types))
	}
	for _, dt := range types {
		id := dt.Prefix + "-000042"
		if !documentID.MatchString(id) {
			t.Errorf("documentID rejects %s", id)
		}
		if match := documentFile.FindStringSubmatch(id + "-a-slug.md"); match == nil || match[1] != id {
			t.Errorf("documentFile rejects %s", id)
		}
		if documentPrefix.FindString(id+"-a-slug.md") != id {
			t.Errorf("documentPrefix rejects %s", id)
		}
		if got := findDocumentID("../records/" + id + "-moved.md"); got != id {
			t.Errorf("findDocumentID found %q in a path to %s", got, id)
		}
		if !dt.hasID(id) || dt.hasID(id+"0") || dt.hasID("X"+id) {
			t.Errorf("%s: hasID must accept only its own prefix and six digits", dt.Prefix)
		}
	}
}

func TestSuitesAreLookedUpByName(t *testing.T) {
	for _, name := range Suites {
		if s := suiteNamed(name); s == nil || s.Name != name {
			t.Errorf("suite %q not found", name)
		}
	}
	if suiteNamed("nope") != nil {
		t.Error("an unknown suite must not resolve")
	}
}
