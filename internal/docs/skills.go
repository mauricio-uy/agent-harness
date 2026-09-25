package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// skillFields are the frontmatter fields defined by the Agent Skills specification
// (https://agentskills.io/specification). Client-specific behavior belongs in
// client files the installer adds, never in the shared skill.
var (
	skillFields = []string{"name", "description", "license", "compatibility", "metadata", "allowed-tools"}
	skillName   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

func validateSkill(dir string) []string {
	path := filepath.Join(dir, "SKILL.md")
	if !isFile(path) {
		return []string{"missing SKILL.md"}
	}
	data, err := ReadMetadata(path)
	if err != nil {
		return []string{err.Error()}
	}
	var errors []string
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if !slices.Contains(skillFields, key) {
			errors = append(errors, "unsupported frontmatter field: "+key)
		}
	}
	name, _ := data["name"].(string)
	switch {
	case !skillName.MatchString(name) || len(name) > 64:
		errors = append(errors, "name must be 1-64 lowercase letters, digits, and single hyphens")
	case name != filepath.Base(dir):
		errors = append(errors, "name must match the skill directory")
	}
	if description, ok := data["description"].(string); !ok || description == "" || utf8.RuneCountInString(description) > 1024 {
		errors = append(errors, "description must be 1-1024 characters")
	}
	for field, limit := range map[string]int{"compatibility": 500, "license": 0, "allowed-tools": 0} {
		value, present := data[field]
		if !present {
			continue
		}
		s, ok := value.(string)
		if !ok || s == "" || (limit > 0 && utf8.RuneCountInString(s) > limit) {
			errors = append(errors, fmt.Sprintf("%s must be a nonempty string", field)+limitNote(limit))
		}
	}
	if value, present := data["metadata"]; present {
		mapping, ok := value.(map[string]any)
		for _, item := range mapping {
			if _, isString := item.(string); !isString {
				ok = false
			}
		}
		if !ok {
			errors = append(errors, "metadata must map string keys to string values")
		}
	}
	return errors
}

func limitNote(limit int) string {
	if limit == 0 {
		return ""
	}
	return fmt.Sprintf(" of at most %d characters", limit)
}

// SkillResult is the outcome of validating the skills.
type SkillResult struct {
	Count  int     `json:"count"`
	Errors []Issue `json:"errors"`
}

// InspectSkills validates every skill under .agents/skills against the Agent Skills specification.
func InspectSkills(root string) SkillResult {
	result := SkillResult{Errors: []Issue{}}
	base := filepath.Join(root, ".agents", "skills")
	entries, err := os.ReadDir(base)
	if err != nil {
		result.Errors = append(result.Errors, Issue{".agents/skills", err.Error()})
		return result
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		result.Count++
		for _, e := range validateSkill(filepath.Join(base, entry.Name())) {
			result.Errors = append(result.Errors, Issue{".agents/skills/" + entry.Name(), e})
		}
	}
	return result
}

// Report writes the result and reports whether every skill is valid.
func (r SkillResult) Report(out report.Sink) bool {
	for _, e := range r.Errors {
		out.Emit(report.Error, e.String())
	}
	if len(r.Errors) > 0 {
		report.Emitf(out, report.Plain, "%d skill(s); %d error(s).", r.Count, len(r.Errors))
		return false
	}
	report.Emitf(out, report.Plain, "%d skill(s); frontmatter follows the Agent Skills specification.", r.Count)
	return true
}

// CheckSkills validates the skills and reports the result; it returns the exit status.
func CheckSkills(root string, out report.Sink) int {
	if InspectSkills(root).Report(out) {
		return 0
	}
	return 1
}
