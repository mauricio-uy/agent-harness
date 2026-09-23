package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mauricio-uy/agent-harness/internal/install"
)

func TestPrinterStylesKnownLinesAndCountsActions(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter(&out)
	fmt.Fprint(p, "CREATE AGENTS.md\nCREATE docs/README.md\nSKIP   opencode.json (exists)\n")
	fmt.Fprint(p, "LINK   .claude/skills/x -> .agents/skills/x (symlink)\nWARN   something\nplain line")
	p.Flush()
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
	fmt.Fprintln(p, "== links ==")
	fmt.Fprintln(p, "Scanned 3 Markdown file(s); found 0 error(s).")
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
		if !strings.Contains(view, c.Name) || !strings.Contains(view, c.Summary) {
			t.Errorf("selector lacks %s:\n%s", c.Name, view)
		}
	}
	if strings.Contains(view, "toggle") {
		t.Errorf("keys belong only in the help bar, not in the description:\n%s", view)
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
