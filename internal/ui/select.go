// Package ui renders the CLI's interactive prompts and styled output.
package ui

import (
	"errors"
	"fmt"
	"os"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/mauricio-uy/agent-harness/internal/install"
)

// ErrCancelled reports that the user left the prompt without confirming.
var ErrCancelled = errors.New("cancelled; nothing was written")

// SelectClients asks which clients to configure with a filterable multi-select.
// Set HARNESS_ACCESSIBLE=1 for plain prompts that screen readers can follow.
func SelectClients() ([]string, error) {
	var selected []string
	form := huh.NewForm(huh.NewGroup(clientField(&selected))).
		WithTheme(huh.ThemeFunc(huh.ThemeCharm)).
		WithKeyMap(keyMap()).
		WithAccessible(os.Getenv("HARNESS_ACCESSIBLE") != "")
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	return selected, nil
}

func clientField(selected *[]string) *huh.MultiSelect[string] {
	summary := lipgloss.NewStyle().Faint(true)
	options := make([]huh.Option[string], len(install.Clients))
	for i, c := range install.Clients {
		options[i] = huh.NewOption(fmt.Sprintf("%-12s %s", c.Name, summary.Render(c.Summary)), c.ID)
	}
	return huh.NewMultiSelect[string]().
		Title("Which clients should the harness configure?").
		// Keys are listed only in the help bar under the list, so they cannot disagree.
		Description("The base installation always goes to .agents/ and docs/.").
		Options(options...).
		Filterable(true).
		// The title, description, and every client stay visible without scrolling.
		Height(len(options) + 3).
		Value(selected)
}

// keyMap names space as the toggle key in the help bar; both space and x still
// toggle. Esc clears a filter, so only ctrl+c cancels.
func keyMap() *huh.KeyMap {
	keys := huh.NewDefaultKeyMap()
	keys.MultiSelect.Toggle = key.NewBinding(key.WithKeys("space", "x"), key.WithHelp("space", "toggle"))
	return keys
}
