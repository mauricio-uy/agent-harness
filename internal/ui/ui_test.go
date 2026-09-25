package ui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mauricio-uy/agent-harness/internal/install"
	"github.com/mauricio-uy/agent-harness/internal/report"
)

func TestPrinterStylesKnownLinesAndCountsActions(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter(&out)
	p.Emit(report.Create, "AGENTS.md")
	p.Emit(report.Create, "docs/README.md")
	p.Emit(report.Skip, "opencode.json (exists)")
	p.Emit(report.Link, ".claude/skills/x -> .agents/skills/x (symlink)")
	p.Emit(report.Warn, "something")
	p.Emit(report.Plain, "plain line")
	styled := out.String()
	if !strings.Contains(styled, "\x1b[") {
		t.Fatal("known prefixes must be styled")
	}
	plain := ansi.Strip(styled)
	for _, line := range []string{"CREATE AGENTS.md", "SKIP   opencode.json (exists)", "LINK   .claude/skills/x", "plain line"} {
		if !strings.Contains(plain, line) {
			t.Errorf("text must survive styling: %q in\n%s", line, plain)
		}
	}
	if got := p.Summary(); got != "2 created · 1 linked · 1 skipped · 1 warnings" {
		t.Fatalf("summary %q", got)
	}
}

func TestPrinterHighlightsCheckResults(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter(&out)
	p.Emit(report.Section, "links")
	p.Emit(report.Pass, "Scanned 3 Markdown file(s); found 0 error(s).")
	if plain := ansi.Strip(out.String()); !strings.Contains(plain, "▸ links") || !strings.Contains(plain, "found 0 error(s)") {
		t.Fatalf("unexpected output %q", plain)
	}
	if NewPrinter(&out).Summary() != "no changes" {
		t.Fatal("an empty run has no changes")
	}
}

func TestClientSelectorListsEveryClientWithItsKeys(t *testing.T) {
	var selected []string
	field := clientField(&selected)
	field.Focus()
	view := ansi.Strip(field.View())
	for _, c := range install.Clients {
		if !strings.Contains(view, c.Name) {
			t.Errorf("selector lacks %s:\n%s", c.Name, view)
		}
	}
	// Only the names: no summaries of what each client adds, no description.
	for _, extra := range []string{".claude", "opencode.json", ".agents", "toggle"} {
		if strings.Contains(view, extra) {
			t.Errorf("the selector must show only the client names, found %q:\n%s", extra, view)
		}
	}
	field.WithKeyMap(keyMap())
	var help []string
	for _, binding := range field.KeyBinds() {
		help = append(help, binding.Help().Key+" "+binding.Help().Desc)
	}
	if got := strings.Join(help, " · "); !strings.Contains(got, "space toggle") || strings.Contains(got, "x toggle") {
		t.Errorf("the help bar must name space as the toggle key: %s", got)
	}
	t.Log("\n" + view + "\n" + strings.Join(help, " · "))
}

func press(field *huh.MultiSelect[string], keys ...tea.KeyPressMsg) {
	for _, k := range keys {
		model, _ := field.Update(k)
		*field = *model.(*huh.MultiSelect[string])
	}
}

func text(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func TestClientSelectorTogglesWithSpaceAndFilters(t *testing.T) {
	var selected []string
	field := clientField(&selected)
	field.WithKeyMap(keyMap())
	field.Focus()
	down := tea.KeyPressMsg{Code: tea.KeyDown}
	press(field, text(' '), down, down, text(' '))
	if got := field.GetValue().([]string); strings.Join(got, ",") != "claude-code,opencode" {
		t.Fatalf("space must toggle the highlighted clients, got %v", got)
	}
	press(field, text('/'), text('c'), text('o'), text('d'), text('e'), text('x'), tea.KeyPressMsg{Code: tea.KeyEnter}, text(' '))
	if got := field.GetValue().([]string); !strings.Contains(strings.Join(got, ","), "codex") {
		t.Fatalf("filtering must narrow the list to Codex, got %v\n%s", got, ansi.Strip(field.View()))
	}
}

// Every kind renders to the same text as the plain sink once styling is
// stripped, except sections, which the terminal marks with an arrow.
func TestPrinterKeepsThePlainTextOfEveryKind(t *testing.T) {
	for kind := report.Plain; kind <= report.Fail; kind++ {
		var styled, plain bytes.Buffer
		NewPrinter(&styled).Emit(kind, "text")
		report.Text{W: &plain}.Emit(kind, "text")
		want := plain.String()
		if kind == report.Section {
			want = "▸ text\n"
		}
		if got := ansi.Strip(styled.String()); got != want {
			t.Errorf("kind %d: got %q, want %q", kind, got, want)
		}
	}
}
