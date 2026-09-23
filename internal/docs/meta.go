// Package docs validates harness documentation, regenerates its indexes, and
// checks local Markdown links.
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

	"go.yaml.in/yaml/v3"
)

// Date is a calendar date read from YAML frontmatter.
type Date struct{ time.Time }

// Timestamp is a YAML timestamp with a time component; it is never a valid date field.
type Timestamp struct{ time.Time }

var (
	isoPattern  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	lineBreaker = strings.NewReplacer("\r\n", "\n", "\r", "\n")
)

// ReadMetadata returns the YAML frontmatter of a Markdown file as plain values:
// map[string]any, []any, string, int64, float64, bool, Date, Timestamp, or nil.
func ReadMetadata(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(lineBreaker.Replace(strings.TrimPrefix(string(raw), "\ufeff")), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, errors.New("missing YAML frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, errors.New("unclosed YAML frontmatter")
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &node); err != nil {
		return nil, err
	}
	if node.Kind == 0 {
		return nil, errors.New("frontmatter must be a mapping")
	}
	value, err := decodeNode(&node)
	if err != nil {
		return nil, err
	}
	data, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("frontmatter must be a mapping")
	}
	return data, nil
}

// decodeNode rejects ambiguous YAML instead of silently using the last duplicate key.
func decodeNode(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return decodeNode(node.Content[0])
	case yaml.AliasNode:
		return decodeNode(node.Alias)
	case yaml.MappingNode:
		mapping := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
				return nil, errors.New("frontmatter keys must be strings")
			}
			if _, exists := mapping[key.Value]; exists {
				return nil, fmt.Errorf("duplicate YAML key: %s", key.Value)
			}
			value, err := decodeNode(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			mapping[key.Value] = value
		}
		return mapping, nil
	case yaml.SequenceNode:
		items := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := decodeNode(child)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		return items, nil
	case yaml.ScalarNode:
		switch node.ShortTag() {
		case "!!null":
			return nil, nil
		case "!!bool":
			var value bool
			err := node.Decode(&value)
			return value, err
		case "!!int":
			var value int64
			err := node.Decode(&value)
			return value, err
		case "!!float":
			var value float64
			err := node.Decode(&value)
			return value, err
		case "!!timestamp":
			if isoPattern.MatchString(node.Value) {
				parsed, err := time.Parse(time.DateOnly, node.Value)
				if err != nil {
					return nil, err
				}
				return Date{parsed}, nil
			}
			var value time.Time
			err := node.Decode(&value)
			return Timestamp{value}, err
		case "!!str", "!!binary":
			return node.Value, nil
		default:
			return nil, fmt.Errorf("unsupported YAML tag: %s", node.Tag)
		}
	}
	return nil, fmt.Errorf("unsupported YAML node at line %d", node.Line)
}

// ISODate accepts a YAML date or a quoted YYYY-MM-DD string naming a real date.
func ISODate(value any) (time.Time, bool) {
	switch v := value.(type) {
	case Date:
		return v.Time, true
	case string:
		if isoPattern.MatchString(v) {
			if parsed, err := time.Parse(time.DateOnly, v); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

// PositiveInt accepts only YAML integers greater than zero; booleans are not integers.
func PositiveInt(value any) (int64, bool) {
	v, ok := value.(int64)
	return v, ok && v > 0
}

var tableEscapes = regexp.MustCompile(`([\\` + "`" + `*_{}\[\]()#+.!|~-])`)

// TableText escapes Markdown and HTML syntax so a title cannot create links or columns.
func TableText(value string) string {
	value = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
	return tableEscapes.ReplaceAllString(value, `\$1`)
}

// QuotePath percent-encodes a slash-separated path, keeping unreserved characters and slashes.
func QuotePath(path string) string {
	var b strings.Builder
	for _, c := range []byte(path) {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("_.-~/", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func singleLine(value any) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "\r\n")
}

func formatDate(t time.Time) string { return t.Format(time.DateOnly) }

// within reports whether path, after resolving symbolic links, stays inside root.
func within(root, path string) bool {
	rel, err := filepath.Rel(resolve(root), resolve(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolve follows symbolic links in the longest existing prefix of path.
func resolve(path string) string {
	path = filepath.Clean(path)
	var rest []string
	for current := path; ; {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		rest = append([]string{filepath.Base(current)}, rest...)
		current = parent
	}
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// markdownFiles lists *.md files under dir without following linked directories.
func markdownFiles(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sortPaths(files)
	return files
}

func sortPaths(paths []string) {
	slices.SortFunc(paths, func(a, b string) int { return strings.Compare(filepath.ToSlash(a), filepath.ToSlash(b)) })
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// readText reads a generated file with universal newlines, as the comparison
// must not flag an index checked out with CRLF line endings.
func readText(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return lineBreaker.Replace(string(raw)), true
}

func writeText(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
