package docs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Suites lists the document synchronizations in the order they run.
var Suites = []string{"plans", "specifications", "research", "runbooks"}

// SyncSuite runs one named synchronization.
func SyncSuite(name, root string, apply, check bool, out io.Writer) (int, error) {
	switch name {
	case "plans":
		return SyncPlans(root, apply, check, out), nil
	case "specifications":
		return Specifications.Sync(root, apply, check, out), nil
	case "research":
		return Research.Sync(root, apply, check, out), nil
	case "runbooks":
		return Runbooks.Sync(root, apply, check, out), nil
	}
	return 1, fmt.Errorf("unknown document suite %q; choose one of %v", name, Suites)
}

// Check runs every read-only validation: index synchronization, skill
// frontmatter, and local links. Every check runs even if an earlier one fails.
func Check(root string, links LinkReport, out io.Writer) int {
	failed := false
	for _, name := range Suites {
		fmt.Fprintf(out, "\n== %s ==\n", name)
		status, _ := SyncSuite(name, root, false, true, out)
		failed = failed || status != 0
	}
	fmt.Fprintln(out, "\n== skills ==")
	failed = CheckSkills(root, out) != 0 || failed
	fmt.Fprintln(out, "\n== links ==")
	count, errors := CheckLinks(root)
	failed = ReportLinks(count, errors, links, out) != 0 || failed
	if failed {
		fmt.Fprintln(out, "\nDocumentation checks failed. Fix the reported issues before retrying.")
		return 1
	}
	return 0
}

// CheckStaged runs Check against a disposable copy of the Git index, so
// unstaged edits neither hide nor cause failures. The working tree and index
// are not changed. GIT_INDEX_FILE is inherited, as Git can supply an alternate index.
func CheckStaged(root string, links LinkReport, out io.Writer) int {
	snapshot, err := os.MkdirTemp("", "harness-index-")
	if err != nil {
		fmt.Fprintf(out, "ERROR: cannot check staged documentation: %v\n", err)
		return 1
	}
	defer os.RemoveAll(snapshot)
	command := exec.Command("git", "-C", root, "checkout-index", "--all", "--prefix", filepath.ToSlash(snapshot)+"/")
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(out, "ERROR: cannot check staged documentation: %v\n%s", err, output)
		return 1
	}
	return Check(snapshot, links, out)
}
