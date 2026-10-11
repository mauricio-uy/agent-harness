package docs

import "slices"

// Neighbor summarizes a document linked to the one a graph describes.
type Neighbor struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Path   string `json:"path"`
}

// Graph lists the direct relations of one document in both directions.
// referenced_by and superseded_by show what a change to it affects.
type Graph struct {
	Neighbor
	Related      []Neighbor `json:"related"`
	ReferencedBy []Neighbor `json:"referenced_by"`
	Supersedes   []Neighbor `json:"supersedes"`
	SupersededBy []Neighbor `json:"superseded_by"`
	// Unresolved lists relation targets that match no valid document.
	Unresolved []string `json:"unresolved,omitempty"`
}

// RelationGraph reads every valid document and returns the graph around id.
func RelationGraph(root, id string) (Graph, error) {
	if !documentID.MatchString(id) {
		return Graph{}, queryError("invalid_arguments", "id must be a document ID")
	}
	source, cleanup, err := queryRoot(root, false)
	if err != nil {
		return Graph{}, err
	}
	defer cleanup()
	records, _ := queryRecords(source, nil)
	byID := map[string]record{}
	for _, r := range records {
		byID[r.data["id"].(string)] = r
	}
	self, ok := byID[id]
	if !ok {
		return Graph{}, queryError("not_found", "no valid document "+id+"; run harness docs get "+id+" for details")
	}
	g := Graph{Neighbor: neighbor(self), Related: []Neighbor{}, ReferencedBy: []Neighbor{}, Supersedes: []Neighbor{}, SupersededBy: []Neighbor{}}
	for _, relation := range []struct {
		field string
		list  *[]Neighbor
	}{{"related", &g.Related}, {"supersedes", &g.Supersedes}} {
		list := relation.list
		for _, target := range relationValues(self.data, relation.field) {
			if r, ok := byID[target]; ok {
				*list = append(*list, neighbor(r))
			} else {
				g.Unresolved = append(g.Unresolved, target)
			}
		}
	}
	for _, r := range records {
		if slices.Contains(relationValues(r.data, "related"), id) {
			g.ReferencedBy = append(g.ReferencedBy, neighbor(r))
		}
		if slices.Contains(relationValues(r.data, "supersedes"), id) {
			g.SupersededBy = append(g.SupersededBy, neighbor(r))
		}
	}
	return g, nil
}

func neighbor(r record) Neighbor {
	text := func(key string) string { s, _ := r.data[key].(string); return s }
	return Neighbor{ID: text("id"), Type: text("type"), Title: text("title"), Status: text("status"), Path: r.rel}
}
