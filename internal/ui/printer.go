package ui

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	green   = lipgloss.NewStyle().Foreground(lipgloss.Green)
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Bold(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
	faint   = lipgloss.NewStyle().Faint(true)
	heading = lipgloss.NewStyle().Bold(true)

	sectionLine = regexp.MustCompile(`^== (.+) ==$`)
	scannedLine = regexp.MustCompile(`^Scanned \d+ Markdown file\(s\); found (\d+) error\(s\)\.$`)
)

// labels maps an output prefix to the style of the prefix and the action it
// counts in the summary; an empty action is not counted.
var labels = []struct {
	prefix string
	style  lipgloss.Style
	action string
	whole  bool // style the whole line, not only the prefix
}{
	{"CREATE ", green, "created", false},
	{"LINK ", cyan, "linked", false},
	{"UPDATE ", cyan, "updated", false},
	{"RECORD ", cyan, "", false},
	{"INDEX ", cyan, "", false},
	{"MOVE ", cyan, "", false},
	{"SKIP ", faint, "skipped", true},
	{"OK ", faint, "", true},
	{"WARN ", yellow, "warnings", false},
	{"ERROR:", red, "errors", false},
	{"FILE ", heading, "", true},
}

// Printer styles each complete line written to it and counts actions for a
// summary. Wrap its destination in a color profile writer, which strips the
// styling when the output is not a terminal or NO_COLOR is set.
type Printer struct {
	out     io.Writer
	pending []byte
	counts  map[string]int
}

// NewPrinter returns a Printer writing to out.
func NewPrinter(out io.Writer) *Printer {
	return &Printer{out: out, counts: map[string]int{}}
}

func (p *Printer) Write(b []byte) (int, error) {
	p.pending = append(p.pending, b...)
	for {
		i := bytes.IndexByte(p.pending, '\n')
		if i < 0 {
			return len(b), nil
		}
		line := string(p.pending[:i])
		p.pending = p.pending[i+1:]
		if _, err := io.WriteString(p.out, p.style(line)+"\n"); err != nil {
			return 0, err
		}
	}
}

// Flush writes any unterminated final line.
func (p *Printer) Flush() {
	if len(p.pending) > 0 {
		_, _ = io.WriteString(p.out, p.style(string(p.pending)))
		p.pending = nil
	}
}

func (p *Printer) style(line string) string {
	for _, l := range labels {
		if !strings.HasPrefix(line, l.prefix) {
			continue
		}
		if l.action != "" {
			p.counts[l.action]++
		}
		if l.whole {
			return l.style.Render(line)
		}
		label := strings.TrimRight(l.prefix, " ")
		return l.style.Render(label) + line[len(label):]
	}
	if match := sectionLine.FindStringSubmatch(line); match != nil {
		return heading.Render("▸ " + match[1])
	}
	if match := scannedLine.FindStringSubmatch(line); match != nil {
		if match[1] == "0" {
			return green.Render(line)
		}
		return red.Render(line)
	}
	switch {
	case line == "Next steps:":
		return heading.Render(line)
	case strings.HasPrefix(line, "Documentation checks failed"):
		return red.Render(line)
	}
	return line
}

// Summary describes the counted actions, such as "40 created · 2 skipped".
func (p *Printer) Summary() string {
	var parts []string
	for _, action := range []string{"created", "linked", "updated", "skipped", "warnings", "errors"} {
		if n := p.counts[action]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, action))
		}
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, " · ")
}

// Success renders a final status line.
func Success(message string) string { return green.Render("✓ " + message) }

// Warning renders a final status line for a result that needs attention.
func Warning(message string) string { return yellow.Render("! " + message) }

// Failure renders an error line.
func Failure(message string) string { return red.Render("✗ " + message) }
