package install

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mauricio-uy/agent-harness/internal/report"
)

// Upgrade brings an installation up to the embedded payload. A file is
// replaced only when it still holds what the harness installed; a file the
// project changed is reported and kept. Generated indexes are left to
// synchronization. It writes only with apply.
func (in *Installer) Upgrade(apply bool) error {
	if _, err := os.Stat(filepath.Join(in.Root, filepath.FromSlash(stateFile))); err != nil {
		return errors.New("no installation found in " + stateFile + "; run harness init first")
	}
	recorded, err := in.readState()
	if err != nil {
		return err
	}
	in.files = map[string]string{}
	for rel, sum := range recorded.Files {
		in.files[rel] = sum
	}
	type write struct {
		rel     string
		content []byte
	}
	var writes []write
	installed := map[string]bool{}
	for _, dir := range in.payloadDirs(recorded.Clients) {
		err := fs.WalkDir(in.Payload, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			rel := strings.TrimPrefix(name, dir+"/")
			if merged[rel] {
				return nil
			}
			installed[rel] = true
			content, err := fs.ReadFile(in.Payload, name)
			if err != nil {
				return err
			}
			current, err := os.ReadFile(filepath.Join(in.Root, filepath.FromSlash(rel)))
			switch {
			case errors.Is(err, fs.ErrNotExist):
				in.Out.Emit(report.Create, rel)
				writes = append(writes, write{rel, content})
			case err != nil:
				return err
			case digest(current) == digest(content):
				in.remember(rel, content)
			case !tracked(rel):
			case in.files[rel] == digest(current):
				in.Out.Emit(report.Update, rel)
				writes = append(writes, write{rel, content})
			default:
				report.Emitf(in.Out, report.Skip, "%s (changed in this project; not replaced)", rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	var gone []string
	for rel := range in.files {
		if !installed[rel] {
			gone = append(gone, rel)
		}
	}
	slices.Sort(gone)
	for _, rel := range gone {
		report.Emitf(in.Out, report.Skip, "%s (no longer installed by the harness; left in place)", rel)
		delete(in.files, rel)
	}
	if len(writes) == 0 {
		in.Out.Emit(report.Plain, "The installed files are up to date.")
	}
	if !apply {
		if len(writes) > 0 {
			report.Blank(in.Out)
			in.Out.Emit(report.Plain, "Preview only. Run harness upgrade --apply to write these changes.")
		}
		return nil
	}
	for _, w := range writes {
		if err := in.writeFile(w.rel, w.content); err != nil {
			return err
		}
	}
	for _, id := range recorded.Clients {
		switch id {
		case "claude-code":
			err = in.linkClaudeSkills()
		case "opencode":
			err = in.mergeOpenCodeConfig()
		}
		if err != nil {
			return err
		}
	}
	if in.Version != "" {
		recorded.Version = in.Version
	}
	if err := in.writeState(recorded); err != nil {
		return err
	}
	report.Blank(in.Out)
	in.Out.Emit(report.Heading, "Next steps:")
	in.Out.Emit(report.Plain, "  - Regenerate the indexes: harness sync --apply")
	return in.result()
}

// payloadDirs lists the payload directories an installation with clients uses.
func (in *Installer) payloadDirs(clients []string) []string {
	dirs := []string{"template"}
	for _, id := range clients {
		dir := path.Join("template-clients", id)
		if _, err := fs.Stat(in.Payload, dir); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}
