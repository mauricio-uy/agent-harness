// Package report carries the CLI's output as classified lines, so the
// packages that produce it never format for a terminal and the terminal
// never has to parse their text.
package report

import (
	"fmt"
	"io"
	"strings"
)

// Kind classifies one line of output.
type Kind int

const (
	Plain   Kind = iota
	Create       // a file was created
	Link         // a link was created
	Update       // a file was changed
	Record       // installation state was recorded
	Index        // a generated index is out of date or was rewritten
	Skip         // a file was left alone
	OK           // something already is as it should be
	Warn         // something needs the human's attention
	Error        // a check failed
	File         // the file the following errors belong to
	Section      // the start of a group of lines
	Heading      // a heading inside a group
	Pass         // a successful result line
	Fail         // a failed result line
)

// prefixes are the text labels written before a line of each kind.
var prefixes = map[Kind]string{
	Create: "CREATE ",
	Link:   "LINK   ",
	Update: "UPDATE ",
	Record: "RECORD ",
	Index:  "INDEX ",
	Skip:   "SKIP   ",
	OK:     "OK     ",
	Warn:   "WARN   ",
	Error:  "ERROR: ",
	File:   "FILE ",
}

// Label returns the text label of a kind, trimmed of its padding, or "".
func Label(kind Kind) string {
	return strings.TrimRight(prefixes[kind], " ")
}

// Sink receives output lines.
type Sink interface {
	Emit(kind Kind, text string)
}

// Emitf formats a line and sends it to sink.
func Emitf(sink Sink, kind Kind, format string, args ...any) {
	sink.Emit(kind, fmt.Sprintf(format, args...))
}

// Blank sends an empty line.
func Blank(sink Sink) { sink.Emit(Plain, "") }

// Text writes lines as plain text with their labels.
type Text struct{ W io.Writer }

// Line renders one line as plain text, without its newline.
func Line(kind Kind, text string) string {
	if kind == Section {
		return "== " + text + " =="
	}
	return prefixes[kind] + text
}

// Emit writes the line.
func (t Text) Emit(kind Kind, text string) {
	_, _ = io.WriteString(t.W, Line(kind, text)+"\n")
}

// Discard drops every line.
var Discard Sink = discard{}

type discard struct{}

func (discard) Emit(Kind, string) {}
