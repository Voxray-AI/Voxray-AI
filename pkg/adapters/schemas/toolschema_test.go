package schemas

import (
	"encoding/json"
	"testing"
)

func TestAdapterTypeConstants(t *testing.T) {
	if AdapterTypeGemini != "gemini" || AdapterTypeShim != "shim" {
		t.Errorf("AdapterType constants: gemini=%q shim=%q", AdapterTypeGemini, AdapterTypeShim)
	}
}

func TestFunctionSchema_JSONRoundTrip(t *testing.T) {
	s := FunctionSchema{
		Name:        "get_weather",
		Description: "Get weather",
		Parameters:  map[string]any{"type": "object"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out FunctionSchema
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Name != s.Name || out.Description != s.Description {
		t.Errorf("round-trip: got %+v", out)
	}
}

func TestToolsSchema_ZeroValue(t *testing.T) {
	var ts ToolsSchema
	if ts.StandardTools != nil {
		t.Error("zero ToolsSchema StandardTools should be nil")
	}
	ts.StandardTools = []FunctionSchema{{Name: "x"}}
	if len(ts.StandardTools) != 1 {
		t.Errorf("StandardTools len = %d", len(ts.StandardTools))
	}
}
