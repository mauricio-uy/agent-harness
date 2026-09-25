package docs

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

var (
	planStates         = []string{"draft", "awaiting-approval", "approved", "in-progress", "completed", "cancelled", "superseded"}
	approvedPlanStates = []string{"approved", "in-progress", "completed"}
	planType           = DocType{"plan", "PLAN", "plans", "Plans", planStates, ""}
)

// Plans covers implementation plans. Unlike specifications, a plan records its
// replacement with superseded_by and several states require current approval.
var Plans = &Suite{
	Name:      "plans",
	Types:     []DocType{planType},
	validate:  func(r record, _ DocType, _ *Suite) []string { return validatePlan(r.path, r.data) },
	relations: planRelations,
}

// planRelations requires every superseded_by to name an existing plan.
func planRelations(records []record, catalog map[string][]record) []string {
	var errors []string
	for _, r := range records {
		if target, ok := r.data["superseded_by"].(string); ok && len(catalog[target]) == 0 {
			errors = append(errors, fmt.Sprintf("%s: superseded_by references missing ID %s", r.rel, target))
		}
	}
	return errors
}

func validatePlan(path string, data map[string]any) []string {
	var errors []string
	for _, key := range []string{"id", "title", "status", "created", "updated", "revision", "approval"} {
		if _, ok := data[key]; !ok {
			errors = append(errors, "missing field: "+key)
		}
	}
	identifier, _ := data["id"].(string)
	if !planType.hasID(identifier) {
		errors = append(errors, "id must match PLAN-000001")
	}
	if match := documentFile.FindStringSubmatch(filepath.Base(path)); match == nil || match[1] != identifier {
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
		case !ok || !planType.hasID(target):
			errors = append(errors, "superseded_by must be a valid plan ID")
		case target == identifier:
			errors = append(errors, "superseded_by must not refer to the same plan")
		}
	}
	return errors
}
