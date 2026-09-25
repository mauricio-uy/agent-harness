package docs

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// DocType describes one document type. Its directory under docs/ holds a
// hand-written README.md, one generated index per state, and the documents
// themselves in records/, where they never move.
type DocType struct {
	Kind, Prefix, Folder, Title string
	// States lists the lifecycle states in index order. A type without states
	// gets a single all.md index, so states can be added later without moving files.
	States []string
	// Approved is the state that records approval of the current revision.
	Approved string
}

// RecordsDir is the directory, inside each type's folder, that holds its documents.
const RecordsDir = "records"

// Suite is a group of document types synchronized together by one command.
// Each suite owns only its own indexes.
type Suite struct {
	Name     string
	Types    []DocType
	validate func(record, DocType, *Suite) []string
	// relations checks references across records; nil uses relationErrors.
	relations func(records []record, catalog map[string][]record) []string
	columns   []column
}

type column struct {
	label string
	value func(map[string]any) string
}

type record struct {
	path, rel string
	data      map[string]any
	kind      string // empty for external documents that only establish identity
}

var (
	documentID   = regexp.MustCompile(`^(?:ADR|UC|FR|NFR|RES|RUN|PLAN)-[0-9]{6}$`)
	documentFile = regexp.MustCompile(`^((?:ADR|UC|FR|NFR|RES|RUN)-[0-9]{6})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	externalFile = regexp.MustCompile(`^([A-Z]+-[0-9]{6})[-.]`)
)

// reviewStates are the states of a reviewed document whose approval state is approved.
func reviewStates(approved string) []string {
	return []string{"awaiting-approval", "draft", approved, "rejected", "superseded"}
}

// Specifications covers ADRs, use cases, and functional and non-functional requirements.
var Specifications = &Suite{Name: "specifications", Types: []DocType{
	{"adr", "ADR", "decisions", "Architecture Decision Records", reviewStates("accepted"), "accepted"},
	{"use-case", "UC", "use-cases", "Use Cases", reviewStates("approved"), "approved"},
	{"functional-requirement", "FR", "requirements/functional", "Functional Requirements", reviewStates("approved"), "approved"},
	{"non-functional-requirement", "NFR", "requirements/non-functional", "Non-Functional Requirements", reviewStates("approved"), "approved"},
}}

// Research covers research records.
var Research = &Suite{Name: "research", Types: []DocType{
	{"research", "RES", "research", "Research", reviewStates("approved"), "approved"},
}}

var (
	runbookStates    = []string{"awaiting-approval", "draft", "approved", "rejected", "retired", "superseded"}
	validationStates = []string{"not-validated", "partial", "passed", "failed", "stale"}
)

// Runbooks covers operational runbooks, which add ownership and validation records.
var Runbooks = &Suite{
	Name:     "runbooks",
	Types:    []DocType{{"runbook", "RUN", "runbooks", "Runbooks", runbookStates, "approved"}},
	validate: validateRunbook,
	columns: []column{
		{"Validation", func(data map[string]any) string { return fmt.Sprint(asMap(data["validation"])["status"]) }},
		{"Validated on", func(data map[string]any) string {
			if when, ok := ISODate(asMap(data["validation"])["date"]); ok {
				return formatDate(when)
			}
			return "—"
		}},
	},
}

func (s *Suite) owns(prefix string) bool {
	return slices.ContainsFunc(s.Types, func(t DocType) bool { return t.Prefix == prefix })
}

func (s *Suite) approvedState(kind string) string {
	for _, t := range s.Types {
		if t.Kind == kind {
			return t.Approved
		}
	}
	return ""
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func validateDocument(r record, t DocType, allowed []string) []string {
	var errors []string
	data := r.data
	for _, key := range []string{"id", "type", "title", "status", "created", "updated", "revision", "approval", "related"} {
		if _, ok := data[key]; !ok {
			errors = append(errors, "missing field: "+key)
		}
	}
	identifier, _ := data["id"].(string)
	if !regexp.MustCompile(`^` + t.Prefix + `-[0-9]{6}$`).MatchString(identifier) {
		errors = append(errors, fmt.Sprintf("id must use prefix %s and six digits", t.Prefix))
	}
	if match := documentFile.FindStringSubmatch(filepath.Base(r.path)); match == nil || match[1] != identifier {
		errors = append(errors, "filename must match the ID and a lowercase hyphenated slug")
	}
	if kind, _ := data["type"].(string); kind != t.Kind {
		errors = append(errors, fmt.Sprintf("type must be %s in this directory", t.Kind))
	}
	if !singleLine(data["title"]) {
		errors = append(errors, "title must be a nonempty single-line string")
	}
	status, _ := data["status"].(string)
	if !slices.Contains(allowed, status) {
		errors = append(errors, "status must be one of: "+strings.Join(allowed, ", "))
	}
	created, createdOK := ISODate(data["created"])
	updated, updatedOK := ISODate(data["updated"])
	if !createdOK {
		errors = append(errors, "created must be a valid YYYY-MM-DD date")
	}
	if !updatedOK {
		errors = append(errors, "updated must be a valid YYYY-MM-DD date")
	}
	if createdOK && updatedOK && created.After(updated) {
		errors = append(errors, "updated must not precede created")
	}
	revision, revisionOK := PositiveInt(data["revision"])
	if !revisionOK {
		errors = append(errors, "revision must be a positive integer")
	}
	approval, isMap := data["approval"].(map[string]any)
	_, hasRevision := approval["revision"]
	_, hasDate := approval["date"]
	if !isMap || !hasRevision || !hasDate {
		errors = append(errors, "approval must contain revision and date")
	} else {
		approvedValue, when := approval["revision"], approval["date"]
		approved, approvedOK := PositiveInt(approvedValue)
		if (approvedValue == nil) != (when == nil) {
			errors = append(errors, "approval fields must be populated together")
		}
		if approvedValue != nil && !approvedOK {
			errors = append(errors, "approval.revision must be a positive integer or null")
		} else if approvedOK && revisionOK && approved > revision {
			errors = append(errors, "approval.revision must not exceed revision")
		}
		if when != nil {
			if parsed, ok := ISODate(when); !ok {
				errors = append(errors, "approval.date must be a valid YYYY-MM-DD date")
			} else if createdOK && updatedOK && (parsed.Before(created) || parsed.After(updated)) {
				errors = append(errors, "approval.date must fall between created and updated")
			}
		}
		if status == t.Approved && (!approvedOK || !revisionOK || approved != revision || when == nil) {
			errors = append(errors, status+" requires approval of the current revision")
		}
	}
	seen := map[string]bool{}
	for _, field := range []string{"related", "supersedes"} {
		value, present := data[field]
		if !present {
			continue
		}
		values, ok := value.([]any)
		if !ok {
			errors = append(errors, field+" must be a list of IDs")
			continue
		}
		for _, item := range values {
			target, ok := item.(string)
			if !ok || !documentID.MatchString(target) {
				errors = append(errors, field+" contains an invalid document ID")
				continue
			}
			if target == identifier {
				errors = append(errors, field+" must not refer to this document")
			}
			if seen[target] {
				errors = append(errors, "duplicate relation: "+target)
			}
			seen[target] = true
		}
	}
	return errors
}

func validateRunbook(r record, t DocType, s *Suite) []string {
	data := r.data
	errors := validateDocument(r, t, runbookStates)
	owner, hasOwner := data["owner"]
	if !hasOwner {
		errors = append(errors, "missing field: owner")
	}
	if owner == nil {
		if status, _ := data["status"].(string); status != "draft" {
			errors = append(errors, "owner is required outside draft")
		}
	} else if !singleLine(owner) {
		errors = append(errors, "owner must be a nonempty single-line string or null in draft")
	}

	validation, isMap := data["validation"].(map[string]any)
	complete := isMap
	for _, key := range []string{"status", "revision", "date", "environment"} {
		if _, ok := validation[key]; !ok {
			complete = false
		}
	}
	if !complete {
		return append(errors, "validation must contain status, revision, date, and environment")
	}
	status, _ := validation["status"].(string)
	if !slices.Contains(validationStates, status) {
		return append(errors, "validation.status must be one of: "+strings.Join(validationStates, ", "))
	}
	if status == "not-validated" {
		if validation["revision"] != nil || validation["date"] != nil || validation["environment"] != nil {
			errors = append(errors, "not-validated requires null validation revision, date, and environment")
		}
		return errors
	}
	validated, validatedOK := PositiveInt(validation["revision"])
	revision, revisionOK := PositiveInt(data["revision"])
	if !validatedOK {
		errors = append(errors, "validation.revision must be a positive integer")
	} else if revisionOK {
		if validated > revision {
			errors = append(errors, "validation.revision must not exceed revision")
		} else if status != "stale" && validated != revision {
			errors = append(errors, fmt.Sprintf("validation.status %s requires the current revision; use stale for older evidence", status))
		}
	}
	created, createdOK := ISODate(data["created"])
	updated, updatedOK := ISODate(data["updated"])
	if when, ok := ISODate(validation["date"]); !ok {
		errors = append(errors, "validation.date must be a valid YYYY-MM-DD date")
	} else if createdOK && updatedOK && (when.Before(created) || when.After(updated)) {
		errors = append(errors, "validation.date must fall between created and updated")
	}
	if !singleLine(validation["environment"]) {
		errors = append(errors, "validation.environment must be a nonempty single-line string")
	}
	return errors
}

func relationValues(data map[string]any, key string) []string {
	values, _ := data[key].([]any)
	var result []string
	for _, value := range values {
		if s, ok := value.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

type replacement struct {
	data map[string]any
	kind string
}

func (s *Suite) relationErrors(records []record, catalog map[string][]record) []string {
	var errors []string
	graph := map[string]map[string]bool{}
	var order []string
	replacements := map[string][]replacement{}
	for _, r := range records {
		identifier, ok := r.data["id"].(string)
		if !ok {
			continue
		}
		if graph[identifier] == nil {
			graph[identifier] = map[string]bool{}
			order = append(order, identifier)
		}
		for _, field := range []string{"related", "supersedes"} {
			for _, target := range relationValues(r.data, field) {
				matches := catalog[target]
				switch {
				case len(matches) != 1:
					errors = append(errors, fmt.Sprintf("%s: %s must resolve to one document: %s", r.rel, field, target))
				case field != "supersedes":
				case matches[0].kind != r.kind:
					errors = append(errors, fmt.Sprintf("%s: supersedes must reference the same type: %s", r.rel, target))
				default:
					graph[identifier][target] = true
					replacements[target] = append(replacements[target], replacement{r.data, r.kind})
				}
			}
		}
	}
	// Topological elimination scales to long replacement chains without recursion.
	degrees := map[string]int{}
	for _, node := range order {
		degrees[node] += 0
		for target := range graph[node] {
			degrees[target]++
		}
	}
	var queue []string
	for _, node := range order {
		if degrees[node] == 0 {
			queue = append(queue, node)
		}
	}
	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		targets := make([]string, 0, len(graph[node]))
		for target := range graph[node] {
			targets = append(targets, target)
		}
		slices.Sort(targets)
		for _, target := range targets {
			degrees[target]--
			if degrees[target] == 0 {
				queue = append(queue, target)
			}
		}
	}
	if visited != len(degrees) {
		var involved []string
		for node, degree := range degrees {
			if degree != 0 {
				involved = append(involved, node)
			}
		}
		slices.Sort(involved)
		errors = append(errors, "supersedes cycle detected; involved or downstream IDs: "+strings.Join(involved, ", "))
	}
	for _, r := range records {
		identifier, ok := r.data["id"].(string)
		if status, _ := r.data["status"].(string); status != "superseded" || !ok {
			continue
		}
		approved := slices.ContainsFunc(replacements[identifier], func(successor replacement) bool {
			status, _ := successor.data["status"].(string)
			_, revisionOK := PositiveInt(asMap(successor.data["approval"])["revision"])
			return (status == s.approvedState(successor.kind) || status == "superseded" || status == "retired") && revisionOK
		})
		if !approved {
			errors = append(errors, r.rel+": superseded document requires an approved replacement")
		}
	}
	return errors
}

// index is one generated index file of a type: a state, or every document
// for a type without states.
type index struct {
	name, title string
	includes    func(record) bool
}

func indexesOf(t DocType) []index {
	if len(t.States) == 0 {
		return []index{{"all", "All", func(record) bool { return true }}}
	}
	indexes := make([]index, len(t.States))
	for i, state := range t.States {
		indexes[i] = index{state, titleCase(state), func(r record) bool { return r.data["status"] == state }}
	}
	return indexes
}

func (s *Suite) indexContent(t DocType, ix index, records []record, directory string) string {
	var matching []record
	for _, r := range records {
		if r.kind == t.Kind && ix.includes(r) {
			matching = append(matching, r)
		}
	}
	slices.SortStableFunc(matching, func(a, b record) int {
		return strings.Compare(fmt.Sprint(a.data["id"]), fmt.Sprint(b.data["id"]))
	})
	headings := []string{"ID", "Document", "Revision"}
	for _, c := range s.columns {
		headings = append(headings, c.label)
	}
	headings = append(headings, "Updated")
	separators := make([]string, len(headings))
	for i := range separators {
		separators[i] = "---"
	}
	lines := []string{"# " + t.Title + ": " + ix.title, "",
		"| " + strings.Join(headings, " | ") + " |", "| " + strings.Join(separators, " | ") + " |"}
	for _, r := range matching {
		updated, _ := ISODate(r.data["updated"])
		cells := []string{fmt.Sprint(r.data["id"]),
			fmt.Sprintf("[%s](%s)", TableText(fmt.Sprint(r.data["title"])), QuotePath(relSlash(directory, r.path))),
			fmt.Sprint(r.data["revision"])}
		for _, c := range s.columns {
			cells = append(cells, TableText(c.value(r.data)))
		}
		cells = append(cells, formatDate(updated))
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	if len(matching) == 0 {
		lines = append(lines, "", "No documents in this state.")
	}
	return strings.Join(lines, "\n") + "\n"
}

func titleCase(status string) string {
	words := strings.Split(status, "-")
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// discover returns the records of one type and any structural errors. Only
// README.md and the generated indexes may sit beside records/.
func discover(root string, t DocType) ([]record, []string) {
	directory := filepath.Join(root, "docs", filepath.FromSlash(t.Folder))
	records := filepath.Join(directory, RecordsDir)
	if !within(root, directory) || isFile(directory) {
		return nil, []string{relSlash(root, directory) + ": invalid document directory"}
	}
	var found []record
	var errors []string
	allowed := map[string]bool{"README.md": true}
	for _, ix := range indexesOf(t) {
		allowed[ix.name+".md"] = true
		if path := filepath.Join(directory, ix.name+".md"); isSymlink(path) || notRegular(path) {
			errors = append(errors, relSlash(root, path)+": index must be a regular file")
		}
	}
	for _, path := range markdownFiles(directory) {
		rel := relSlash(root, path)
		inRecords := within(records, path) && !isSymlink(path)
		switch {
		case filepath.Dir(path) == directory && allowed[filepath.Base(path)]:
			continue
		case strings.HasPrefix(relSlash(directory, path), RecordsDir+"/") && !inRecords:
			errors = append(errors, rel+": document must be a regular file within "+RecordsDir+"/")
			continue
		case !inRecords:
			errors = append(errors, fmt.Sprintf("%s: documents belong in docs/%s/%s/", rel, t.Folder, RecordsDir))
			continue
		}
		data, err := ReadMetadata(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		found = append(found, record{path, rel, data, t.Kind})
	}
	return found, errors
}

// Sync validates the suite's documents and regenerates one index per state.
// Documents never move: a status change moves only a row between indexes. It
// writes only with apply; with check it fails when an index is out of date.
// Any validation error aborts before the first write.
func (s *Suite) Sync(root string, apply, check bool, out io.Writer) int {
	var records []record
	var errors []string
	catalog := map[string][]record{}
	for _, t := range s.Types {
		found, structural := discover(root, t)
		errors = append(errors, structural...)
		validate := func(r record, t DocType, s *Suite) []string { return validateDocument(r, t, t.States) }
		if s.validate != nil {
			validate = s.validate
		}
		for _, r := range found {
			records = append(records, r)
			for _, e := range validate(r, t, s) {
				errors = append(errors, r.rel+": "+e)
			}
			if identifier, ok := r.data["id"].(string); ok {
				catalog[identifier] = append(catalog[identifier], r)
			}
		}
	}
	// Only inspect external records that an owned record actually references.
	external := map[string]bool{}
	for _, r := range records {
		for _, field := range []string{"related", "supersedes"} {
			for _, target := range relationValues(r.data, field) {
				if documentID.MatchString(target) && !s.owns(strings.SplitN(target, "-", 2)[0]) {
					external[target] = true
				}
			}
		}
	}
	if len(external) > 0 {
		for _, path := range markdownFiles(filepath.Join(root, "docs")) {
			match := externalFile.FindStringSubmatch(filepath.Base(path))
			if match == nil || !external[match[1]] {
				continue
			}
			rel := relSlash(root, path)
			if !within(root, path) {
				errors = append(errors, rel+": referenced document resolves outside repository")
				continue
			}
			data, err := ReadMetadata(path)
			switch {
			case err != nil:
				errors = append(errors, fmt.Sprintf("%s: %v", rel, err))
			case data["id"] != match[1]:
				errors = append(errors, fmt.Sprintf("%s: referenced document ID must match filename: %s", rel, match[1]))
			default:
				catalog[match[1]] = append(catalog[match[1]], record{path, rel, data, ""})
			}
		}
	}
	identifiers := make([]string, 0, len(catalog))
	for identifier := range catalog {
		identifiers = append(identifiers, identifier)
	}
	slices.Sort(identifiers)
	for _, identifier := range identifiers {
		if matches := catalog[identifier]; len(matches) > 1 {
			var paths []string
			for _, m := range matches {
				paths = append(paths, m.rel)
			}
			errors = append(errors, fmt.Sprintf("duplicate ID %s: %s", identifier, strings.Join(paths, ", ")))
		}
	}
	if s.relations != nil {
		errors = append(errors, s.relations(records, catalog)...)
	} else {
		errors = append(errors, s.relationErrors(records, catalog)...)
	}
	if len(errors) > 0 {
		return reportErrors(out, errors)
	}
	type update struct{ path, content string }
	var updates []update
	for _, t := range s.Types {
		directory := filepath.Join(root, "docs", filepath.FromSlash(t.Folder))
		for _, ix := range indexesOf(t) {
			path := filepath.Join(directory, ix.name+".md")
			content := s.indexContent(t, ix, records, directory)
			if current, ok := readText(path); !ok || current != content {
				updates = append(updates, update{path, content})
			}
		}
	}
	for _, u := range updates {
		fmt.Fprintln(out, "INDEX "+relSlash(root, u.path))
	}
	if apply {
		for _, u := range updates {
			if err := writeText(u.path, u.content); err != nil {
				fmt.Fprintf(out, "ERROR: %v\n", err)
				return 1
			}
		}
	}
	fmt.Fprintf(out, "%d document(s); %d index update(s). %s\n", len(records), len(updates), modeLabel(apply))
	if check && len(updates) > 0 {
		return 1
	}
	return 0
}

func reportErrors(out io.Writer, errors []string) int {
	for _, e := range errors {
		fmt.Fprintln(out, "ERROR: "+e)
	}
	fmt.Fprintf(out, "%d validation error(s); no files changed.\n", len(errors))
	return 1
}

func modeLabel(apply bool) string {
	if apply {
		return "Applied."
	}
	return "Read-only."
}
