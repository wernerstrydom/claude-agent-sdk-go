package agent

import (
	"fmt"
	"reflect"
	"strings"
)

const maxSchemaDepth = 10

// schemaFromValue generates a JSON Schema from a Go value.
// The value should be a struct or pointer to struct.
func schemaFromValue(v any) (map[string]any, error) {
	if v == nil {
		return nil, &SchemaError{Type: "nil", Reason: "cannot generate schema from nil value"}
	}
	return schemaFromType(reflect.TypeOf(v))
}

// schemaFromType generates a JSON Schema from a Go type.
// The type must be a struct (or pointer to struct).
func schemaFromType(t reflect.Type) (map[string]any, error) {
	return schemaFromTypeWithDepth(t, 0)
}

// schemaFromTypeWithDepth generates schema with depth tracking to prevent infinite recursion.
func schemaFromTypeWithDepth(t reflect.Type, depth int) (map[string]any, error) {
	if depth > maxSchemaDepth {
		return nil, &SchemaError{
			Type:   t.String(),
			Reason: "maximum nesting depth exceeded (possible circular reference)",
		}
	}

	// Unwrap pointers
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		return buildStructSchema(t, depth)
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Slice, reflect.Array:
		return buildArraySchema(t, depth)
	case reflect.Map:
		return buildMapSchema(t, depth)
	case reflect.Interface:
		// any/interface{} - no type constraint
		return map[string]any{}, nil
	case reflect.Func, reflect.Chan, reflect.Complex64, reflect.Complex128:
		return nil, &SchemaError{
			Type:   t.String(),
			Reason: fmt.Sprintf("unsupported type kind: %s", t.Kind()),
		}
	default:
		return nil, &SchemaError{
			Type:   t.String(),
			Reason: fmt.Sprintf("unsupported type kind: %s", t.Kind()),
		}
	}
}

// buildStructSchema creates a JSON Schema for a struct type.
func buildStructSchema(t reflect.Type, depth int) (map[string]any, error) {
	properties := make(map[string]any)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Handle embedded structs
		if field.Anonymous {
			embeddedProps, embeddedRequired, err := flattenEmbeddedStruct(field, depth)
			if err != nil {
				return nil, err
			}
			for k, v := range embeddedProps {
				properties[k] = v
			}
			required = append(required, embeddedRequired...)
			continue
		}

		// Parse json tag
		name, omitempty, skip := parseJSONTag(field.Tag.Get("json"))
		if skip {
			continue
		}
		if name == "" {
			name = field.Name
		}

		// Build field schema
		fieldSchema, err := schemaFromTypeWithDepth(field.Type, depth+1)
		if err != nil {
			return nil, err
		}

		// Add description from desc tag
		if desc := field.Tag.Get("desc"); desc != "" {
			fieldSchema["description"] = desc
		}

		properties[name] = fieldSchema

		// Determine if field is required
		isPointer := field.Type.Kind() == reflect.Ptr
		if !omitempty && !isPointer {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// flattenEmbeddedStruct extracts properties from an embedded struct.
func flattenEmbeddedStruct(field reflect.StructField, depth int) (map[string]any, []string, error) {
	t := field.Type

	// Unwrap pointer for embedded struct
	isPointer := t.Kind() == reflect.Ptr
	if isPointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		// Not a struct, treat as regular field
		return nil, nil, nil
	}

	properties := make(map[string]any)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if !f.IsExported() {
			continue
		}

		name, omitempty, skip := parseJSONTag(f.Tag.Get("json"))
		if skip {
			continue
		}
		if name == "" {
			name = f.Name
		}

		fieldSchema, err := schemaFromTypeWithDepth(f.Type, depth+1)
		if err != nil {
			return nil, nil, err
		}

		if desc := f.Tag.Get("desc"); desc != "" {
			fieldSchema["description"] = desc
		}

		properties[name] = fieldSchema

		// Embedded pointer struct fields are optional
		fieldIsPointer := f.Type.Kind() == reflect.Ptr
		if !omitempty && !isPointer && !fieldIsPointer {
			required = append(required, name)
		}
	}

	return properties, required, nil
}

// buildArraySchema creates a JSON Schema for a slice/array type.
func buildArraySchema(t reflect.Type, depth int) (map[string]any, error) {
	elemSchema, err := schemaFromTypeWithDepth(t.Elem(), depth+1)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"type":  "array",
		"items": elemSchema,
	}, nil
}

// buildMapSchema creates a JSON Schema for a map type.
func buildMapSchema(t reflect.Type, depth int) (map[string]any, error) {
	// Only support string keys
	if t.Key().Kind() != reflect.String {
		return nil, &SchemaError{
			Type:   t.String(),
			Reason: "only maps with string keys are supported",
		}
	}

	valueSchema, err := schemaFromTypeWithDepth(t.Elem(), depth+1)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": valueSchema,
	}, nil
}

// parseJSONTag extracts field name and flags from a json struct tag.
// Returns (name, omitempty, skip).
func parseJSONTag(tag string) (string, bool, bool) {
	if tag == "" {
		return "", false, false
	}
	if tag == "-" {
		return "", false, true
	}

	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "-" {
		return "", false, true
	}

	omitempty := false
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitempty = true
			break
		}
	}

	return name, omitempty, false
}
