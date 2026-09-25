package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// LinkError is one broken local link. Its JSON form is the report contract.
type LinkError struct {
	Source      string `json:"source"`
	Line        int    `json:"line"`
	Destination string `json:"destination"`
	Reason      string `json:"reason"`
	Resolved    string `json:"resolved"`
	Suggestion  string `json:"suggestion"`
}

var (
	markdown  = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	uriScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// linkSources are the directories and root files whose Markdown the checker scans.
var (
	linkSourceDirs  = []string{"docs", ".agents", ".claude/commands", ".claude/skills", ".opencode/commands"}
	linkSourceFiles = []string{"README.md", "AGENTS.md", "CLAUDE.md"}
)

type link struct {
	destination string
	line        int
}

type parsed struct {
	links   []link
	anchors map[string]bool
}

func stripFrontmatter(source string) string {
	lines := strings.SplitAfter(source, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				// Blank lines preserve original source positions.
				return strings.Repeat("\n", i+1) + strings.Join(lines[i+1:], "")
			}
		}
	}
	return source
}

func parseDocument(path string) (parsed, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return parsed{}, err
	}
	source := []byte(stripFrontmatter(strings.TrimPrefix(string(raw), "\ufeff")))
	lineOf := lineIndex(source)
	result := parsed{anchors: map[string]bool{}}
	usedSlugs := map[string]bool{}
	document := markdown.Parser().Parse(text.NewReader(source))
	err = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Heading:
			base := headingSlug(headingText(n, source))
			slug := base
			for suffix := 1; usedSlugs[slug]; suffix++ {
				slug = fmt.Sprintf("%s-%d", base, suffix)
			}
			usedSlugs[slug] = true
			result.anchors[slug] = true
		case *ast.HTMLBlock:
			var content bytes.Buffer
			for i := 0; i < n.Lines().Len(); i++ {
				segment := n.Lines().At(i)
				content.Write(segment.Value(source))
			}
			if n.HasClosure() {
				content.Write(n.ClosureLine.Value(source))
			}
			scanHTML(content.String(), blockLine(n, lineOf), true, &result)
		case *ast.RawHTML:
			var content strings.Builder
			for i := 0; i < n.Segments.Len(); i++ {
				segment := n.Segments.At(i)
				content.Write(segment.Value(source))
			}
			line := blockLine(n, lineOf)
			if n.Segments.Len() > 0 {
				line = lineOf(n.Segments.At(0).Start)
			}
			scanHTML(content.String(), line, false, &result)
		case *ast.Link:
			result.links = append(result.links, link{string(n.Destination), inlineLine(n, lineOf)})
		case *ast.Image:
			result.links = append(result.links, link{string(n.Destination), inlineLine(n, lineOf)})
		}
		return ast.WalkContinue, nil
	})
	return result, err
}

func lineIndex(source []byte) func(int) int {
	var starts []int
	for i, c := range source {
		if c == '\n' {
			starts = append(starts, i)
		}
	}
	return func(offset int) int {
		line, _ := slices.BinarySearch(starts, offset)
		return line + 1
	}
}

// blockLine reports the first source line of the nearest block containing node.
func blockLine(node ast.Node, lineOf func(int) int) int {
	for n := node; n != nil; n = n.Parent() {
		if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
			return lineOf(n.Lines().At(0).Start)
		}
	}
	return 1
}

// inlineLine reports the line of an inline node's first text, falling back to its block.
func inlineLine(node ast.Node, lineOf func(int) int) int {
	line := 0
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*ast.Text); ok && entering {
			line = lineOf(t.Segment.Start)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if line == 0 {
		return blockLine(node, lineOf)
	}
	return line
}

func scanHTML(content string, line int, countLines bool, result *parsed) {
	tokenizer := html.NewTokenizer(strings.NewReader(content))
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			return
		}
		newlines := bytes.Count(tokenizer.Raw(), []byte("\n"))
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			name, more := tokenizer.TagName()
			tag := string(name)
			for more {
				var key, value []byte
				key, value, more = tokenizer.TagAttr()
				switch k := string(key); {
				case k == "href" || k == "src":
					result.links = append(result.links, link{string(value), line})
				case k == "id" || (tag == "a" && k == "name"):
					result.anchors[string(value)] = true
				}
			}
		}
		if countLines {
			line += newlines
		}
	}
}

