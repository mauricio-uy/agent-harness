// Package install copies the harness into a project and adds what each chosen
// client needs, preferring links over copies so no skill is read twice.
package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mauricio-uy/agent-harness/internal/docs"
	"github.com/mauricio-uy/agent-harness/internal/report"
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

// state is what .agents/harness.json records about an installation.
type state struct {
	// Version is the version of the CLI that last installed or upgraded the payload.
	Version string   `json:"version,omitempty"`
	Clients []string `json:"clients"`
	// Files maps each installed payload file to the digest of the content the
	// harness wrote, so an upgrade can tell untouched files from changed ones.
	// Generated indexes and merged files are not recorded.
	Files map[string]string `json:"files,omitempty"`
}

// Installer applies the embedded payload to one project root.
type Installer struct {
	Root    string
	Payload fs.FS
	Out     report.Sink
	// Version is recorded in the installation state.
	Version string
	failed  bool
	// files collects the digests of the payload files in place.
	files map[string]string
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
	recorded, err := in.readState()
	if err != nil {
		return err
	}
	in.files = recorded.Files
	if err := in.copyTree("template"); err != nil {
		return err
	}
	for _, id := range clients {
		report.Blank(in.Out)
		in.Out.Emit(report.Section, id)
		if err := in.addClient(id); err != nil {
			return err
		}
	}
	if in.Version != "" {
		recorded.Version = in.Version
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
		report.Emitf(in.Out, report.Plain, "No recorded client needs links (%s: %v).", stateFile, recorded.Clients)
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
		in.Out.Emit(report.Plain, "Pi reads .agents/skills and AGENTS.md directly; nothing to add.")
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
		content, err := fs.ReadFile(in.Payload, name)
		if err != nil {
			return err
		}
		target := filepath.Join(in.Root, filepath.FromSlash(rel))
		if _, err := os.Lstat(target); err == nil {
			// A file that already matches the payload is adopted as installed.
			if current, err := os.ReadFile(target); err == nil && digest(current) == digest(content) {
				in.remember(rel, content)
			}
			report.Emitf(in.Out, report.Skip, "%s (exists)", rel)
			return nil
		}
		if err := in.writeFile(rel, content); err != nil {
			return err
		}
		in.Out.Emit(report.Create, rel)
		return nil
	})
}

// writeFile writes a payload file at rel and records its digest.
func (in *Installer) writeFile(rel string, content []byte) error {
	target := filepath.Join(in.Root, filepath.FromSlash(rel))
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
	in.remember(rel, content)
	return nil
}

// tracked reports whether an upgrade manages rel. Generated indexes belong to
// synchronization and merged files to their merge.
func tracked(rel string) bool {
	return !merged[rel] && !docs.IsGeneratedIndex(rel)
}

// remember records the digest of a payload file the project now holds.
func (in *Installer) remember(rel string, content []byte) {
	if !tracked(rel) {
		return
	}
	if in.files == nil {
		in.files = map[string]string{}
	}
	in.files[rel] = digest(content)
}

// digest identifies content regardless of its line endings, so a checkout
// that converts them does not look like a local change.
func digest(content []byte) string {
	sum := sha256.Sum256(bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
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
	if in.files != nil {
		s.Files = in.files
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(in.Root, filepath.FromSlash(stateFile))
	if current, err := os.ReadFile(target); err == nil && string(current) == string(raw)+"\n" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	report.Blank(in.Out)
	report.Emitf(in.Out, report.Record, "%s %v", stateFile, s.Clients)
	return nil
}

func clientOrder(id string) int {
	return slices.IndexFunc(Clients, func(c Client) bool { return c.ID == id })
}

func (in *Installer) warn(format string, args ...any) {
	in.failed = true
	report.Emitf(in.Out, report.Warn, format, args...)
}

func (in *Installer) result() error {
	if in.failed {
		return errors.New("finished with warnings that need manual action")
	}
	return nil
}

func (in *Installer) nextSteps(clients []string) {
	report.Blank(in.Out)
	in.Out.Emit(report.Heading, "Next steps:")
	in.Out.Emit(report.Plain, "  - Ask your agent to complete docs/overview.md (Purpose and Structure).")
	setup := hookSetup(in.Root)
	switch {
	case len(setup.others()) > 0:
		in.Out.Emit(report.Warn, setup.managersWarning())
	case setup.hooksPath != ".githooks":
		in.Out.Emit(report.Plain, "  - Enable the pre-commit check in each clone: git config core.hooksPath .githooks")
	}
	if slices.Contains(clients, "claude-code") {
		in.Out.Emit(report.Plain, "  - Skill links are not committed; other clones run: harness link")
	}
	in.Out.Emit(report.Plain, "  - Check the setup of a clone at any time: harness doctor")
}
