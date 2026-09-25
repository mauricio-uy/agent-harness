package docs

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// suites lists every document suite in the order they run. The document types
// and every pattern that recognizes their IDs derive from it.
var suites = []*Suite{Plans, Specifications, Research, Runbooks}

// Suites names the suites in the order they run.
var Suites = suiteNames()

func suiteNames() []string {
	names := make([]string, len(suites))
	for i, s := range suites {
		names[i] = s.Name
	}
	return names
}

// suiteNamed returns the suite called name, or nil.
func suiteNamed(name string) *Suite {
	i := slices.IndexFunc(suites, func(s *Suite) bool { return s.Name == name })
	if i < 0 {
		return nil
	}
	return suites[i]
}

// documentTypes lists every document type across the suites.
func documentTypes() []DocType {
	var types []DocType
	for _, s := range suites {
		types = append(types, s.Types...)
	}
	return types
}

// idPrefixes is a regular expression alternation of every ID prefix, longest
// first so that no prefix shadows a longer one ending in it, such as FR in NFR.
func idPrefixes() string {
	var prefixes []string
	for _, t := range documentTypes() {
		prefixes = append(prefixes, regexp.QuoteMeta(t.Prefix))
	}
	slices.SortFunc(prefixes, func(a, b string) int { return cmp.Or(len(b)-len(a), strings.Compare(a, b)) })
	return "(?:" + strings.Join(prefixes, "|") + ")"
}

// The ID patterns are compiled in init because the suites' validators use
// them, and a package-level initializer would make that a cycle.
var (
	documentID     *regexp.Regexp // a whole ID
	documentFile   *regexp.Regexp // a record filename, capturing its ID
	documentPrefix *regexp.Regexp // an ID at the start of a name
	documentAny    *regexp.Regexp // an ID anywhere
	sixDigits      = regexp.MustCompile(`^[0-9]{6}$`)
)

func init() {
	anyID := idPrefixes() + `-[0-9]{6}`
	documentID = regexp.MustCompile(`^` + anyID + `$`)
	documentFile = regexp.MustCompile(`^(` + anyID + `)-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	documentPrefix = regexp.MustCompile(`^` + anyID)
	documentAny = regexp.MustCompile(anyID)
}

// hasID reports whether id is this type's prefix followed by six digits.
func (t DocType) hasID(id string) bool {
	number, ok := strings.CutPrefix(id, t.Prefix+"-")
	return ok && sixDigits.MatchString(number)
}