func headingText(heading *ast.Heading, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(heading, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			value := string(t.Segment.Value(source))
			if _, code := t.Parent().(*ast.CodeSpan); !code {
				value = html.UnescapeString(unescapePunctuation(value))
			}
			b.WriteString(value)
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		case *ast.AutoLink:
			b.Write(t.Label(source))
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func unescapePunctuation(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) && strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", value[i+1]) >= 0 {
			i++
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

// headingSlug follows GitHub-style slugs for the documented plain-text heading convention.
func headingSlug(value string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(value) {
		switch {
		case c == ' ':
			b.WriteByte('-')
		case c == '-' || c == '_' || unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.IsMark(c):
			b.WriteRune(c)
		}
	}
	return b.String()
}

// unquote percent-decodes a URL component, leaving malformed escapes intact.
func unquote(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '%' && i+2 < len(value) {
			if decoded, err := strconv.ParseUint(value[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(decoded))
				i += 2
				continue
			}
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

type destination struct {
	external              bool
	path, query, fragment string
}

func splitDestination(value string) destination {
	var d destination
	if uriScheme.MatchString(value) || strings.HasPrefix(value, "//") {
		d.external = true
		return d
	}
	if i := strings.IndexByte(value, '#'); i >= 0 {
		value, d.fragment = value[:i], value[i+1:]
	}
	if i := strings.IndexByte(value, '?'); i >= 0 {
		value, d.query = value[:i], value[i+1:]
	}
	d.path = value
	return d
}

func linkSources(root string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, dir := range linkSourceDirs {
		for _, path := range markdownFiles(filepath.Join(root, filepath.FromSlash(dir))) {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	for _, name := range linkSourceFiles {
		path := filepath.Join(root, name)
		if isFile(path) && !seen[path] {
			paths = append(paths, path)
		}
	}
	sortPaths(paths)
	return paths
}

// CheckLinks scans every Markdown source and reports broken local links and
// fragments, suggesting a new destination for documents that moved.
func CheckLinks(root string) (int, []LinkError) {
	var errors []LinkError
	cache := map[string]parsed{}
	document := func(path string) (parsed, error) {
		if p, ok := cache[path]; ok {
			return p, nil
		}
		p, err := parseDocument(path)
		if err == nil {
			cache[path] = p
		}
		return p, err
	}
	documents := map[string][]string{}
	for _, path := range markdownFiles(filepath.Join(root, "docs")) {
		if id := documentPrefix.FindString(filepath.Base(path)); id != "" && !startsWithDigit(filepath.Base(path)[len(id):]) && within(root, path) {
			documents[id] = append(documents[id], path)
		}
	}
	sources := linkSources(root)
	if !isDir(filepath.Join(root, "docs")) {
		errors = append(errors, LinkError{Source: "docs", Line: 1, Reason: "documentation directory is missing"})
	}
	for _, source := range sources {
		name := relSlash(root, source)
		if !within(root, source) {
			errors = append(errors, LinkError{Source: name, Line: 1, Reason: "source resolves outside repository"})
			continue
		}
		p, err := document(source)
		if err != nil {
			errors = append(errors, LinkError{Source: name, Line: 1, Reason: fmt.Sprintf("cannot parse source: %v", err)})
			continue
		}
		for _, l := range p.links {
			d := splitDestination(l.destination)
			if d.external {
				continue
			}
			local := unquote(d.path)
			if strings.Contains(local, `\`) {
				errors = append(errors, LinkError{Source: name, Line: l.line, Destination: l.destination, Reason: "use forward slashes in local links"})
				continue
			}
			target := source
			switch {
			case strings.HasPrefix(local, "/"):
				target = filepath.Join(root, filepath.FromSlash(strings.TrimLeft(local, "/")))
			case local != "":
				target = filepath.Join(filepath.Dir(source), filepath.FromSlash(local))
			}
			if !within(root, target) {
				errors = append(errors, LinkError{Source: name, Line: l.line, Destination: l.destination, Reason: "target resolves outside repository", Resolved: resolve(target)})
				continue
			}
			resolved := relSlash(root, target)
			switch {
			case !exists(target):
				suggestion := ""
				if id := findDocumentID(local); id != "" && len(documents[id]) == 1 {
					if rel, err := filepath.Rel(filepath.Dir(source), documents[id][0]); err == nil {
						suggestion = QuotePath(filepath.ToSlash(rel))
						if d.query != "" {
							suggestion += "?" + d.query
						}
						if d.fragment != "" {
							suggestion += "#" + d.fragment
						}
					}
				}
				errors = append(errors, LinkError{name, l.line, l.destination, "missing target", resolved, suggestion})
			case d.fragment != "":
				anchorTarget := target
				if isDir(target) {
					anchorTarget = filepath.Join(target, "README.md")
				}
				if isFile(anchorTarget) && strings.EqualFold(filepath.Ext(anchorTarget), ".md") {
					if anchors, err := document(anchorTarget); err == nil && !anchors.anchors[unquote(d.fragment)] {
						errors = append(errors, LinkError{Source: name, Line: l.line, Destination: l.destination, Reason: "missing Markdown fragment", Resolved: resolved})
					}
				}
			}
		}
	}
	// Preserve repeated occurrences on different lines, avoiding duplicate parser findings.
	unique := map[LinkError]bool{}
	var result []LinkError
	for _, e := range errors {
		if !unique[e] {
			unique[e] = true
			result = append(result, e)
		}
	}
	slices.SortStableFunc(result, func(a, b LinkError) int {
		if c := strings.Compare(a.Source, b.Source); c != 0 {
			return c
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Destination, b.Destination)
	})
	return len(sources), result
}

func startsWithDigit(value string) bool { return value != "" && value[0] >= '0' && value[0] <= '9' }

// findDocumentID returns the first document ID not embedded in a longer identifier.
func findDocumentID(value string) string {
	for _, m := range documentAny.FindAllStringIndex(value, -1) {
		before := m[0] == 0 || !isAlnum(value[m[0]-1])
		after := m[1] == len(value) || !(value[m[1]] >= '0' && value[m[1]] <= '9')
		if before && after {
			return value[m[0]:m[1]]
		}
	}
	return ""
}

func isAlnum(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// LinkReport options control console format and report files.
type LinkReport struct {
	GitHub    bool
	ReportDir string
}

// LinkResult is the outcome of checking local links.
type LinkResult struct {
	Scanned int         `json:"scanned"`
	Errors  []LinkError `json:"errors"`
}

// InspectLinks checks every local link under root.
func InspectLinks(root string) LinkResult {
	count, errors := CheckLinks(root)
	if errors == nil {
		errors = []LinkError{}
	}
	return LinkResult{count, errors}
}

// ReportLinks prints the errors and writes the requested reports; it returns
// the exit status.
func ReportLinks(count int, errors []LinkError, options LinkReport, out report.Sink) int {
	previous := ""
	for _, e := range errors {
		if e.Source != previous {
			report.Blank(out)
			out.Emit(report.File, e.Source)
			previous = e.Source
		}
		line := fmt.Sprintf("  line %d: %q: %s; resolved=%q", e.Line, e.Destination, e.Reason, e.Resolved)
		if e.Suggestion != "" {
			line += fmt.Sprintf("; suggested=%q", e.Suggestion)
		}
		out.Emit(report.Plain, line)
	}
	report.Blank(out)
	outcome := report.Pass
	if len(errors) > 0 {
		outcome = report.Fail
	}
	report.Emitf(out, outcome, "Scanned %d Markdown file(s); found %d error(s).", count, len(errors))
	if options.GitHub {
		// GitHub may cap visible annotations; full reports and logs remain complete.
		for _, e := range errors[:min(len(errors), 50)] {
			message := fmt.Sprintf("%s: %s; resolved=%s", e.Destination, e.Reason, e.Resolved)
			if e.Suggestion != "" {
				message += "; suggested=" + e.Suggestion
			}
			report.Emitf(out, report.Plain, "::error file=%s,line=%d,title=Broken documentation link::%s",
				commandEscape(e.Source, true), e.Line, commandEscape(message, false))
		}
		if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
			if f, err := os.OpenFile(summary, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				_, _ = f.WriteString(markdownReport(count, errors, 30))
				_ = f.Close()
			}
		}
	}
	if options.ReportDir != "" {
		if err := WriteLinkReports(options.ReportDir, count, errors); err != nil {
			report.Emitf(out, report.Error, "%v", err)
			return 1
		}
	}
	if len(errors) > 0 {
		return 1
	}
	return 0
}

// WriteLinkReports writes links.json and links.md to dir.
func WriteLinkReports(dir string, count int, errors []LinkError) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if errors == nil {
		errors = []LinkError{}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{"scanned_files": count, "errors": errors}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "links.json"), buffer.Bytes(), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "links.md"), []byte(markdownReport(count, errors, 0)), 0o644)
}

func cell(value string) string {
	value = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
	return strings.NewReplacer("|", "&#124;", "\n", " ", "\r", " ", "`", "&#96;").Replace(value)
}

// markdownReport renders every error, or at most limit abbreviated entries when limit > 0.
func markdownReport(count int, errors []LinkError, limit int) string {
	lines := []string{"# Documentation Links", "", fmt.Sprintf("Scanned %d Markdown file(s); found %d error(s).", count, len(errors)), ""}
	visible := errors
	if limit > 0 {
		visible = errors[:min(len(errors), limit)]
	}
	if len(visible) > 0 {
		lines = append(lines, "| Source | Line | Destination | Problem | Resolved target | Suggested destination |",
			"| --- | --- | --- | --- | --- | --- |")
		for _, e := range visible {
			values := []string{e.Source, strconv.Itoa(e.Line), e.Destination, e.Reason, e.Resolved, e.Suggestion}
			for i, v := range values {
				if limit > 0 && len(v) > 180 {
					v = v[:180]
				}
				values[i] = cell(v)
			}
			lines = append(lines, "| "+strings.Join(values, " | ")+" |")
		}
	}
	if limit > 0 {
		lines = append(lines, "", "Summary entries may be abbreviated. The docs-link-report artifact contains every error "+
			"in links.json and links.md; the logs also contain the full list.")
	}
	return strings.Join(lines, "\n") + "\n"
}

func commandEscape(value string, property bool) string {
	value = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(value)
	if property {
		value = strings.NewReplacer(":", "%3A", ",", "%2C").Replace(value)
	}
	return value
}
