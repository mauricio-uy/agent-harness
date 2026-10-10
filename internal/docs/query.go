package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Parameter defines one CLI option and the JSON field it decodes into.
type Parameter struct {
	Name, Kind, Description string
	Default                 any
	Enum                    []string
}

// ListParameters are the options of harness docs list.
var ListParameters = []Parameter{
	{"type", "string", "Comma-separated document types (OR within a filter)", defaultQuery.Type, TypeNames()},
	{"status", "string", "Comma-separated lifecycle states; all states by default", defaultQuery.Status, nil},
	{"query", "string", "Case-insensitive words searched in title and Markdown body. Every word must appear", defaultQuery.Query, nil},
	{"related-to", "string", "Documents whose related list directly contains this ID", defaultQuery.RelatedTo, nil},
	{"approval", "string", "Filter approval of the current revision", defaultQuery.Approval, []string{"current", "missing", "stale"}},
	{"updated-since", "string", "Inclusive earliest updated date (YYYY-MM-DD)", defaultQuery.UpdatedSince, nil},
	{"updated-until", "string", "Inclusive latest updated date (YYYY-MM-DD)", defaultQuery.UpdatedUntil, nil},
	{"fields", "string", "Comma-separated metadata fields; dotted paths supported", defaultQuery.Fields, nil},
	{"sort", "string", "Sort field, with ID and path as stable tie breakers", defaultQuery.Sort, []string{"id", "title", "created", "updated"}},
	{"order", "string", "Sort direction", defaultQuery.Order, []string{"asc", "desc"}},
	{"limit", "integer", "Maximum results (1-1000)", defaultQuery.Limit, nil},
	{"offset", "integer", "Results to skip (zero or greater)", defaultQuery.Offset, nil},
	{"staged", "boolean", "Read staged Markdown from the Git index", defaultQuery.Staged, nil},
}

// GetParameters are the options of harness docs get.
var GetParameters = []Parameter{
	{"body", "boolean", "Include the complete Markdown body", false, nil},
	{"staged", "boolean", "Read staged Markdown from the Git index", false, nil},
}

