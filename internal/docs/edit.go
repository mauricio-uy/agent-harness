package docs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// templates maps each document type to the template its skill owns, relative
// to the project root, so a project's own edits to a template are used.
var templates = map[string]string{
	"plan":                       ".agents/skills/write-plan/assets/plan-template.md",
	"adr":                        ".agents/skills/write-specification/assets/adr-template.md",
	"use-case":                   ".agents/skills/write-specification/assets/use-case-template.md",
	"functional-requirement":     ".agents/skills/write-specification/assets/functional-requirement-template.md",
	"non-functional-requirement": ".agents/skills/write-specification/assets/non-functional-requirement-template.md",
	"research":                   ".agents/skills/write-research-report/assets/research-template.md",
	"runbook":                    ".agents/skills/write-runbook/assets/runbook-template.md",
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// historicalStates end a document's life; a change needs a new document.
var historicalStates = []string{"superseded", "cancelled", "completed", "retired"}

// Editor changes document frontmatter. Every change also sets updated to
// Today and is refused when it would add a validation error to the document.
type Editor struct {
	Root  string
	Today time.Time
}

// NewDocument holds what a new document needs besides its template.
type NewDocument struct {
	Type, Title, Slug   string
	Related, Supersedes []string
}

// Create copies the type's template into a new document with the next free
// ID. It holds a lock while choosing the ID, so concurrent agents in the same
// clone never receive the same one.
func (e Editor) Create(n NewDocument) (map[string]any, error) {
	t, ok := TypeNamed(n.Type)
	if !ok {
		return nil, queryError("invalid_arguments", "unknown document type: "+n.Type+"; choose one of "+strings.Join(TypeNames(), ", "))
	}
	if !singleLine(n.Title) {
		return nil, queryError("invalid_arguments", "title must be a nonempty single line")
	}
	slug := n.Slug
	if slug == "" {
		slug = slugify(n.Title)
	}
	if !slugPattern.MatchString(slug) {
		return nil, queryError("invalid_arguments", "slug must use lowercase letters, digits and single hyphens")
	}
	raw, err := os.ReadFile(filepath.Join(e.Root, filepath.FromSlash(templates[t.Kind])))
	if err != nil {
		return nil, queryError("template_missing", fmt.Sprintf("cannot read the %s template (%v); run harness init", t.Kind, err))
	}
	document, err := parseEditable(raw)
	if err != nil {
		return nil, queryError("template_invalid", templates[t.Kind]+": "+err.Error())
	}
	unlock, err := e.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	id, err := NextID(e.Root, t.Kind)
	if err != nil {
		return nil, queryError("invalid_arguments", err.Error())
	}
	if err := e.checkTargets(id, t, n.Related, n.Supersedes); err != nil {
		return nil, err
	}
	document.set("id", stringNode(id))
	document.set("title", stringNode(n.Title))
	document.set("created", dateNode(e.Today))
	document.set("updated", dateNode(e.Today))
	document.set("related", listNode(n.Related))
	if len(n.Supersedes) > 0 || document.lookup("supersedes") != nil {
		document.set("supersedes", listNode(n.Supersedes))
	}
	document.replaceTitleHeading(n.Title)
	content, err := document.bytes()
	if err != nil {
		return nil, err
	}
	rel := "docs/" + t.Folder + "/" + id + "-" + slug + ".md"
	path := filepath.Join(e.Root, filepath.FromSlash(rel))
	if !within(e.Root, path) {
		return nil, queryError("invalid_root", "docs/"+t.Folder+" resolves outside the project")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return Get(e.Root, id, false, false)
}

// lock serializes ID allocation through a directory, which the file system
// creates atomically. A lock older than a minute belongs to a process that
// died and is taken over.
func (e Editor) lock() (func(), error) {
	path := filepath.Join(e.Root, ".agents", ".docs-new.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		err := os.Mkdir(path, 0o755)
		if err == nil {
			return func() { os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > time.Minute {
			os.Remove(path)
			continue
		}
		if attempt == 100 {
			return nil, queryError("locked", "another harness docs new is running; retry, or delete "+relSlash(e.Root, path)+" if none is")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// SetStatus moves a document to state. States that require approval need the
// current revision approved, and superseded needs an approved replacement.
func (e Editor) SetStatus(id, state string) (map[string]any, error) {
	return e.update(id, func(d *editable, data map[string]any, t DocType) error {
		if !slices.Contains(t.States, state) {
			return queryError("invalid_arguments", fmt.Sprintf("%s states are: %s", t.Kind, strings.Join(t.States, ", ")))
		}
		current, _ := data["status"].(string)
		if slices.Contains(t.Approved, current) && (state == "draft" || state == "awaiting-approval") {
			if slices.Contains(historicalStates, current) {
				return queryError("invalid_state", id+" is "+current+"; create a new document instead")
			}
			return queryError("invalid_state", id+" is "+current+"; run harness docs revise "+id+" for a material change, or harness docs revoke "+id+" when the human withdraws the approval")
		}
		if slices.Contains(t.Approved, state) && approvalStatus(data) != "current" {
			return queryError("approval_required", state+" requires approval of the current revision; after the human approves it, run harness docs approve "+id)
		}
		if state == "superseded" {
			if err := e.requireApprovedReplacement(id, t); err != nil {
				return err
			}
		}
		d.set("status", stringNode(state))
		return nil
	})
}

// Approve records the human's approval of the current revision and sets the
// document approved. Only a document under review can be approved.
func (e Editor) Approve(id string) (map[string]any, error) {
	return e.update(id, func(d *editable, data map[string]any, _ DocType) error {
		status, _ := data["status"].(string)
		if status != "draft" && status != "awaiting-approval" {
			return queryError("invalid_state", "only a draft or awaiting-approval document can be approved; "+id+" is "+status)
		}
		revision, ok := PositiveInt(data["revision"])
		if !ok {
			return queryError("invalid_document", "revision must be a positive integer")
		}
		d.set("approval.revision", intNode(revision))
		d.set("approval.date", dateNode(e.Today))
		d.set("status", stringNode("approved"))
		return nil
	})
}

// Revise starts a new revision for material changes. The earlier approval is
// kept as history and no longer covers the document, and runbook validation
// evidence becomes stale. An approved ADR keeps its decision, so a change
// needs a replacement ADR.
func (e Editor) Revise(id string) (map[string]any, error) {
	return e.update(id, func(d *editable, data map[string]any, t DocType) error {
		status, _ := data["status"].(string)
		if slices.Contains(historicalStates, status) {
			return queryError("invalid_state", id+" is "+status+"; create a new document instead")
		}
		if t.Kind == "adr" && status == "approved" {
			return queryError("invalid_state", id+" is approved; propose a replacement with harness docs new adr --title TEXT --supersedes "+id)
		}
		revision, ok := PositiveInt(data["revision"])
		if !ok {
			return queryError("invalid_document", "revision must be a positive integer")
		}
		d.set("revision", intNode(revision+1))
		d.set("status", stringNode("draft"))
		validation := asMap(data["validation"])
		if validation != nil && slices.Contains([]string{"passed", "partial", "failed"}, fmt.Sprint(validation["status"])) {
			d.set("validation.status", stringNode("stale"))
		}
		return nil
	})
}

// Revoke clears an approval the human withdrew and returns the document to review.
func (e Editor) Revoke(id string) (map[string]any, error) {
	return e.update(id, func(d *editable, data map[string]any, _ DocType) error {
		if approvalStatus(data) == "missing" {
			return queryError("invalid_state", id+" has no recorded approval")
		}
		status, _ := data["status"].(string)
		if slices.Contains(historicalStates, status) {
			return queryError("invalid_state", id+" is "+status+"; its approval is history")
		}
		d.set("approval.revision", nullNode())
		d.set("approval.date", nullNode())
		d.set("status", stringNode("awaiting-approval"))
		return nil
	})
}

// Touch records a change to the body, such as plan progress.
func (e Editor) Touch(id string) (map[string]any, error) {
	return e.update(id, func(*editable, map[string]any, DocType) error { return nil })
}

// SetField changes a free-text field: the title, or a runbook owner.
func (e Editor) SetField(id, name, value string) (map[string]any, error) {
	return e.update(id, func(d *editable, _ map[string]any, t DocType) error {
		switch {
		case name == "title":
		case name == "owner" && t.Kind == "runbook":
		default:
			return queryError("invalid_arguments", "only title, and owner for runbooks, can be set; use the other docs commands for lifecycle fields")
		}
		if !singleLine(value) {
			return queryError("invalid_arguments", name+" must be a nonempty single line")
		}
		d.set(name, stringNode(value))
		return nil
	})
}

// Relate adds targets to a relation list, or removes them. field is related
// or supersedes.
func (e Editor) Relate(id, field string, targets []string, remove bool) (map[string]any, error) {
	if field != "related" && field != "supersedes" {
		return nil, queryError("invalid_arguments", "relation must be related or supersedes")
	}
	return e.update(id, func(d *editable, data map[string]any, t DocType) error {
		values := relationValues(data, field)
		if remove {
			values = slices.DeleteFunc(values, func(v string) bool { return slices.Contains(targets, v) })
		} else {
			related, supersedes := targets, []string(nil)
			if field == "supersedes" {
				related, supersedes = nil, targets
			}
			if err := e.checkTargets(id, t, related, supersedes); err != nil {
				return err
			}
			for _, target := range targets {
				if slices.Contains(relationValues(data, "related"), target) || slices.Contains(relationValues(data, "supersedes"), target) {
					return queryError("invalid_arguments", target+" is already listed in "+id)
				}
				values = append(values, target)
			}
		}
		d.set(field, listNode(values))
		return nil
	})
}

// RecordValidation records the outcome of operating a runbook. passed, partial
// and failed describe the current revision in environment; stale keeps the
// earlier evidence, and not-validated clears it.
func (e Editor) RecordValidation(id, state, environment string) (map[string]any, error) {
	return e.update(id, func(d *editable, data map[string]any, t DocType) error {
		if t.Kind != "runbook" {
			return queryError("invalid_arguments", "only runbooks record validation")
		}
		switch state {
		case "passed", "partial", "failed":
			if !singleLine(environment) {
				return queryError("invalid_arguments", state+" requires --environment naming where the runbook ran")
			}
			revision, _ := PositiveInt(data["revision"])
			d.set("validation.revision", intNode(revision))
			d.set("validation.date", dateNode(e.Today))
			d.set("validation.environment", stringNode(environment))
		case "stale":
			if _, ok := PositiveInt(asMap(data["validation"])["revision"]); !ok {
				return queryError("invalid_state", "stale needs earlier validation evidence; use not-validated")
			}
		case "not-validated":
			d.set("validation.revision", nullNode())
			d.set("validation.date", nullNode())
			d.set("validation.environment", nullNode())
		default:
			return queryError("invalid_arguments", "validation states are: "+strings.Join(validationStates, ", "))
		}
		d.set("validation.status", stringNode(state))
		return nil
	})
}

// update locates the document, applies change and sets updated. The file is
// written only if the document gains no validation error, so an agent cannot
// leave it inconsistent; errors it already had do not block the change.
func (e Editor) update(id string, change func(*editable, map[string]any, DocType) error) (map[string]any, error) {
	path, t, err := e.locate(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data, _, err := readDocument(path)
	if err != nil {
		return nil, queryError("invalid_document", relSlash(e.Root, path)+": "+err.Error())
	}
	document, err := parseEditable(raw)
	if err != nil {
		return nil, queryError("invalid_document", relSlash(e.Root, path)+": "+err.Error())
	}
	if err := change(document, data, t); err != nil {
		return nil, err
	}
	if created, ok := ISODate(data["created"]); ok && e.Today.Before(created) {
		return nil, queryError("invalid_date", "today precedes the creation date of "+id+"; check the system clock")
	}
	document.set("updated", dateNode(e.Today))
	content, err := document.bytes()
	if err != nil {
		return nil, err
	}
	suite := suiteOf(t)
	before := issueSet(suite.Inspect(e.Root), relSlash(e.Root, path))
	if err := writeAtomic(path, content); err != nil {
		return nil, err
	}
	var added []Issue
	for _, issue := range suite.Inspect(e.Root).Errors {
		if (issue.File == "" || issue.File == relSlash(e.Root, path)) && !before[issue.String()] {
			added = append(added, issue)
		}
	}
	if len(added) > 0 {
		if err := writeAtomic(path, raw); err != nil {
			return nil, err
		}
		return nil, &QueryError{Code: "invalid_change", Message: "the change would make " + id + " invalid; nothing was written", Issues: added}
	}
	return Get(e.Root, id, false, false)
}

func issueSet(result ValidationResult, rel string) map[string]bool {
	set := map[string]bool{}
	for _, issue := range result.Errors {
		if issue.File == "" || issue.File == rel {
			set[issue.String()] = true
		}
	}
	return set
}

// locate finds the one file of id in its type directory, valid or not.
func (e Editor) locate(id string) (string, DocType, error) {
	if !documentID.MatchString(id) {
		return "", DocType{}, queryError("invalid_arguments", "id must be a document ID")
	}
	for _, t := range documentTypes() {
		if !t.hasID(id) {
			continue
		}
		found, _ := discover(e.Root, t)
		var paths []string
		for _, r := range found {
			if strings.HasPrefix(filepath.Base(r.path), id+"-") {
				paths = append(paths, r.path)
			}
		}
		switch len(paths) {
		case 0:
			return "", t, queryError("not_found", "document not found: "+id)
		case 1:
			return paths[0], t, nil
		}
		return "", t, queryError("duplicate_id", id+" is used by several files; run harness check")
	}
	return "", DocType{}, queryError("not_found", "document not found: "+id)
}

// checkTargets verifies relations before they are written: every target must
// be another existing document, and replacements must share the type.
func (e Editor) checkTargets(id string, t DocType, related, supersedes []string) error {
	seen := map[string]bool{}
	for _, list := range [][]string{related, supersedes} {
		for _, target := range list {
			if target == id {
				return queryError("invalid_arguments", "a document cannot refer to itself")
			}
			if seen[target] {
				return queryError("invalid_arguments", "duplicate relation: "+target)
			}
			seen[target] = true
			path, targetType, err := e.locate(target)
			if err != nil || path == "" {
				return queryError("invalid_arguments", "relation target not found: "+target)
			}
			if slices.Contains(supersedes, target) && targetType.Kind != t.Kind {
				return queryError("invalid_arguments", "supersedes must reference the same type: "+target)
			}
		}
	}
	return nil
}

// requireApprovedReplacement applies the rule that check enforces, before writing.
func (e Editor) requireApprovedReplacement(id string, t DocType) error {
	records, _ := queryRecords(e.Root, []string{t.Kind})
	for _, r := range records {
		status, _ := r.data["status"].(string)
		governs := slices.Contains(t.Approved, status) || status == "superseded" || status == "retired"
		if slices.Contains(relationValues(r.data, "supersedes"), id) && governs && approvalStatus(r.data) != "missing" {
			return nil
		}
	}
	return queryError("approval_required", id+" can be superseded only after the human approves a replacement that lists it in supersedes")
}

func suiteOf(t DocType) *Suite {
	for _, s := range suites {
		if s.owns(t.Prefix) {
			return s
		}
	}
	return nil
}

// writeAtomic replaces path through a temporary file, so a reader never sees
// a partial document.
func writeAtomic(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".harness-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// slugify turns a title into a filename slug of at most six words.
func slugify(title string) string {
	folded := strings.NewReplacer("á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i", "ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u", "ñ", "n", "ç", "c").Replace(strings.ToLower(title))
	words := strings.FieldsFunc(folded, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') })
	if len(words) > 6 {
		words = words[:6]
	}
	if len(words) == 0 {
		return "document"
	}
	return strings.Join(words, "-")
}
