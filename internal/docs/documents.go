package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// DocType describes a document type and its directory under docs/.
type DocType struct {
	Kind, Prefix, Folder string
	// States lists the lifecycle states.
	States []string
	// Approved lists the states that require approval of the current revision.
	Approved []string
}

// Suite groups document types for validation.
type Suite struct {
	Name     string
	Types    []DocType
	validate func(record, DocType) []string
}

type record struct {
	path, rel string
	body      string
	data      map[string]any
	kind      string // empty for external documents that only establish identity
}

var externalFile = regexp.MustCompile(`^([A-Z]+-[0-9]{6})[-.]`)

var reviewStates = []string{"awaiting-approval", "draft", "approved", "rejected", "superseded"}

// Specifications covers ADRs, use cases, and functional and non-functional requirements.
var Specifications = &Suite{Name: "specifications", Types: []DocType{
	{"adr", "ADR", "decisions", reviewStates, []string{"approved"}},
	{"use-case", "UC", "use-cases", reviewStates, []string{"approved"}},
	{"functional-requirement", "FR", "requirements/functional", reviewStates, []string{"approved"}},
	{"non-functional-requirement", "NFR", "requirements/non-functional", reviewStates, []string{"approved"}},
}}

// Research covers research records.
var Research = &Suite{Name: "research", Types: []DocType{
	{"research", "RES", "research", reviewStates, []string{"approved"}},
}}

var (
	runbookStates    = []string{"awaiting-approval", "draft", "approved", "rejected", "retired", "superseded"}
	validationStates = []string{"not-validated", "partial", "passed", "failed", "stale"}
)

// Runbooks covers operational runbooks, which add ownership and validation records.
var Runbooks = &Suite{
	Name:     "runbooks",
	Types:    []DocType{{"runbook", "RUN", "runbooks", runbookStates, []string{"approved"}}},
	validate: validateRunbook,
}

func (s *Suite) owns(prefix string) bool {
	return slices.ContainsFunc(s.Types, func(t DocType) bool { return t.Prefix == prefix })
}

