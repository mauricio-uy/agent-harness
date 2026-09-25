package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

var (
	green   = lipgloss.NewStyle().Foreground(lipgloss.Green)
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Cyan)
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Yellow).Bold(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
	faint   = lipgloss.NewStyle().Faint(true)
	heading = lipgloss.NewStyle().Bold(true)
)

// actions names what each kind counts in the summary.
var actions = map[report.Kind]string{
	report.Create: "created",
	report.Link:   "linked",
	report.Update: "updated",
	report.Skip:   "skipped",
	report.Warn:   "warnings",
	report.Error:  "errors",
}

// Printer styles each line by its kind and counts actions for a summary.
// Wrap its destination in a color profile writer, which strips the styling
// when the output is not a terminal or NO_COLOR is set.
type Printer struct {
	out    io.Writer
	counts map[string]int
}

// NewPrinter returns a Printer writing to out.
func NewPrinter(out io.Writer) *Printer {
	return &Printer{out: out, counts: map[string]int{}}
}

// Emit writes one styled line.
func (p *Printer) Emit(kind report.Kind, text string) {
	if action := actions[kind]; action != "" {
		p.counts[action]++
	}
	_, _ = io.WriteString(p.out, render(kind, text)+"\n")
}

func render(kind report.Kind, text string) string {
	line := report.Line(kind, text)
	switch kind {
	case report.Create:
		return label(green, kind, line)
	case report.Link, report.Update, report.Record, report.Index:
		return label(cyan, kind, line)
	case report.Warn:
		return label(yellow, kind, line)
	case report.Error:
		return label(red, kind, line)
	case report.Skip, report.OK:
		return faint.Render(line)
	case report.File, report.Heading:
		return heading.Render(line)
	case report.Section:
		return heading.Render("▸ " + text)
	case report.Pass:
		return green.Render(line)
	case report.Fail:
		return red.Render(line)
	}
	return line
}

// label styles only the label at the start of line.
func label(style lipgloss.Style, kind report.Kind, line string) string {
	name := report.Label(kind)
	return style.Render(name) + line[len(name):]
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
