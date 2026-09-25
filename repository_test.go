package harness

import (
	"testing"

	"github.com/mauricio-uy/agent-harness/internal/docs"
)

// The repository root has no docs/ of its own; its README and AGENTS.md link
// into the payload, and those links must resolve.
func TestRepositoryLinks(t *testing.T) {
	_, errors := docs.CheckLinks(".")
	for _, e := range errors {
		if e.Source == "docs" {
			continue
		}
		t.Errorf("%s:%d: %s: %s", e.Source, e.Line, e.Destination, e.Reason)
	}
}
