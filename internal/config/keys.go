package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MaxConfigBytes bounds a config file read or imported.
const MaxConfigBytes = 1 << 20

// Parse decodes a config strictly (unknown fields refused) and validates it.
func Parse(b []byte) (File, error) {
	if len(b) > MaxConfigBytes {
		return File{}, fmt.Errorf("config is larger than %d bytes", MaxConfigBytes)
	}
	var f File
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}
	if dec.More() {
		return File{}, errors.New("parse config: trailing data")
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

// Marshal is the config as indented JSON, the form Save writes.
func (f File) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return append(b, '\n'), nil
}

// Get returns the JSON value at a dotted key such as "defaults.model" or
// "providers.0.base_url".
func (f File) Get(key string) (string, error) {
	tree, err := f.tree()
	if err != nil {
		return "", err
	}
	v, err := walk(tree, key)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode %s: %w", key, err)
	}
	return string(b), nil
}

// Set returns the config with key set to value. A value that parses as JSON
// (a number, true/false, an object, a quoted string) is used as such; anything
// else is a string. The result is decoded strictly and validated.
func (f File) Set(key, value string) (File, error) {
	tree, err := f.tree()
	if err != nil {
		return File{}, err
	}
	var v any
	if json.Unmarshal([]byte(value), &v) != nil {
		v = value
	}
	parts := strings.Split(key, ".")
	parent, err := walk(tree, strings.Join(parts[:len(parts)-1], "."))
	if key == "" || err != nil {
		return File{}, fmt.Errorf("config key %q: %w", key, errors.Join(err, errEmptyKey(key)))
	}
	last := parts[len(parts)-1]
	switch p := parent.(type) {
	case map[string]any:
		p[last] = v
	case []any:
		i, err := strconv.Atoi(last)
		if err != nil || i < 0 || i >= len(p) {
			return File{}, fmt.Errorf("config key %q: no element %s", key, last)
		}
		p[i] = v
	default:
		return File{}, fmt.Errorf("config key %q: its parent is not an object or a list", key)
	}
	b, err := json.Marshal(tree)
	if err != nil {
		return File{}, fmt.Errorf("encode config: %w", err)
	}
	return Parse(b)
}

func errEmptyKey(key string) error {
	if key == "" {
		return errors.New("empty key")
	}
	return nil
}

func (f File) tree() (any, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	var tree any
	if err := json.Unmarshal(b, &tree); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return tree, nil
}

// walk follows a dotted key through objects and lists; "" is the root.
func walk(v any, key string) (any, error) {
	if key == "" {
		return v, nil
	}
	for _, part := range strings.Split(key, ".") {
		switch node := v.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("no key %q", key)
			}
			v = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil, fmt.Errorf("no element %s in %q", part, key)
			}
			v = node[i]
		default:
			return nil, fmt.Errorf("no key %q", key)
		}
	}
	return v, nil
}
