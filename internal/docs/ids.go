package docs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// TypeNamed returns the document type whose kind or ID prefix is name,
// ignoring case, such as "adr", "ADR" or "use-case".
func TypeNamed(name string) (DocType, bool) {
	for _, t := range documentTypes() {
		if strings.EqualFold(name, t.Kind) || strings.EqualFold(name, t.Prefix) {
			return t, true
		}
	}
	return DocType{}, false
}

// TypeNames lists the kind of every document type.
func TypeNames() []string {
	var names []string
	for _, t := range documentTypes() {
		names = append(names, t.Kind)
	}
	return names
}

// NextID returns the ID after the highest one of the type found anywhere in
// docs/, in any state and even when misplaced. Gaps are never reused.
func NextID(root, typeName string) (string, error) {
	t, ok := TypeNamed(typeName)
	if !ok {
		return "", fmt.Errorf("unknown document type %q; choose one of %s", typeName, strings.Join(TypeNames(), ", "))
	}
	highest := 0
	for _, path := range markdownFiles(filepath.Join(root, "docs")) {
		name := filepath.Base(path)
		id := documentPrefix.FindString(name)
		if !t.hasID(id) || startsWithDigit(name[len(id):]) {
			continue
		}
		number, _ := strconv.Atoi(id[len(t.Prefix)+1:])
		highest = max(highest, number)
	}
	if highest >= 999999 {
		return "", fmt.Errorf("%s-999999 is taken; no six-digit number is left", t.Prefix)
	}
	return fmt.Sprintf("%s-%06d", t.Prefix, highest+1), nil
}
