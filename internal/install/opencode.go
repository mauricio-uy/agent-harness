package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// mergeOpenCodeConfig adds the harness's skill permissions to opencode.json.
// Keys the project already sets are kept, as is the order of the file.
func (in *Installer) mergeOpenCodeConfig() error {
	template, err := fs.ReadFile(in.Payload, "template-clients/opencode/opencode.json")
	if err != nil {
		return err
	}
	target := filepath.Join(in.Root, "opencode.json")
	raw, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		if exists(filepath.Join(in.Root, "opencode.jsonc")) {
			in.warn("opencode.jsonc exists; add the permission.skill entries from the harness's opencode.json manually")
			return nil
		}
		if err := os.WriteFile(target, template, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(in.Out, "CREATE opencode.json")
		return nil
	}
	if err != nil {
		return err
	}
	wanted, err := decodeJSON(template)
	if err != nil {
		return fmt.Errorf("embedded opencode.json: %w", err)
	}
	current, err := decodeJSON(raw)
	if err != nil {
		in.warn("opencode.json is not plain JSON (%v); add the permission.skill entries manually", err)
		return nil
	}
	root, ok := current.(*object)
	if !ok {
		in.warn("opencode.json must contain an object; add the permission.skill entries manually")
		return nil
	}
	skills, err := root.child("permission", "skill")
	if err != nil {
		in.warn("opencode.json: %v; add the permission.skill entries manually", err)
		return nil
	}
	required, _ := wanted.(*object).child("permission", "skill")
	added := 0
	for _, key := range required.keys {
		if _, set := skills.values[key]; !set {
			skills.set(key, required.values[key])
			added++
		}
	}
	if added == 0 {
		fmt.Fprintln(in.Out, "OK     opencode.json")
		return nil
	}
	var buffer bytes.Buffer
	encodeJSON(&buffer, root, "")
	buffer.WriteByte('\n')
	if err := os.WriteFile(target, buffer.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(in.Out, "UPDATE opencode.json (%d permission.skill entries)\n", added)
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// object is a JSON object that remembers key order.
type object struct {
	keys   []string
	values map[string]any
}

func (o *object) set(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// child returns the nested object at path, creating missing levels.
func (o *object) child(path ...string) (*object, error) {
	current := o
	for _, key := range path {
		value, ok := current.values[key]
		if !ok {
			next := &object{values: map[string]any{}}
			current.set(key, next)
			current = next
			continue
		}
		next, ok := value.(*object)
		if !ok {
			return nil, fmt.Errorf("%s is not an object", key)
		}
		current = next
	}
	return current, nil
}

func decodeJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the top-level value")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		o := &object{values: map[string]any{}}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			o.set(key.(string), value)
		}
		_, err := decoder.Token()
		return o, err
	case json.Delim('['):
		var items []any
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		_, err := decoder.Token()
		return items, err
	}
	return token, nil
}

func encodeJSON(b *bytes.Buffer, value any, indent string) {
	switch v := value.(type) {
	case *object:
		if len(v.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, key := range v.keys {
			b.WriteString(indent + "  ")
			writeScalar(b, key)
			b.WriteString(": ")
			encodeJSON(b, v.values[key], indent+"  ")
			if i < len(v.keys)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(indent + "}")
	case []any:
		if len(v) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, item := range v {
			b.WriteString(indent + "  ")
			encodeJSON(b, item, indent+"  ")
			if i < len(v)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(indent + "]")
	default:
		writeScalar(b, v)
	}
}

func writeScalar(b *bytes.Buffer, value any) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	b.WriteString(strings.TrimSuffix(buffer.String(), "\n"))
}
