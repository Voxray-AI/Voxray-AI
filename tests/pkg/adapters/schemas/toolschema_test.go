package schemas_test

import (
	"encoding/json"
	"testing"

	"voila-go/pkg/adapters/schemas"
)

func TestAdapterTypeConstants(t *testing.T) {
	if schemas.AdapterTypeGemini != "gemini" || schemas.AdapterTypeShim != "shim" {
		t.Errorf("AdapterType constants: gemini=%q shim=%q", schemas.AdapterTypeGemini, schemas.AdapterTypeShim)
	}
}

func TestFunctionSchema_JSONRoundTrip(t *testing.T) {
	s := schemas.FunctionSchema{
		Name:        "get_weather",
		Description: "Get weather",
		Parameters:  map[string]any{"type": "object"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out schemas.FunctionSchema
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Name != s.Name || out.Description != s.Description {
		t.Errorf("round-trip: got %+v", out)
	}
}

func TestToolsSchema_ZeroValue(t *testing.T) {
	var ts schemas.ToolsSchema
	if ts.StandardTools != nil {
		t.Error("zero ToolsSchema StandardTools should be nil")
	}
	ts.StandardTools = []schemas.FunctionSchema{{Name: "x"}}
	if len(ts.StandardTools) != 1 {
		t.Errorf("StandardTools len = %d", len(ts.StandardTools))
	}
}

// TestFunctionSchema_WithParameters mirrors upstream function-calling adapter: parameters (tool name/param mapping) round-trip.
func TestFunctionSchema_WithParameters(t *testing.T) {
	s := schemas.FunctionSchema{
		Name:        "get_weather",
		Description: "Get the weather for a location",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string", "description": "City name"},
				"unit":     map[string]any{"type": "string", "enum": []any{"celsius", "fahrenheit"}},
			},
			"required": []any{"location"},
		},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out schemas.FunctionSchema
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Name != s.Name {
		t.Errorf("name: got %q", out.Name)
	}
	params, ok := out.Parameters["type"].(string)
	if !ok || params != "object" {
		t.Errorf("parameters type: got %v", out.Parameters["type"])
	}
	props, ok := out.Parameters["properties"].(map[string]any)
	if !ok || props["location"] == nil {
		t.Errorf("parameters.properties: got %v", out.Parameters["properties"])
	}
}

// TestToolsSchema_CustomTools mirrors upstream: adapter-specific tool mapping by AdapterType.
func TestToolsSchema_CustomTools(t *testing.T) {
	ts := schemas.ToolsSchema{
		StandardTools: []schemas.FunctionSchema{{Name: "standard_tool", Description: "A standard tool"}},
		CustomTools: map[schemas.AdapterType][]map[string]any{
			schemas.AdapterTypeGemini: {
				{"name": "gemini_tool", "description": "Gemini-specific"},
			},
		},
	}
	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out schemas.ToolsSchema
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.StandardTools) != 1 || out.StandardTools[0].Name != "standard_tool" {
		t.Errorf("StandardTools: got %v", out.StandardTools)
	}
	geminiTools := out.CustomTools[schemas.AdapterTypeGemini]
	if len(geminiTools) != 1 || geminiTools[0]["name"] != "gemini_tool" {
		t.Errorf("CustomTools gemini: got %v", out.CustomTools)
	}
}
