package docs

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// ignoreFile lists, with .gitignore syntax, the project files whose links and
// document references the checks skip.
const ignoreFile = ".harnessignore"

// ignoreRule is one pattern of the ignore file.
type ignoreRule struct {
	pattern  *regexp.Regexp
	anchored bool // matches the whole path from the root, not just a name
	dirOnly  bool
}

type ignoreRules []ignoreRule

// loadIgnore reads the ignore file under root. A missing file ignores
// nothing. Lines it cannot use are returned as errors, so a pattern never
// silently fails to apply.
func loadIgnore(root string) (ignoreRules, []LinkError) {
	raw, err := os.ReadFile(filepath.Join(root, ignoreFile))
	if err != nil {
		return nil, nil
	}
	var rules ignoreRules
	var errors []LinkError
	for i, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t")
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "!"):
			errors = append(errors, LinkError{Source: ignoreFile, Line: i + 1, Destination: line, Reason: "negated patterns are not supported"})
			continue
		}
		rule := ignoreRule{dirOnly: strings.HasSuffix(line, "/")}
		line = strings.TrimSuffix(line, "/")
		rule.anchored = strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			errors = append(errors, LinkError{Source: ignoreFile, Line: i + 1, Destination: "/", Reason: "pattern matches nothing"})
			continue
		}
		pattern, err := regexp.Compile("^" + globExpression(line) + "$")
		if err != nil {
			errors = append(errors, LinkError{Source: ignoreFile, Line: i + 1, Destination: line, Reason: "invalid pattern"})
			continue
		}
		rule.pattern = pattern
		rules = append(rules, rule)
	}
	return rules, errors
}

// globExpression translates a .gitignore glob into a regular expression:
// ** crosses folders, while *, ? and [...] stay within one name.
func globExpression(glob string) string {
	var out strings.Builder
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; {
		case strings.HasPrefix(glob[i:], "**/"):
			out.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			out.WriteString(".*")
			i++
		case c == '*':
			out.WriteString("[^/]*")
		case c == '?':
			out.WriteString("[^/]")
		case c == '[':
			end := strings.IndexByte(glob[i+1:], ']')
			if end < 0 {
				out.WriteString(`\[`)
				continue
			}
			class := glob[i+1 : i+1+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			out.WriteString("[" + class + "]")
			i += end + 1
		default:
			out.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return out.String()
}

// ignores reports whether rel, a file path relative to the root with forward
// slashes, or any folder above it matches a rule.
func (rules ignoreRules) ignores(rel string) bool {
	if len(rules) == 0 {
		return false
	}
	for current, isDir := rel, false; current != "." && current != "/"; current, isDir = path.Dir(current), true {
		for _, r := range rules {
			if r.dirOnly && !isDir {
				continue
			}
			candidate := current
			if !r.anchored {
				candidate = path.Base(current)
			}
			if r.pattern.MatchString(candidate) {
				return true
			}
		}
	}
	return false
}
