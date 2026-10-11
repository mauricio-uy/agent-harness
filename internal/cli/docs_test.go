package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mauricio-uy/agent-harness/internal/docs"
)

func docsCLI(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status := Run(args, IO{Out: &out, Err: &errOut})
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("stdout must be JSON: %v\n%s\nstderr: %s", err, out.String(), errOut.String())
	}
	return status, result, errOut.String()
}

func TestDocumentationCLIJSONContractAndInterspersedGetFlags(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "research"), 0o755)
	raw := "---\nid: RES-000001\ntype: research\ntitle: Cache\nstatus: draft\ncreated: 2026-09-20\nupdated: 2026-09-20\nrevision: 1\napproval: {revision: null, date: null}\nrelated: []\n---\n\n# Findings\n"
	os.WriteFile(filepath.Join(root, "docs", "research", "RES-000001-cache.md"), []byte(raw), 0o644)
	status, result, stderr := docsCLI(t, "docs", "list", "--root", root, "--type", "research", "--limit", "1")
	if status != 0 || stderr != "" || result["total"] != float64(1) {
		t.Fatalf("%d %v %s", status, result, stderr)
	}
	for _, args := range [][]string{{"docs", "get", "RES-000001", "--root", root, "--body"}, {"docs", "get", "--root", root, "RES-000001", "--body"}, {"docs", "get", "--body", "RES-000001", "--root", root}} {
		status, result, stderr = docsCLI(t, args...)
		if status != 0 || stderr != "" || !strings.Contains(result["body"].(string), "Findings") {
			t.Fatalf("%d %v %s", status, result, stderr)
		}
	}
	for _, args := range [][]string{{"docs", "list", "--not-a-flag"}, {"docs", "list", "--limit", "wrong"}, {"docs", "list", "--limit"}, {"docs", "get", "--root", root, "RES-999999"}, {"docs", "get", "--root", root}} {
		status, result, _ = docsCLI(t, args...)
		if status != 1 || result["error"] == nil {
			t.Fatalf("%d %v", status, result)
		}
	}
}

func TestDocumentationCLIUsesAllSharedQueryOptions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "research")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 2; n++ {
		raw := fmt.Sprintf("---\nid: RES-%06d\ntype: research\ntitle: Cache %d\nstatus: approved\ncreated: 2026-09-20\nupdated: 2026-09-%d\nrevision: 2\napproval: {revision: 1, date: 2026-09-20}\nrelated: [ADR-000001]\n---\n\n# Cache findings\n", n, n, 19+n)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("RES-%06d-cache.md", n)), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "core.autocrlf", "false"}, {"add", "."}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	// The staged query must ignore this invalid working-tree edit.
	if err := os.WriteFile(filepath.Join(dir, "RES-000001-cache.md"), []byte("missing frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := docs.QueryOptions{
		Type: "research", Status: "approved,draft", Query: "cache", RelatedTo: "ADR-000001", Approval: "stale",
		UpdatedSince: "2026-09-20", UpdatedUntil: "2026-09-21", Fields: "id,title,status,path,approval",
		Sort: "updated", Order: "desc", Limit: 1, Offset: 1, Staged: true,
	}
	status, actual, stderr := docsCLI(t, "docs", "list", "--root", root,
		"--type", options.Type, "--status", options.Status, "--query", options.Query,
		"--related-to", options.RelatedTo, "--approval", options.Approval,
		"--updated-since", options.UpdatedSince, "--updated-until", options.UpdatedUntil,
		"--fields", options.Fields, "--sort", options.Sort, "--order", options.Order,
		"--limit", "1", "--offset", "1", "--staged")
	expected, err := docs.List(root, options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if status != 0 || stderr != "" || !reflect.DeepEqual(actual, decoded) || actual["total"] != float64(2) {
		t.Fatalf("CLI options differ from shared query options: %d %v %s", status, actual, stderr)
	}
	status, actual, stderr = docsCLI(t, "docs", "get", "RES-000001", "--root", root, "--body", "--staged")
	if status != 0 || stderr != "" || !strings.Contains(actual["body"].(string), "Cache findings") {
		t.Fatalf("get did not use staged options: %d %v %s", status, actual, stderr)
	}
}

func TestDocumentationHelpComesFromTheParameterDefinitions(t *testing.T) {
	for _, command := range []string{"list", "get"} {
		var out, errOut bytes.Buffer
		if status := Run([]string{"docs", command, "--help"}, IO{Out: &out, Err: &errOut}); status != 0 || errOut.Len() > 0 {
			t.Fatalf("%d %s", status, errOut.String())
		}
		parameters := docs.ListParameters
		if command == "get" {
			parameters = docs.GetParameters
		}
		for _, p := range parameters {
			if !strings.Contains(out.String(), "-"+p.Name) {
				t.Errorf("%s missing %s", command, p.Name)
			}
		}
	}
	var out, errOut bytes.Buffer
	if status := Run([]string{"help", "docs", "list"}, IO{Out: &out, Err: &errOut}); status != 2 {
		t.Fatal("the redundant help alias must not be supported")
	}
}

func TestDocumentationQueriesFindTheProjectFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".agents"), 0o755)
	os.WriteFile(filepath.Join(root, ".agents", "harness.json"), []byte(`{"clients":[]}`), 0o644)
	os.MkdirAll(filepath.Join(root, "docs", "research"), 0o755)
	raw := "---\nid: RES-000001\ntype: research\ntitle: Cache\nstatus: draft\ncreated: 2026-09-20\nupdated: 2026-09-20\nrevision: 1\napproval: {revision: null, date: null}\nrelated: []\n---\n\n# Findings\n"
	os.WriteFile(filepath.Join(root, "docs", "research", "RES-000001-cache.md"), []byte(raw), 0o644)
	t.Chdir(filepath.Join(root, "docs", "research"))
	if status, result, stderr := docsCLI(t, "docs", "list"); status != 0 || result["total"] != float64(1) {
		t.Fatalf("the installation above the working directory is the root: %d %v %s", status, result, stderr)
	}
	// An explicit root is used as given.
	if status, result, _ := docsCLI(t, "docs", "list", "--root", "."); status != 0 || result["total"] != float64(0) {
		t.Fatalf("explicit root: %d %v", status, result)
	}
}

