// Package install copies the harness into a project and adds what each chosen
// client needs, preferring links over copies so no skill is read twice.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Client is a product that loads Agent Skills, in the specification's terms.
type Client struct{ ID, Name, Summary string }

// Clients lists the supported clients in the order they are offered.
var Clients = []Client{
	{"claude-code", "Claude Code", "links each skill into .claude/skills; write-plan gets a manual-only adapter"},
	{"codex", "Codex", "adds agents/openai.yaml invocation policies to each skill"},
	{"opencode", "OpenCode", "denies automatic write-plan loading in opencode.json and adds a /write-plan command"},
	{"pi", "Pi", "reads .agents/skills and AGENTS.md directly; nothing to add"},
}

const stateFile = ".agents/harness.json"

type state struct {
	Clients []string `json:"clients"`
}

// Installer applies the embedded payload to one project root.
type Installer struct {
	Root    string
	Payload fs.FS
	Out     io.Writer
	failed  bool
}

// ParseClients validates a comma-separated client list; "none" or "" selects none.
func ParseClients(value string) ([]string, error) {
	var ids []string
	for _, item := range strings.Split(value, ",") {
		id := strings.TrimSpace(strings.ToLower(item))
		if id == "" || id == "none" {
			continue
		}
		if !slices.ContainsFunc(Clients, func(c Client) bool { return c.ID == id }) {
			return nil, fmt.Errorf("unknown client %q; supported clients: %s", id, clientIDs())
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func clientIDs() string {
	ids := make([]string, len(Clients))
	for i, c := range Clients {
		ids[i] = c.ID
	}
	return strings.Join(ids, ", ")
}

// Init installs the base harness and the files each client needs. Existing
// files are never overwritten; they are reported and left for the human.
func (in *Installer) Init(clients []string) error {
	if err := in.copyTree("template"); err != nil {
		return err
	}
	for _, id := range clients {
		fmt.Fprintf(in.Out, "\n== %s ==\n", id)
		if err := in.addClient(id); err != nil {
			return err
		}
	}
	recorded, err := in.readState()
	if err != nil {
		return err
	}
	for _, id := range clients {
		if !slices.Contains(recorded.Clients, id) {
			recorded.Clients = append(recorded.Clients, id)
		}
	}
	slices.SortFunc(recorded.Clients, func(a, b string) int { return clientOrder(a) - clientOrder(b) })
	if err := in.writeState(recorded); err != nil {
		return err
	}
	in.nextSteps(recorded.Clients)
	return in.result()
}

// Link recreates the links of every recorded client. Links are local to each
// clone and not committed, so every clone runs this once.
func (in *Installer) Link() error {
	recorded, err := in.readState()
	if err != nil {
		return err
	}
	if !slices.Contains(recorded.Clients, "claude-code") {
		fmt.Fprintf(in.Out, "No recorded client needs links (%s: %v).\n", stateFile, recorded.Clients)
		return nil
	}
	if err := in.linkClaudeSkills(); err != nil {
		return err
	}
	return in.result()
}

func (in *Installer) addClient(id string) error {
	switch id {
	case "claude-code":
		if err := in.copyTree("template-clients/claude-code"); err != nil {
			return err
		}
		return in.linkClaudeSkills()
	case "codex":
		return in.copyTree("template-clients/codex")
	case "opencode":
		if err := in.copyTree("template-clients/opencode"); err != nil {
			return err
		}
		return in.mergeOpenCodeConfig()
	case "pi":
		fmt.Fprintln(in.Out, "Pi reads .agents/skills and AGENTS.md directly; nothing to add.")
		return nil
	}
	return fmt.Errorf("unknown client %q", id)
}

// merged lists payload files that are merged into an existing file rather than copied.
var merged = map[string]bool{"opencode.json": true}

// copyTree copies payload files under dir to the same relative paths in the project.
func (in *Installer) copyTree(dir string) error {
	return fs.WalkDir(in.Payload, dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(name, dir+"/")
		if merged[rel] {
			return nil
		}
		target := filepath.Join(in.Root, filepath.FromSlash(rel))
		if _, err := os.Lstat(target); err == nil {
			fmt.Fprintf(in.Out, "SKIP   %s (exists)\n", rel)
			return nil
		}
		content, err := fs.ReadFile(in.Payload, name)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if path.Dir(rel) == ".githooks" {
			mode = 0o755
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, mode); err != nil {
			return err
		}
		fmt.Fprintf(in.Out, "CREATE %s\n", rel)
		return nil
	})
}

func (in *Installer) readState() (state, error) {
	var s state
	raw, err := os.ReadFile(filepath.Join(in.Root, filepath.FromSlash(stateFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("%s: %w", stateFile, err)
	}
	return s, nil
}

func (in *Installer) writeState(s state) error {
	if s.Clients == nil {
		s.Clients = []string{}
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(in.Root, filepath.FromSlash(stateFile))
	if current, err := os.ReadFile(target); err == nil && string(current) == string(raw)+"\n" {
		return nil
	}
	if err := os.WriteFile(target, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(in.Out, "\nRECORD %s %v\n", stateFile, s.Clients)
	return nil
}

func clientOrder(id string) int {
	return slices.IndexFunc(Clients, func(c Client) bool { return c.ID == id })
}

func (in *Installer) warn(format string, args ...any) {
	in.failed = true
	fmt.Fprintf(in.Out, "WARN   "+format+"\n", args...)
}

func (in *Installer) result() error {
	if in.failed {
		return errors.New("finished with warnings that need manual action")
	}
	return nil
}

func (in *Installer) nextSteps(clients []string) {
	fmt.Fprintln(in.Out, "\nNext steps:")
	fmt.Fprintln(in.Out, "  - Ask your agent to complete docs/overview.md (Purpose and Structure).")
	fmt.Fprintln(in.Out, "  - Enable the pre-commit check in each clone: git config core.hooksPath .githooks")
	if slices.Contains(clients, "claude-code") {
		fmt.Fprintln(in.Out, "  - Skill links are not committed; other clones run: harness link")
	}
}
