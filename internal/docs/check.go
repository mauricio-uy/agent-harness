package docs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// SyncSuite runs one named synchronization.
func SyncSuite(name, root string, apply, check bool, out report.Sink) (int, error) {
	s := suiteNamed(name)
	if s == nil {
		return 1, fmt.Errorf("unknown document suite %q; choose one of %v", name, Suites)
	}
	return s.Sync(root, apply, check, out), nil
}

// CheckResult is the outcome of every read-only validation. Its JSON form is
// the contract of `harness check --format json`.
type CheckResult struct {
	OK     bool                  `json:"ok"`
	Suites map[string]SyncResult `json:"suites"`
	Skills SkillResult           `json:"skills"`
	Links  LinkResult            `json:"links"`
}

// Inspect runs every read-only validation: index synchronization, skill
// frontmatter, and local links. Every check runs even if an earlier one fails.
func Inspect(root string) CheckResult {
	result := CheckResult{Suites: map[string]SyncResult{}, Skills: InspectSkills(root), Links: InspectLinks(root)}
	result.OK = len(result.Skills.Errors) == 0 && len(result.Links.Errors) == 0
	for _, s := range suites {
		r := s.Inspect(root)
		result.Suites[s.Name] = r
		result.OK = result.OK && len(r.Errors) == 0 && len(r.StaleIndexes) == 0
	}
	return result
}

// Report writes the result section by section; it returns the exit status.
func (r CheckResult) Report(links LinkReport, out report.Sink) int {
	for _, name := range Suites {
		report.Blank(out)
		out.Emit(report.Section, name)
		r.Suites[name].Report(out, false)
	}
	report.Blank(out)
	out.Emit(report.Section, "skills")
	r.Skills.Report(out)
	report.Blank(out)
	out.Emit(report.Section, "links")
	status := ReportLinks(r.Links.Scanned, r.Links.Errors, links, out)
	if !r.OK || status != 0 {
		report.Blank(out)
		out.Emit(report.Fail, "Documentation checks failed. Fix the reported issues before retrying.")
		return 1
	}
	return 0
}

// Check runs every read-only validation and reports it; it returns the exit status.
func Check(root string, links LinkReport, out report.Sink) int {
	return Inspect(root).Report(links, out)
}

// InspectStaged runs Inspect against a disposable copy of the Git index, so
// unstaged edits neither hide nor cause failures. The working tree and index
// are not changed. GIT_INDEX_FILE is inherited, as Git can supply an alternate index.
func InspectStaged(root string) (CheckResult, error) {
	snapshot, err := os.MkdirTemp("", "harness-index-")
	if err != nil {
		return CheckResult{}, fmt.Errorf("cannot check staged documentation: %w", err)
	}
	defer os.RemoveAll(snapshot)
	command := exec.Command("git", "-C", root, "checkout-index", "--all", "--prefix", filepath.ToSlash(snapshot)+"/")
	if output, err := command.CombinedOutput(); err != nil {
		return CheckResult{}, fmt.Errorf("cannot check staged documentation: %v\n%s", err, output)
	}
	return Inspect(snapshot), nil
}

// CheckStaged runs Check against the Git index; it returns the exit status.
func CheckStaged(root string, links LinkReport, out report.Sink) int {
	result, err := InspectStaged(root)
	if err != nil {
		out.Emit(report.Error, err.Error())
		return 1
	}
	return result.Report(links, out)
}
