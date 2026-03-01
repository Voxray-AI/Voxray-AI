package schemas

// AdapterType identifies a family of adapter-specific custom tools.
type AdapterType string

const (
	AdapterTypeGemini AdapterType = "gemini"
	AdapterTypeShim   AdapterType = "shim"
)

// FunctionSchema describes a function/tool exposed to an LLM for calling.
// It mirrors the structure commonly used in OpenAI- and Gemini-style tools.
type FunctionSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]any         `json:"parameters,omitempty"`
	Extra       map[string]any         `json:"extra,omitempty"`
}

// ToolsSchema groups standard, schema-based tools together with optional
// adapter-specific custom tools.
type ToolsSchema struct {
	StandardTools []FunctionSchema                     `json:"standard_tools"`
	CustomTools   map[AdapterType][]map[string]any     `json:"custom_tools,omitempty"`
}