func (s *Suite) approvedStates(kind string) []string {
	for _, t := range s.Types {
		if t.Kind == kind {
			return t.Approved
		}
	}
	return nil
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

// MatchesDocument checks the shared identity rules used by validation and
// queries. filename is the document's base name.
func (t DocType) MatchesDocument(filename string, data map[string]any) bool {
	return len(t.identityErrors(filename, data)) == 0
}

func (t DocType) identityErrors(filename string, data map[string]any) []string {
	var errors []string
	identifier, _ := data["id"].(string)
	if !t.hasID(identifier) {
		errors = append(errors, fmt.Sprintf("id must use prefix %s and six digits", t.Prefix))
	}
	if match := documentFile.FindStringSubmatch(filename); match == nil || match[1] != identifier {
		errors = append(errors, "filename must match the ID and a lowercase hyphenated slug")
	}
	if kind, _ := data["type"].(string); kind != t.Kind {
		errors = append(errors, fmt.Sprintf("type must be %s in this directory", t.Kind))
	}
	return errors
}

func validateDocument(r record, t DocType) []string {
	var errors []string
	data := r.data
	for _, key := range []string{"id", "type", "title", "status", "created", "updated", "revision", "approval", "related"} {
		if _, ok := data[key]; !ok {
			errors = append(errors, "missing field: "+key)
		}
	}
	errors = append(errors, t.identityErrors(filepath.Base(r.path), data)...)
	errors = append(errors, validateDiscoveryFields(data)...)
	status, _ := data["status"].(string)
	if !slices.Contains(t.States, status) {
		errors = append(errors, "status must be one of: "+strings.Join(t.States, ", "))
	}
	created, createdOK := ISODate(data["created"])
	updated, updatedOK := ISODate(data["updated"])
	if createdOK && updatedOK && created.After(updated) {
		errors = append(errors, "updated must not precede created")
	}
	errors = append(errors, validateApproval(data, slices.Contains(t.Approved, status))...)
	seen := map[string]bool{}
	identifier, _ := data["id"].(string)
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

// validateDiscoveryFields checks values needed to locate, sort and describe a
// document. Lifecycle checks add chronology, current approval and relations.
func validateDiscoveryFields(data map[string]any) []string {
	var errors []string
	if !singleLine(data["title"]) {
		errors = append(errors, "title must be a nonempty single-line string")
	}
	for _, field := range []string{"created", "updated"} {
		if _, ok := ISODate(data[field]); !ok {
			errors = append(errors, field+" must be a valid YYYY-MM-DD date")
		}
	}
	if _, ok := PositiveInt(data["revision"]); !ok {
		errors = append(errors, "revision must be a positive integer")
	}
	return errors
}

// validateApproval checks the evidence itself. Queries allow a well-formed
// approval of an older revision; lifecycle validation can require a current one.
func validateApproval(data map[string]any, requireCurrent bool) []string {
	var errors []string
	revision, revisionOK := PositiveInt(data["revision"])
	created, createdOK := ISODate(data["created"])
	updated, updatedOK := ISODate(data["updated"])
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
		if requireCurrent && (!approvedOK || !revisionOK || approved != revision || when == nil) {
			status, _ := data["status"].(string)
			errors = append(errors, status+" requires approval of the current revision")
		}
	}
	return errors
}

func validateRunbook(r record, t DocType) []string {
	data := r.data
	errors := validateDocument(r, t)
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

func (s *Suite) relationErrors(records []record, catalog map[string][]record) []Issue {
	var errors []Issue
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
					errors = append(errors, Issue{r.rel, fmt.Sprintf("%s must resolve to one document: %s", field, target)})
				case field != "supersedes":
				case matches[0].kind != r.kind:
					errors = append(errors, Issue{r.rel, "supersedes must reference the same type: " + target})
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
		errors = append(errors, Issue{"", "supersedes cycle detected; involved or downstream IDs: " + strings.Join(involved, ", ")})
	}
	for _, r := range records {
		identifier, ok := r.data["id"].(string)
		if status, _ := r.data["status"].(string); status != "superseded" || !ok {
			continue
		}
		approved := slices.ContainsFunc(replacements[identifier], func(successor replacement) bool {
			status, _ := successor.data["status"].(string)
			_, revisionOK := PositiveInt(asMap(successor.data["approval"])["revision"])
			return (slices.Contains(s.approvedStates(successor.kind), status) || status == "superseded" || status == "retired") && revisionOK
		})
		if !approved {
			errors = append(errors, Issue{r.rel, "superseded document requires an approved replacement"})
		}
	}
	return errors
}

// discover reads documents directly from their type directory. Walk errors are
// reported so a query never silently returns an incomplete catalog.
func discover(root string, t DocType) ([]record, []Issue) {
	directory := filepath.Join(root, "docs", filepath.FromSlash(t.Folder))
	if !within(root, directory) || isFile(directory) || isSymlink(directory) {
		return nil, []Issue{{relSlash(root, directory), "invalid document directory"}}
	}
	var found []record
	var issues []Issue
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == directory && os.IsNotExist(err) {
				return nil
			}
			issues = append(issues, Issue{relSlash(root, path), err.Error()})
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		rel := relSlash(root, path)
		if filepath.Dir(path) == directory && entry.Name() == "README.md" {
			return nil
		}
		if filepath.Dir(path) != directory || !entry.Type().IsRegular() || !within(root, path) {
			issues = append(issues, Issue{rel, "documents belong directly in docs/" + t.Folder + "/ and must be regular files"})
			return nil
		}
		data, body, err := readDocument(path)
		if err != nil {
			issues = append(issues, Issue{rel, err.Error()})
			return nil
		}
		found = append(found, record{path: path, rel: rel, data: data, body: body, kind: t.Kind})
		return nil
	})
	if err != nil {
		issues = append(issues, Issue{relSlash(root, directory), err.Error()})
	}
	return found, issues
}

// Issue is one validation error, located in File when it concerns one file.
type Issue struct {
	File    string `json:"file,omitempty"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	if i.File == "" {
		return i.Message
	}
	return i.File + ": " + i.Message
}

// ValidationResult contains the documents counted and all validation errors.
type ValidationResult struct {
	Documents int     `json:"documents"`
	Errors    []Issue `json:"errors"`
}

// Inspect validates documents, approvals and relationships without writing.
func (s *Suite) Inspect(root string) ValidationResult {
	var records []record
	var errors []Issue
	catalog := map[string][]record{}
	validate := s.validate
	if validate == nil {
		validate = validateDocument
	}
	for _, t := range s.Types {
		found, structural := discover(root, t)
		errors = append(errors, structural...)
		for _, r := range found {
			records = append(records, r)
			for _, e := range validate(r, t) {
				errors = append(errors, Issue{r.rel, e})
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
				errors = append(errors, Issue{rel, "referenced document resolves outside repository"})
				continue
			}
			data, err := ReadMetadata(path)
			switch {
			case err != nil:
				errors = append(errors, Issue{rel, err.Error()})
			case data["id"] != match[1]:
				errors = append(errors, Issue{rel, "referenced document ID must match filename: " + match[1]})
			default:
				catalog[match[1]] = append(catalog[match[1]], record{path: path, rel: rel, data: data})
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
			errors = append(errors, Issue{"", fmt.Sprintf("duplicate ID %s: %s", identifier, strings.Join(paths, ", "))})
		}
	}
	errors = append(errors, s.relationErrors(records, catalog)...)
	return ValidationResult{Documents: len(records), Errors: append([]Issue{}, errors...)}
}

// Report emits validation errors and a document count.
func (r ValidationResult) Report(out report.Sink) bool {
	for _, e := range r.Errors {
		out.Emit(report.Error, e.String())
	}
	report.Emitf(out, report.Plain, "%d document(s); %d validation error(s). Read-only.", r.Documents, len(r.Errors))
	return len(r.Errors) == 0
}
