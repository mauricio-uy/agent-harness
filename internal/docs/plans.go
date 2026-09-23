package docs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	activePlanStates = []string{"draft", "awaiting-approval", "approved", "in-progress"}
	activePlanTitles = map[string]string{
		"draft": "Draft", "awaiting-approval": "Awaiting Approval", "approved": "Approved", "in-progress": "In Progress",
	}
	planStates         = append(slices.Clone(activePlanStates), "completed", "cancelled", "superseded")
	approvedPlanStates = []string{"approved", "in-progress", "completed"}
	planID             = regexp.MustCompile(`^PLAN-[0-9]{6}$`)
	planFile           = regexp.MustCompile(`^(PLAN-[0-9]{6})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
)

func validatePlan(path string, data map[string]any) []string {
	var errors []string
	for _, key := range []string{"id", "title", "status", "created", "updated", "revision", "approval"} {
		if _, ok := data[key]; !ok {
			errors = append(errors, "missing field: "+key)
		}
	}
	identifier, _ := data["id"].(string)
	if !planID.MatchString(identifier) {
		errors = append(errors, "id must match PLAN-000001")
	}
	if match := planFile.FindStringSubmatch(filepath.Base(path)); match == nil || match[1] != identifier {
		errors = append(errors, "filename must contain the matching ID and a lowercase hyphenated slug")
	}
	if !singleLine(data["title"]) {
		errors = append(errors, "title must be a nonempty single-line string")
	}
	status, _ := data["status"].(string)
	if !slices.Contains(planStates, status) {
		sorted := slices.Clone(planStates)
		slices.Sort(sorted)
		errors = append(errors, "status must be one of: "+strings.Join(sorted, ", "))
	}
	created, createdOK := ISODate(data["created"])
	updated, updatedOK := ISODate(data["updated"])
	if !createdOK {
		errors = append(errors, "created must be a valid YYYY-MM-DD date")
	}
	if !updatedOK {
		errors = append(errors, "updated must be a valid YYYY-MM-DD date")
	}
	if createdOK && updatedOK && updated.Before(created) {
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
		errors = append(errors, "approval must contain revision and date (both null or both populated)")
	} else {
		approvedValue, when := approval["revision"], approval["date"]
		approved, approvedOK := PositiveInt(approvedValue)
		if (approvedValue == nil) != (when == nil) {
			errors = append(errors, "approval.revision and approval.date must be populated together")
		}
		if approvedValue != nil {
			if !approvedOK {
				errors = append(errors, "approval.revision must be a positive integer or null")
			} else if revisionOK && approved > revision {
				errors = append(errors, "approval.revision must not exceed revision")
			}
		}
		if when != nil {
			if parsed, ok := ISODate(when); !ok {
				errors = append(errors, "approval.date must be a valid YYYY-MM-DD date")
			} else if createdOK && updatedOK && (parsed.Before(created) || parsed.After(updated)) {
				errors = append(errors, "approval.date must fall between created and updated")
			}
		}
		if slices.Contains(approvedPlanStates, status) && (!approvedOK || !revisionOK || approved != revision || when == nil) {
			errors = append(errors, status+" requires approval of the current revision")
		}
	}
	replacement, present := data["superseded_by"]
	if status == "superseded" || (present && replacement != nil) {
		target, ok := replacement.(string)
		switch {
		case !ok || !planID.MatchString(target):
			errors = append(errors, "superseded_by must be a valid plan ID")
		case target == identifier:
			errors = append(errors, "superseded_by must not refer to the same plan")
		}
	}
	return errors
}

type plan struct {
	path, rel string
	data      map[string]any
}

func planIndexContent(state string, plans []plan) string {
	lines := []string{"# " + activePlanTitles[state] + " Plans", "", "| ID | Plan | Status | Updated |", "| --- | --- | --- | --- |"}
	var matching []plan
	for _, p := range plans {
		if p.data["status"] == state {
			matching = append(matching, p)
		}
	}
	slices.SortStableFunc(matching, func(a, b plan) int { return strings.Compare(fmt.Sprint(a.data["id"]), fmt.Sprint(b.data["id"])) })
	for _, p := range matching {
		updated, _ := ISODate(p.data["updated"])
		lines = append(lines, fmt.Sprintf("| %v | [%s](%s) | %s | %s |",
			p.data["id"], TableText(fmt.Sprint(p.data["title"])), QuotePath(filepath.Base(p.path)), state, formatDate(updated)))
	}
	if len(matching) == 0 {
		lines = append(lines, "", "No plans in this state.")
	}
	return strings.Join(lines, "\n") + "\n"
}

// SyncPlans validates plans, moves each into the directory for its status, and
// regenerates the active-state indexes. Any validation error aborts before the first write.
func SyncPlans(root string, apply, check bool, out io.Writer) int {
	base := filepath.Join(root, "docs", "plans")
	if !isDir(base) || !within(root, base) {
		fmt.Fprintln(out, "ERROR: docs/plans must be a directory inside the repository")
		return 1
	}
	var errors []string
	var plans []plan
	identifiers := map[string][]string{}
	var order []string
	for _, path := range markdownFiles(base) {
		if filepath.Base(path) == "README.md" {
			continue
		}
		rel := relSlash(root, path)
		if !within(base, path) || isSymlink(path) {
			errors = append(errors, rel+": plan must be a regular file inside docs/plans")
			continue
		}
		data, err := ReadMetadata(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		plans = append(plans, plan{path, rel, data})
		for _, e := range validatePlan(path, data) {
			errors = append(errors, rel+": "+e)
		}
		if identifier, ok := data["id"].(string); ok {
			if identifiers[identifier] == nil {
				order = append(order, identifier)
			}
			identifiers[identifier] = append(identifiers[identifier], rel)
		}
	}
	for _, identifier := range order {
		if paths := identifiers[identifier]; len(paths) > 1 {
			errors = append(errors, fmt.Sprintf("duplicate ID %s: %s", identifier, strings.Join(paths, ", ")))
		}
	}

	type move struct{ source, target string }
	var moves []move
	for _, p := range plans {
		if target, ok := p.data["superseded_by"].(string); ok && identifiers[target] == nil {
			errors = append(errors, fmt.Sprintf("%s: superseded_by references missing ID %s", p.rel, target))
		}
		status, _ := p.data["status"].(string)
		if !slices.Contains(planStates, status) {
			continue
		}
		target := filepath.Join(base, status, filepath.Base(p.path))
		switch {
		case !within(base, target):
			errors = append(errors, relSlash(root, target)+": destination resolves outside docs/plans")
		case filepath.Clean(p.path) != target:
			if exists(target) {
				errors = append(errors, relSlash(root, target)+": destination collision")
			}
			moves = append(moves, move{p.path, target})
		}
	}
	for _, state := range planStates {
		directory := filepath.Join(base, state)
		index := filepath.Join(directory, "README.md")
		if isFile(directory) || !within(base, directory) {
			errors = append(errors, relSlash(root, directory)+": invalid state directory")
		}
		if isSymlink(index) || notRegular(index) {
			errors = append(errors, relSlash(root, index)+": index must be a regular file")
		}
	}
	if len(errors) > 0 {
		return reportErrors(out, errors)
	}

	type update struct{ path, content string }
	var updates []update
	for _, state := range activePlanStates {
		path := filepath.Join(base, state, "README.md")
		content := planIndexContent(state, plans)
		if current, ok := readText(path); !ok || current != content {
			updates = append(updates, update{path, content})
		}
	}
	for _, m := range moves {
		fmt.Fprintf(out, "MOVE %s -> %s\n", relSlash(root, m.source), relSlash(root, m.target))
	}
	for _, u := range updates {
		fmt.Fprintln(out, "INDEX "+relSlash(root, u.path))
	}
	if apply {
		// Metadata and destinations have all been checked before the first write.
		for _, m := range moves {
			if err := os.MkdirAll(filepath.Dir(m.target), 0o755); err != nil {
				fmt.Fprintf(out, "ERROR: %v\n", err)
				return 1
			}
			if err := os.Rename(m.source, m.target); err != nil {
				fmt.Fprintf(out, "ERROR: %v\n", err)
				return 1
			}
		}
		for _, u := range updates {
			if err := writeText(u.path, u.content); err != nil {
				fmt.Fprintf(out, "ERROR: %v\n", err)
				return 1
			}
		}
	}
	fmt.Fprintf(out, "%d plan(s); %d move(s); %d index update(s). %s\n", len(plans), len(moves), len(updates), modeLabel(apply))
	if check && len(moves)+len(updates) > 0 {
		return 1
	}
	return 0
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isFile reports a path that exists but is not a directory.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func notRegular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.Mode().IsRegular()
}
