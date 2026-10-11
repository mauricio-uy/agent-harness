package docs

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// editable is a document whose frontmatter can change without touching the
// Markdown body. Only the edited fields change; order and comments remain.
type editable struct {
	mapping *yaml.Node
	body    []byte // everything after the closing delimiter, byte for byte
	bom     bool
	newline string
}

func parseEditable(raw []byte) (*editable, error) {
	e := &editable{newline: "\n"}
	if rest, ok := bytes.CutPrefix(raw, []byte("\ufeff")); ok {
		e.bom, raw = true, rest
	}
	if bytes.Contains(raw, []byte("\r\n")) {
		e.newline = "\r\n"
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, errors.New("missing YAML frontmatter")
	}
	end := strings.Index(text[3:], "\n---\n")
	closing := len("\n---\n")
	if end < 0 && strings.HasSuffix(text, "\n---") {
		end, closing = len(text)-3-len("\n---"), len("\n---")
	}
	if end < 0 {
		return nil, errors.New("unclosed YAML frontmatter")
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(text[4:3+end+1]), &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("frontmatter must be a mapping")
	}
	e.mapping = document.Content[0]
	e.body = []byte(strings.ReplaceAll(text[3+end+closing:], "\n", e.newline))
	return e, nil
}

func (e *editable) bytes() ([]byte, error) {
	var yamlText bytes.Buffer
	encoder := yaml.NewEncoder(&yamlText)
	encoder.SetIndent(2)
	if err := encoder.Encode(e.mapping); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if e.bom {
		out.WriteString("\ufeff")
	}
	out.WriteString(strings.ReplaceAll("---\n"+yamlText.String()+"---\n", "\n", e.newline))
	out.Write(e.body)
	return out.Bytes(), nil
}

// lookup returns the value node at a dotted path, or nil.
func (e *editable) lookup(path string) *yaml.Node {
	node := e.mapping
	for _, key := range strings.Split(path, ".") {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
			}
		}
		node = next
	}
	return node
}

// set replaces the value at a dotted path, creating missing mappings and
// appending missing keys at the end of their mapping.
func (e *editable) set(path string, value *yaml.Node) {
	node := e.mapping
	keys := strings.Split(path, ".")
	for i, key := range keys {
		index := -1
		for j := 0; j+1 < len(node.Content); j += 2 {
			if node.Content[j].Value == key {
				index = j + 1
			}
		}
		if i == len(keys)-1 {
			if index >= 0 {
				value.HeadComment, value.LineComment = node.Content[index].HeadComment, node.Content[index].LineComment
				node.Content[index] = value
			} else {
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
			}
			return
		}
		if index < 0 || node.Content[index].Kind != yaml.MappingNode {
			child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if index < 0 {
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
			} else {
				node.Content[index] = child
			}
			node = child
			continue
		}
		node = node.Content[index]
	}
}

func nullNode() *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"} }

func intNode(n int64) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(n, 10)}
}

func dateNode(t time.Time) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: formatDate(t)}
}

func stringNode(s string) *yaml.Node {
	node := &yaml.Node{}
	_ = node.Encode(s) // quotes values YAML would read as another type
	return node
}

// listNode keeps relation lists in flow style, as the templates write them.
func listNode(values []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, v := range values {
		node.Content = append(node.Content, stringNode(v))
	}
	return node
}

// replaceTitleHeading changes the first level-one heading of the body.
func (e *editable) replaceTitleHeading(title string) {
	lines := strings.SplitAfter(string(e.body), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			lines[i] = "# " + title + line[len(strings.TrimRight(line, "\r\n")):]
			e.body = []byte(strings.Join(lines, ""))
			return
		}
	}
}
