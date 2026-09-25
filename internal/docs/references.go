package docs

import (
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// referencePath finds a path into docs/, optionally behind ./ or ../, that
// does not continue a longer path or URL.
var referencePath = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./~-])((?:\.\.?/)*docs/[A-Za-z0-9._~%/-]*)`)

// grepLine is one line of a file that may reference a document.
type grepLine struct {
	file string
	line int
	text string
}

// checkReferences checks the document IDs and docs/ paths that tracked files
// other than Markdown mention, such as code comments. It returns how many
// references it checked and the broken ones.
func checkReferences(root string, files tree) (int, []LinkError) {
	lines, err := files.grep(root, referencePattern())
	if err != nil {
		return 0, []LinkError{{Source: ".", Line: 1, Reason: "cannot search files for document references: " + err.Error()}}
	}
	documents := documentPaths(root)
	count := 0
	var found []LinkError
	for _, l := range lines {
		var paths [][2]int
		for _, m := range referencePath.FindAllStringSubmatchIndex(l.text, -1) {
			start := m[2]
			ref := strings.TrimRight(l.text[start:m[3]], ".")
			paths = append(paths, [2]int{start, start + len(ref)})
			// Documents are Markdown. Other paths into docs/, such as the
			// output folder of a documentation generator, are not references.
			if !strings.HasSuffix(ref, ".md") {
				continue
			}
			count++
			rel := ref
			for strings.HasPrefix(rel, "./") || strings.HasPrefix(rel, "../") {
				rel = rel[strings.IndexByte(rel, '/')+1:]
			}
			if files.exists(filepath.Join(root, filepath.FromSlash(rel))) {
				continue
			}
			suggestion := ""
			if id := findDocumentID(ref); len(documents[id]) == 1 {
				suggestion = relSlash(root, documents[id][0])
			}
			found = append(found, LinkError{l.file, l.line, ref, "missing target", rel, suggestion})
		}
		for _, m := range documentAny.FindAllStringIndex(l.text, -1) {
			if !isolatedID(l.text, m) || inside(paths, m[0]) {
				continue
			}
			count++
			id := l.text[m[0]:m[1]]
			switch matches := documents[id]; len(matches) {
			case 0:
				found = append(found, LinkError{Source: l.file, Line: l.line, Destination: id, Reason: "no document has this ID"})
			case 1:
			default:
				var rels []string
				for _, path := range matches {
					rels = append(rels, relSlash(root, path))
				}
				found = append(found, LinkError{Source: l.file, Line: l.line, Destination: id, Reason: "several documents have this ID", Resolved: strings.Join(rels, ", ")})
			}
		}
	}
	return count, found
}

// referencePattern is an extended regular expression, for git grep, matching
// any line that may hold a reference.
func referencePattern() string {
	var prefixes []string
	for _, t := range documentTypes() {
		prefixes = append(prefixes, t.Prefix)
	}
	return "(" + strings.Join(prefixes, "|") + ")-[0-9]{6}|docs/"
}

// isolatedID reports whether the match is a whole ID, not part of a longer word or number.
func isolatedID(text string, m []int) bool {
	return (m[0] == 0 || !isAlnum(text[m[0]-1])) && (m[1] == len(text) || !(text[m[1]] >= '0' && text[m[1]] <= '9'))
}

func inside(spans [][2]int, offset int) bool {
	for _, s := range spans {
		if offset >= s[0] && offset < s[1] {
			return true
		}
	}
	return false
}

// searched excludes Markdown, which the link checker reads, the installation
// record, which lists installed files rather than references, and files the
// project marks with the harness-ignore attribute in .gitattributes, such as
// tests with example IDs. Git applies attributes only inside a repository.
var searched = []string{":(exclude)*.md", ":(exclude).agents/harness.json", ":(exclude,attr:harness-ignore)"}

// stagedSplit is the number of files changed since staging above which the
// staged content is searched as a whole instead of file by file.
const stagedSplit = 500

// grepFiles searches the tracked files other than Markdown under dir, or the
// files Git would not ignore when dir is not a repository. With cached it
// searches the staged content. Without Git there is nothing to search.
func grepFiles(dir, pattern string, cached bool) ([]grepLine, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil
	}
	if !cached {
		if exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run() != nil {
			return runGrep(dir, pattern, []string{"--no-index", "--exclude-standard"}, append([]string{"."}, searched...))
		}
		return runGrep(dir, pattern, nil, append([]string{"."}, searched...))
	}
	// Reading every staged blob is slow in a large repository. A file that
	// has not changed since it was staged is read from the working tree, and
	// only the changed ones from the index.
	listing, err := exec.Command("git", "-C", dir, "diff", "--name-only", "--relative", "-z").Output()
	if err != nil {
		return nil, gitError(err)
	}
	var changed []string
	for _, name := range strings.Split(string(listing), "\x00") {
		if name != "" {
			changed = append(changed, ":(literal)"+name)
		}
	}
	if len(changed) > stagedSplit {
		return runGrep(dir, pattern, []string{"--cached"}, append([]string{"."}, searched...))
	}
	unchanged := append([]string{"."}, searched...)
	for _, name := range changed {
		unchanged = append(unchanged, ":(exclude,literal)"+strings.TrimPrefix(name, ":(literal)"))
	}
	lines, err := runGrep(dir, pattern, nil, unchanged)
	if err != nil || len(changed) == 0 {
		return lines, err
	}
	staged, err := runGrep(dir, pattern, []string{"--cached"}, append(changed, searched...))
	return append(lines, staged...), err
}

func runGrep(dir, pattern string, flags, pathspecs []string) ([]grepLine, error) {
	args := append([]string{"-C", dir, "grep", "--no-color", "-n", "-z", "-I", "-E", "-e", pattern}, flags...)
	args = append(append(args, "--"), pathspecs...)
	output, err := exec.Command("git", args...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil, nil // no match
	}
	if err != nil {
		return nil, gitError(err)
	}
	var lines []grepLine
	for _, raw := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(raw, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		number, _ := strconv.Atoi(parts[1])
		lines = append(lines, grepLine{parts[0], number, parts[2]})
	}
	return lines, nil
}
