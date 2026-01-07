package agent

import (
	"errors"
	"reflect"
	"testing"
)

func TestSchemaFromType_BasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantType string
	}{
		{"string field", struct{ Name string }{}, "string"},
		{"int field", struct{ Count int }{}, "integer"},
		{"int64 field", struct{ ID int64 }{}, "integer"},
		{"uint field", struct{ Size uint }{}, "integer"},
		{"float64 field", struct{ Price float64 }{}, "number"},
		{"float32 field", struct{ Rate float32 }{}, "number"},
		{"bool field", struct{ Active bool }{}, "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := schemaFromValue(tt.input)
			if err != nil {
				t.Fatalf("schemaFromValue error: %v", err)
			}

			if schema["type"] != "object" {
				t.Errorf("type = %v, want object", schema["type"])
			}

			props := schema["properties"].(map[string]any)
			if len(props) != 1 {
				t.Errorf("len(properties) = %d, want 1", len(props))
			}

			for _, prop := range props {
				propMap := prop.(map[string]any)
				if propMap["type"] != tt.wantType {
					t.Errorf("property type = %v, want %s", propMap["type"], tt.wantType)
				}
			}
		})
	}
}

func TestSchemaFromType_JSONTag(t *testing.T) {
	type Example struct {
		Name     string `json:"name"`
		Age      int    `json:"age,omitempty"`
		Internal string `json:"-"`
		NoTag    string
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	// Check json:"name" works
	if _, ok := props["name"]; !ok {
		t.Error("expected property 'name'")
	}

	// Check json:"age,omitempty" works
	if _, ok := props["age"]; !ok {
		t.Error("expected property 'age'")
	}

	// Check json:"-" skips field
	if _, ok := props["Internal"]; ok {
		t.Error("Internal should be skipped")
	}

	// Check no tag uses field name
	if _, ok := props["NoTag"]; !ok {
		t.Error("expected property 'NoTag'")
	}
}

func TestSchemaFromType_Required(t *testing.T) {
	type Example struct {
		Required   string  `json:"required"`
		Optional   string  `json:"optional,omitempty"`
		PointerOpt *string `json:"pointer_opt"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required field not found or not []string")
	}

	// Only "required" should be in required array
	if len(required) != 1 {
		t.Errorf("len(required) = %d, want 1", len(required))
	}
	if required[0] != "required" {
		t.Errorf("required[0] = %s, want 'required'", required[0])
	}
}

func TestSchemaFromType_Description(t *testing.T) {
	type Example struct {
		Name string `json:"name" desc:"The user's full name"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	name := props["name"].(map[string]any)

	if name["description"] != "The user's full name" {
		t.Errorf("description = %v, want 'The user's full name'", name["description"])
	}
}

func TestSchemaFromType_NestedStruct(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	schema, err := schemaFromValue(Person{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	address := props["address"].(map[string]any)

	if address["type"] != "object" {
		t.Errorf("address.type = %v, want object", address["type"])
	}

	addressProps := address["properties"].(map[string]any)
	if _, ok := addressProps["street"]; !ok {
		t.Error("address should have street property")
	}
	if _, ok := addressProps["city"]; !ok {
		t.Error("address should have city property")
	}
}

func TestSchemaFromType_Array(t *testing.T) {
	type Example struct {
		Tags []string `json:"tags"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	tags := props["tags"].(map[string]any)

	if tags["type"] != "array" {
		t.Errorf("tags.type = %v, want array", tags["type"])
	}

	items := tags["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("tags.items.type = %v, want string", items["type"])
	}
}

func TestSchemaFromType_ArrayOfStructs(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}

	type Example struct {
		Items []Item `json:"items"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	items := props["items"].(map[string]any)

	if items["type"] != "array" {
		t.Errorf("items.type = %v, want array", items["type"])
	}

	itemSchema := items["items"].(map[string]any)
	if itemSchema["type"] != "object" {
		t.Errorf("items.items.type = %v, want object", itemSchema["type"])
	}
}

func TestSchemaFromType_Map(t *testing.T) {
	type Example struct {
		Metadata map[string]string `json:"metadata"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	metadata := props["metadata"].(map[string]any)

	if metadata["type"] != "object" {
		t.Errorf("metadata.type = %v, want object", metadata["type"])
	}

	additionalProps := metadata["additionalProperties"].(map[string]any)
	if additionalProps["type"] != "string" {
		t.Errorf("metadata.additionalProperties.type = %v, want string", additionalProps["type"])
	}
}

func TestSchemaFromType_EmbeddedStruct(t *testing.T) {
	type Base struct {
		ID string `json:"id"`
	}

	type Extended struct {
		Base
		Name string `json:"name"`
	}

	schema, err := schemaFromValue(Extended{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	// Both id and name should be at top level (flattened)
	if _, ok := props["id"]; !ok {
		t.Error("expected flattened property 'id'")
	}
	if _, ok := props["name"]; !ok {
		t.Error("expected property 'name'")
	}
}

func TestSchemaFromType_EmbeddedPointerStruct(t *testing.T) {
	type Base struct {
		ID string `json:"id"`
	}

	type Extended struct {
		*Base
		Name string `json:"name"`
	}

	schema, err := schemaFromValue(Extended{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	// Both id and name should be at top level
	if _, ok := props["id"]; !ok {
		t.Error("expected flattened property 'id'")
	}

	// id from embedded pointer should NOT be required
	required, _ := schema["required"].([]string)
	for _, r := range required {
		if r == "id" {
			t.Error("id from embedded pointer should not be required")
		}
	}
}

func TestSchemaFromType_Pointer(t *testing.T) {
	type Example struct {
		Name    string  `json:"name"`
		OptName *string `json:"opt_name"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	// Pointer field should be unwrapped but not required
	props := schema["properties"].(map[string]any)
	optName := props["opt_name"].(map[string]any)

	if optName["type"] != "string" {
		t.Errorf("opt_name.type = %v, want string", optName["type"])
	}

	required := schema["required"].([]string)
	for _, r := range required {
		if r == "opt_name" {
			t.Error("opt_name should not be required")
		}
	}
}

func TestSchemaFromType_UnsupportedType_Func(t *testing.T) {
	type Example struct {
		Fn func() `json:"fn"`
	}

	_, err := schemaFromValue(Example{})
	if err == nil {
		t.Fatal("expected error for func type, got nil")
	}

	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Errorf("error type = %T, want *SchemaError", err)
	}
}

func TestSchemaFromType_UnsupportedType_Chan(t *testing.T) {
	type Example struct {
		Ch chan int `json:"ch"`
	}

	_, err := schemaFromValue(Example{})
	if err == nil {
		t.Fatal("expected error for chan type, got nil")
	}

	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Errorf("error type = %T, want *SchemaError", err)
	}
}

func TestSchemaFromType_UnsupportedType_MapIntKey(t *testing.T) {
	type Example struct {
		Data map[int]string `json:"data"`
	}

	_, err := schemaFromValue(Example{})
	if err == nil {
		t.Fatal("expected error for map[int]string type, got nil")
	}

	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Errorf("error type = %T, want *SchemaError", err)
	}
}

func TestSchemaFromType_NilValue(t *testing.T) {
	_, err := schemaFromValue(nil)
	if err == nil {
		t.Fatal("expected error for nil value, got nil")
	}

	var schemaErr *SchemaError
	if !errors.As(err, &schemaErr) {
		t.Errorf("error type = %T, want *SchemaError", err)
	}
}

func TestSchemaFromType_PointerToStruct(t *testing.T) {
	type Example struct {
		Name string `json:"name"`
	}

	// Should work with pointer to struct
	schema, err := schemaFromType(reflect.TypeOf(&Example{}))
	if err != nil {
		t.Fatalf("schemaFromType error: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}
}

func TestSchemaFromType_Interface(t *testing.T) {
	type Example struct {
		Data any `json:"data"`
	}

	schema, err := schemaFromValue(Example{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	data := props["data"].(map[string]any)

	// interface{}/any should have no type constraint
	if _, hasType := data["type"]; hasType {
		t.Error("interface{} field should not have type constraint")
	}
}

func TestParseJSONTag(t *testing.T) {
	tests := []struct {
		tag      string
		wantName string
		wantOmit bool
		wantSkip bool
	}{
		{"", "", false, false},
		{"-", "", false, true},
		{"name", "name", false, false},
		{"name,omitempty", "name", true, false},
		{",omitempty", "", true, false},
		{"name,string", "name", false, false},
		{"name,omitempty,string", "name", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, omit, skip := parseJSONTag(tt.tag)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if omit != tt.wantOmit {
				t.Errorf("omitempty = %v, want %v", omit, tt.wantOmit)
			}
			if skip != tt.wantSkip {
				t.Errorf("skip = %v, want %v", skip, tt.wantSkip)
			}
		})
	}
}

// TestSchemaFromType_ComplexNested tests a realistic complex type.
func TestSchemaFromType_ComplexNested(t *testing.T) {
	type Ingredient struct {
		Item   string `json:"item" desc:"Ingredient name"`
		Amount string `json:"amount" desc:"Quantity needed"`
	}

	type Recipe struct {
		Name        string       `json:"name" desc:"Recipe name"`
		PrepTime    string       `json:"prep_time" desc:"Preparation time"`
		Servings    int          `json:"servings" desc:"Number of servings"`
		Ingredients []Ingredient `json:"ingredients" desc:"List of ingredients"`
		Steps       []string     `json:"steps" desc:"Cooking steps"`
		Notes       *string      `json:"notes,omitempty" desc:"Optional notes"`
	}

	schema, err := schemaFromValue(Recipe{})
	if err != nil {
		t.Fatalf("schemaFromValue error: %v", err)
	}

	// Verify overall structure
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}

	props := schema["properties"].(map[string]any)
	if len(props) != 6 {
		t.Errorf("len(properties) = %d, want 6", len(props))
	}

	// Verify required fields (all except notes which has omitempty and is pointer)
	required := schema["required"].([]string)
	expectedRequired := map[string]bool{
		"name":        true,
		"prep_time":   true,
		"servings":    true,
		"ingredients": true,
		"steps":       true,
	}
	if len(required) != len(expectedRequired) {
		t.Errorf("len(required) = %d, want %d", len(required), len(expectedRequired))
	}
	for _, r := range required {
		if !expectedRequired[r] {
			t.Errorf("unexpected required field: %s", r)
		}
	}

	// Verify ingredients is array of objects
	ingredients := props["ingredients"].(map[string]any)
	if ingredients["type"] != "array" {
		t.Errorf("ingredients.type = %v, want array", ingredients["type"])
	}
	if ingredients["description"] != "List of ingredients" {
		t.Errorf("ingredients.description = %v, want 'List of ingredients'", ingredients["description"])
	}
}
