package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

type fixture struct {
	t    *testing.T
	root string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{t, t.TempDir()}
}

func (f *fixture) path(rel string) string { return filepath.Join(f.root, filepath.FromSlash(rel)) }

func (f *fixture) write(rel, content string) string {
	f.t.Helper()
	path := f.path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *fixture) read(rel string) string {
	f.t.Helper()
	raw, err := os.ReadFile(f.path(rel))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(raw)
}

func (f *fixture) exists(rel string) bool { return exists(f.path(rel)) }

// field is one ordered frontmatter entry.
type field struct {
	key   string
	value any
}

func frontmatter(fields []field) string {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, f := range fields {
		var value yaml.Node
		if err := value.Encode(f.value); err != nil {
			panic(err)
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: f.key}, &value)
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		panic(err)
	}
	return "---\n" + string(raw) + "---\n\n# Example\n"
}

// with replaces or appends fields, keeping the original order.
func with(base []field, overrides ...field) []field {
	result := append([]field(nil), base...)
	for _, o := range overrides {
		replaced := false
		for i := range result {
			if result[i].key == o.key {
				result[i] = o
				replaced = true
			}
		}
		if !replaced {
			result = append(result, o)
		}
	}
	return result
}

func approval(revision any, date any) map[string]any {
	return map[string]any{"revision": revision, "date": date}
}

type result struct {
	status int
	out    string
}

func (r result) contains(t *testing.T, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(r.out, part) {
			t.Errorf("output lacks %q:\n%s", part, r.out)
		}
	}
}

func (f *fixture) sync(name string, apply, check bool) result {
	var out bytes.Buffer
	status, err := SyncSuite(name, f.root, apply, check, report.Text{W: &out})
	if err != nil {
		f.t.Fatal(err)
	}
	return result{status, out.String()}
}

func expectStatus(t *testing.T, r result, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("status %d, want %d:\n%s", r.status, want, r.out)
	}
}

func (f *fixture) links(options LinkReport) (result, []LinkError) {
	var out bytes.Buffer
	count, errors := CheckLinks(f.root)
	status := ReportLinks(count, errors, options, report.Text{W: &out})
	return result{status, out.String()}, errors
}

func readLinkReport(t *testing.T, dir string) []LinkError {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "links.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Errors []LinkError `json:"errors"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	return report.Errors
}

// index reads the generated index of one state of a document type.
func (f *fixture) index(folder, state string) string {
	f.t.Helper()
	return f.read("docs/" + folder + "/" + state + ".md")
}

func number(n int) string { return fmt.Sprintf("%06d", n) }