// QueryOptions select, sort and page the documents of a list. Their JSON
// names match the CLI flags with underscores instead of hyphens.
type QueryOptions struct {
	Type         string `json:"type"`
	Status       string `json:"status"`
	Query        string `json:"query"`
	RelatedTo    string `json:"related_to"`
	Approval     string `json:"approval"`
	UpdatedSince string `json:"updated_since"`
	UpdatedUntil string `json:"updated_until"`
	Fields       string `json:"fields"`
	Sort         string `json:"sort"`
	Order        string `json:"order"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
	Staged       bool   `json:"staged"`
}

var defaultQuery = QueryOptions{
	Fields: "id,type,title,status,path,revision,updated",
	Sort:   "id",
	Order:  "asc",
	Limit:  20,
}

// maxIssues bounds the issues a list repeats on every call.
const maxIssues = 10

// DefaultQuery returns the options a list uses when no flag changes them.
func DefaultQuery() QueryOptions { return defaultQuery }

// QueryError is a failure with a stable code that agents and scripts can test.
type QueryError struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Issues  []Issue `json:"issues,omitempty"`
}

func (e *QueryError) Error() string { return e.Message }

func queryError(code, message string) error { return &QueryError{Code: code, Message: message} }

// ErrorJSON wraps err in the error object every docs command prints.
func ErrorJSON(err error) map[string]any {
	e, ok := err.(*QueryError)
	if !ok {
		e = &QueryError{Code: "io_error", Message: err.Error()}
	}
	return map[string]any{"error": e}
}

// ListResult is one page of matching documents.
type ListResult struct {
	Documents []map[string]any `json:"documents"`
	Total     int              `json:"total"`
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
	HasMore   bool             `json:"has_more"`
	// Issues lists the first invalid documents, which the results leave out,
	// and IssuesTotal counts all of them; harness check reports every one.
	Issues      []Issue `json:"issues,omitempty"`
	IssuesTotal int     `json:"issues_total,omitempty"`
}

func splitFilter(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func queryRoot(root string, staged bool) (string, func(), error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		return "", nil, queryError("invalid_root", "root must be a directory")
	}
	if !staged {
		return abs, func() {}, nil
	}
	snapshot, _, err := stagedSnapshot(abs)
	if err != nil {
		return "", nil, queryError("staged_error", err.Error())
	}
	return snapshot, func() { os.RemoveAll(snapshot) }, nil
}

// queryRecords returns the valid documents and the issues of the others.
// Invalid documents are left out instead of failing the query, so an agent can
// still find documents while it repairs one.
func queryRecords(root string, types []string) ([]record, []Issue) {
	var candidates []record
	var issues []Issue
	byID := map[string][]string{}
	for _, t := range documentTypes() {
		if len(types) > 0 && !slices.Contains(types, t.Kind) {
			continue
		}
		found, errors := discover(root, t)
		issues = append(issues, errors...)
		for _, r := range found {
			var problems []string
			if _, err := json.Marshal(jsonValue(r.data)); err != nil {
				problems = append(problems, "metadata cannot be represented as JSON: "+err.Error())
			}
			if !t.MatchesDocument(filepath.Base(r.path), r.data) {
				problems = append(problems, "document identity must match its filename, type and directory")
			}
			status, _ := r.data["status"].(string)
			if !slices.Contains(t.States, status) {
				problems = append(problems, "invalid lifecycle status")
			}
			problems = append(problems, validateDiscoveryFields(r.data)...)
			problems = append(problems, validateApproval(r.data, false)...)
			for _, message := range problems {
				issues = append(issues, Issue{r.rel, message})
			}
			if len(problems) == 0 {
				id, _ := r.data["id"].(string)
				byID[id] = append(byID[id], r.rel)
				candidates = append(candidates, r)
			}
		}
	}
	// A duplicated ID is ambiguous, so none of its documents is returned.
	var records []record
	for _, r := range candidates {
		id, _ := r.data["id"].(string)
		if paths := byID[id]; len(paths) > 1 {
			issues = append(issues, Issue{r.rel, "duplicate ID " + id + " in " + strings.Join(paths, ", ")})
			continue
		}
		records = append(records, r)
	}
	return records, issues
}

func validateQuery(o QueryOptions) (time.Time, time.Time, error) {
	invalid := func(message string) (time.Time, time.Time, error) {
		return time.Time{}, time.Time{}, queryError("invalid_arguments", message)
	}
	types := splitFilter(o.Type)
	for _, t := range types {
		if !slices.Contains(TypeNames(), t) {
			return invalid("unknown document type: " + t)
		}
	}
	var states []string
	for _, t := range documentTypes() {
		if len(types) == 0 || slices.Contains(types, t.Kind) {
			states = append(states, t.States...)
		}
	}
	for _, s := range splitFilter(o.Status) {
		if !slices.Contains(states, s) {
			return invalid("unknown lifecycle status for selected types: " + s)
		}
	}
	if o.RelatedTo != "" && !documentID.MatchString(o.RelatedTo) {
		return invalid("related-to must be a document ID")
	}
	if o.Approval != "" && !slices.Contains([]string{"current", "missing", "stale"}, o.Approval) {
		return invalid("approval must be current, missing or stale")
	}
	if !slices.Contains([]string{"id", "title", "created", "updated"}, o.Sort) {
		return invalid("sort must be id, title, created or updated")
	}
	if o.Order != "asc" && o.Order != "desc" {
		return invalid("order must be asc or desc")
	}
	if o.Limit < 1 || o.Limit > 1000 || o.Offset < 0 {
		return invalid("limit must be 1-1000 and offset must be zero or greater")
	}
	for _, f := range splitFilter(o.Fields) {
		if !validField(f) {
			return invalid("unknown field: " + f)
		}
	}
	if o.Fields == "" {
		return invalid("fields cannot be empty")
	}
	var since, until time.Time
	var err error
	if o.UpdatedSince != "" {
		since, err = time.Parse("2006-01-02", o.UpdatedSince)
		if err != nil {
			return invalid("updated-since must be YYYY-MM-DD")
		}
	}
	if o.UpdatedUntil != "" {
		until, err = time.Parse("2006-01-02", o.UpdatedUntil)
		if err != nil {
			return invalid("updated-until must be YYYY-MM-DD")
		}
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return invalid("updated-since must not follow updated-until")
	}
	return since, until, nil
}

func validField(f string) bool {
	return slices.Contains([]string{"id", "type", "title", "status", "path", "revision", "created", "updated", "approval", "approval.revision", "approval.date", "related", "supersedes", "owner", "validation", "validation.status", "validation.revision", "validation.date", "validation.environment"}, f)
}

func approvalStatus(data map[string]any) string {
	approval := asMap(data["approval"])
	a, ok := PositiveInt(approval["revision"])
	if !ok {
		return "missing"
	}
	revision, _ := PositiveInt(data["revision"])
	if a == revision {
		return "current"
	}
	return "stale"
}

func jsonValue(value any) any {
	switch v := value.(type) {
	case Date:
		return formatDate(v.Time)
	case Timestamp:
		return v.Time.Format(time.RFC3339Nano)
	case map[string]any:
		m := map[string]any{}
		for k, x := range v {
			m[k] = jsonValue(x)
		}
		return m
	case []any:
		a := make([]any, len(v))
		for i, x := range v {
			a[i] = jsonValue(x)
		}
		return a
	default:
		return value
	}
}

func fieldValue(data map[string]any, field string) any {
	var value any = data
	for _, part := range strings.Split(field, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[part]
	}
	return jsonValue(value)
}

// List reads the current source files on every call; filters combine with AND.
func List(root string, o QueryOptions) (ListResult, error) {
	since, until, err := validateQuery(o)
	if err != nil {
		return ListResult{}, err
	}
	source, cleanup, err := queryRoot(root, o.Staged)
	if err != nil {
		return ListResult{}, err
	}
	defer cleanup()
	records, issues := queryRecords(source, splitFilter(o.Type))
	statuses, fields := splitFilter(o.Status), splitFilter(o.Fields)
	words := strings.Fields(strings.ToLower(o.Query))
	matching := slices.DeleteFunc(records, func(r record) bool {
		if len(statuses) > 0 && !slices.Contains(statuses, fmt.Sprint(r.data["status"])) {
			return true
		}
		if len(words) > 0 {
			text := strings.ToLower(fmt.Sprint(r.data["title"]) + "\n" + r.body)
			if slices.ContainsFunc(words, func(word string) bool { return !strings.Contains(text, word) }) {
				return true
			}
		}
		if o.RelatedTo != "" && !slices.Contains(relationValues(r.data, "related"), o.RelatedTo) {
			return true
		}
		if o.Approval != "" && approvalStatus(r.data) != o.Approval {
			return true
		}
		updated, _ := ISODate(r.data["updated"])
		return !since.IsZero() && updated.Before(since) || !until.IsZero() && updated.After(until)
	})
	slices.SortFunc(matching, func(a, b record) int {
		c := strings.Compare(strings.ToLower(fmt.Sprint(jsonValue(a.data[o.Sort]))), strings.ToLower(fmt.Sprint(jsonValue(b.data[o.Sort]))))
		if c == 0 {
			c = strings.Compare(fmt.Sprint(a.data["id"]), fmt.Sprint(b.data["id"]))
		}
		if c == 0 {
			c = strings.Compare(a.rel, b.rel)
		}
		if o.Order == "desc" {
			c = -c
		}
		return c
	})
	result := ListResult{Documents: []map[string]any{}, Total: len(matching), Limit: o.Limit, Offset: o.Offset, Issues: issues[:min(len(issues), maxIssues)], IssuesTotal: len(issues)}
	start := min(o.Offset, len(matching))
	end := start + min(o.Limit, len(matching)-start)
	result.HasMore = end < len(matching)
	for _, r := range matching[start:end] {
		data := map[string]any{}
		for _, f := range fields {
			if f == "path" {
				data[f] = r.rel
			} else {
				data[f] = fieldValue(r.data, f)
			}
		}
		result.Documents = append(result.Documents, data)
	}
	return result, nil
}

// Get returns complete frontmatter and optionally the Markdown body.
func Get(root, id string, body, staged bool) (map[string]any, error) {
	if !documentID.MatchString(id) {
		return nil, queryError("invalid_arguments", "id must be a document ID")
	}
	source, cleanup, err := queryRoot(root, staged)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var types []string
	for _, t := range documentTypes() {
		if t.hasID(id) {
			types = append(types, t.Kind)
		}
	}
	records, issues := queryRecords(source, types)
	for _, r := range records {
		if r.data["id"] == id {
			data := jsonValue(r.data).(map[string]any)
			data["path"] = r.rel
			if body {
				data["body"] = r.body
			}
			return data, nil
		}
	}
	// Filenames start with the ID, so these issues belong to the requested document.
	var own []Issue
	for _, issue := range issues {
		name := path.Base(issue.File)
		if strings.HasPrefix(name, id+"-") || name == id+".md" {
			own = append(own, issue)
		}
	}
	if len(own) > 0 {
		return nil, &QueryError{Code: "invalid_documents", Message: id + " is invalid; run harness check", Issues: own}
	}
	return nil, &QueryError{Code: "not_found", Message: "document not found: " + id}
}

// WriteJSON prints value as one compact line, the form agents read cheaply.
func WriteJSON(out io.Writer, value any) error {
	return json.NewEncoder(out).Encode(value)
}