func TestDocumentationWriteCommandsFollowTheLifecycle(t *testing.T) {
	root := t.TempDir()
	install := Run([]string{"init", "--root", root, "--clients", "none"}, IO{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	if install != 0 {
		t.Fatal("init failed")
	}
	now = func() time.Time { return time.Date(2026, 10, 7, 23, 30, 0, 0, time.Local) }
	defer func() { now = time.Now }()
	steps := []struct {
		args   []string
		status int
		check  func(map[string]any) bool
	}{
		{[]string{"new", "adr", "--title", "Use SQLite"}, 0, func(r map[string]any) bool { return r["id"] == "ADR-000001" && r["updated"] == "2026-10-07" }},
		{[]string{"new", "plan", "--title", "Adopt SQLite", "--related", "ADR-000001"}, 0, func(r map[string]any) bool { return r["path"] == "docs/plans/PLAN-000001-adopt-sqlite.md" }},
		{[]string{"status", "PLAN-000001", "approved"}, 1, func(r map[string]any) bool { return errorCode(r) == "approval_required" }},
		{[]string{"approve", "PLAN-000001"}, 0, func(r map[string]any) bool { return r["status"] == "approved" }},
		{[]string{"status", "PLAN-000001", "in-progress"}, 0, func(r map[string]any) bool { return r["status"] == "in-progress" }},
		{[]string{"touch", "PLAN-000001"}, 0, func(r map[string]any) bool { return r["revision"] == float64(1) }},
		{[]string{"set", "PLAN-000001", "title", "Adopt SQLite storage"}, 0, func(r map[string]any) bool { return r["title"] == "Adopt SQLite storage" }},
		{[]string{"graph", "ADR-000001"}, 0, func(r map[string]any) bool {
			by := r["referenced_by"].([]any)
			return len(by) == 1 && by[0].(map[string]any)["id"] == "PLAN-000001"
		}},
		{[]string{"relate", "PLAN-000001", "ADR-000001", "--remove"}, 0, func(r map[string]any) bool { return len(r["related"].([]any)) == 0 }},
		{[]string{"revise", "PLAN-000001"}, 0, func(r map[string]any) bool { return r["status"] == "draft" && r["revision"] == float64(2) }},
		{[]string{"validation", "PLAN-000001", "passed"}, 1, func(r map[string]any) bool { return errorCode(r) == "invalid_arguments" }},
		{[]string{"status", "PLAN-000001"}, 1, func(r map[string]any) bool { return errorCode(r) == "invalid_arguments" }},
		{[]string{"list", "extra"}, 1, func(r map[string]any) bool { return errorCode(r) == "invalid_arguments" }},
		{[]string{"unknown"}, 1, func(r map[string]any) bool { return errorCode(r) == "invalid_arguments" }},
	}
	for _, step := range steps {
		status, result, stderr := docsCLI(t, append(append([]string{"docs"}, step.args...), "--root", root)...)
		if status != step.status || stderr != "" || !step.check(result) {
			t.Fatalf("%v: %d %v %s", step.args, status, result, stderr)
		}
	}
	var out bytes.Buffer
	if status := Run([]string{"check", "--root", root}, IO{Out: &out, Err: &out}); status != 0 {
		t.Fatalf("documents written by the CLI must pass check:\n%s", out.String())
	}
}

func errorCode(result map[string]any) string {
	e, _ := result["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// Skills tell agents which commands to run. Every harness command and flag
// they mention must exist, so an agent never follows a broken instruction.
func TestSkillsMentionOnlyExistingCommandsAndFlags(t *testing.T) {
	command := regexp.MustCompile("`harness ([a-z]+)(?: ([a-z]+))?([^`]*)`")
	flagName := regexp.MustCompile(`--([a-z-]+)`)
	files, err := filepath.Glob(filepath.Join("..", "..", "template", ".agents", "skills", "*", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	references, _ := filepath.Glob(filepath.Join("..", "..", "template", ".agents", "skills", "*", "references", "*.md"))
	files = append(files, references...)
	files = append(files, filepath.Join("..", "..", "template", "AGENTS.md"), filepath.Join("..", "..", "template", "docs", "README.md"))
	known := map[string]bool{"init": true, "link": true, "doctor": true, "docs": true, "check": true, "date": true, "version": true}
	checked := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range command.FindAllStringSubmatch(string(raw), -1) {
			checked++
			if !known[match[1]] {
				t.Errorf("%s: unknown command harness %s", file, match[1])
				continue
			}
			if match[1] != "docs" || match[2] == "" {
				continue
			}
			sub, ok := docsCommands[match[2]]
			if !ok {
				t.Errorf("%s: unknown command harness docs %s", file, match[2])
				continue
			}
			flags := flag.NewFlagSet("", flag.ContinueOnError)
			flags.String("root", "", "")
			sub.flags(flags)
			for _, f := range flagName.FindAllStringSubmatch(match[3], -1) {
				if flags.Lookup(f[1]) == nil && f[1] != "help" {
					t.Errorf("%s: harness docs %s has no --%s", file, match[2], f[1])
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no harness commands found in the skills")
	}
}
