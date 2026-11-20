// Package schema validates model output against the JSON-schema subset that
// Tessivo output templates use: type, properties, required, items, enum,
// minimum, maximum, minItems and additionalProperties=false.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Schema is the supported subset of JSON Schema.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
}

// Parse decodes a schema document.
func Parse(raw []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return &s, nil
}

// ValidateJSON decodes data and validates it. The error lists every problem
// with a JSON path, so it can be fed back to the model for a retry.
func (s *Schema) ValidateJSON(data []byte) error {
	var v any
	if err := json.Unmarshal([]byte(stripFences(string(data))), &v); err != nil {
		return fmt.Errorf("output is not valid JSON: %w", err)
	}
	var errs []string
	s.validate("$", v, &errs)
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// stripFences removes a surrounding ```json … ``` block some models add.
func stripFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:]
	}
	return strings.TrimSuffix(strings.TrimSpace(t), "```")
}

// StripFences is exported for callers that store the cleaned JSON.
func StripFences(s string) string { return strings.TrimSpace(stripFences(s)) }

func (s *Schema) validate(path string, v any, errs *[]string) {
	if len(s.Enum) > 0 && !slices.ContainsFunc(s.Enum, func(e any) bool { return fmt.Sprint(e) == fmt.Sprint(v) }) {
		*errs = append(*errs, fmt.Sprintf("%s: %v is not one of %v", path, v, s.Enum))
		return
	}
	switch s.Type {
	case "object":
		s.validateObject(path, v, errs)
	case "array":
		s.validateArray(path, v, errs)
	case "string":
		if _, ok := v.(string); !ok {
			*errs = append(*errs, fmt.Sprintf("%s: want string, got %s", path, kind(v)))
		}
	case "number", "integer":
		s.validateNumber(path, v, errs)
	case "boolean":
		if _, ok := v.(bool); !ok {
			*errs = append(*errs, fmt.Sprintf("%s: want boolean, got %s", path, kind(v)))
		}
	}
}

func (s *Schema) validateObject(path string, v any, errs *[]string) {
	obj, ok := v.(map[string]any)
	if !ok {
		*errs = append(*errs, fmt.Sprintf("%s: want object, got %s", path, kind(v)))
		return
	}
	for _, r := range s.Required {
		if _, ok := obj[r]; !ok {
			*errs = append(*errs, fmt.Sprintf("%s: missing required property %q", path, r))
		}
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		child, known := s.Properties[k]
		switch {
		case known:
			child.validate(path+"."+k, obj[k], errs)
		case s.AdditionalProperties != nil && !*s.AdditionalProperties:
			*errs = append(*errs, fmt.Sprintf("%s: unexpected property %q", path, k))
		}
	}
}

func (s *Schema) validateArray(path string, v any, errs *[]string) {
	arr, ok := v.([]any)
	if !ok {
		*errs = append(*errs, fmt.Sprintf("%s: want array, got %s", path, kind(v)))
		return
	}
	if s.MinItems != nil && len(arr) < *s.MinItems {
		*errs = append(*errs, fmt.Sprintf("%s: want at least %d items, got %d", path, *s.MinItems, len(arr)))
	}
	if s.Items != nil {
		for i, item := range arr {
			s.Items.validate(fmt.Sprintf("%s[%d]", path, i), item, errs)
		}
	}
}

func (s *Schema) validateNumber(path string, v any, errs *[]string) {
	n, ok := v.(float64)
	if !ok {
		*errs = append(*errs, fmt.Sprintf("%s: want %s, got %s", path, s.Type, kind(v)))
		return
	}
	if s.Type == "integer" && n != float64(int64(n)) {
		*errs = append(*errs, fmt.Sprintf("%s: want integer, got %v", path, n))
	}
	if s.Minimum != nil && n < *s.Minimum {
		*errs = append(*errs, fmt.Sprintf("%s: %v is below minimum %v", path, n, *s.Minimum))
	}
	if s.Maximum != nil && n > *s.Maximum {
		*errs = append(*errs, fmt.Sprintf("%s: %v is above maximum %v", path, n, *s.Maximum))
	}
}

func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return fmt.Sprintf("%T", v)
	}
}
