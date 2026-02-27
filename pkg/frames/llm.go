package frames

// LLMContextFrame carries context for the LLM (messages, tools).
type LLMContextFrame struct {
	DataFrame
	Context *LLMContext `json:"context"`
}

func (*LLMContextFrame) FrameType() string { return "LLMContextFrame" }

// NewLLMContextFrame creates an LLMContextFrame.
func NewLLMContextFrame(ctx *LLMContext) *LLMContextFrame {
	return &LLMContextFrame{DataFrame: DataFrame{Base: NewBase()}, Context: ctx}
}

// LLMContext holds messages and optional tools for an LLM.
type LLMContext struct {
	Messages []map[string]any `json:"messages"`
	Tools    []map[string]any `json:"tools,omitempty"`
}

// LLMRunFrame triggers the LLM to run on current context.
type LLMRunFrame struct {
	DataFrame
}

func (*LLMRunFrame) FrameType() string { return "LLMRunFrame" }

// NewLLMRunFrame creates an LLMRunFrame.
func NewLLMRunFrame() *LLMRunFrame {
	return &LLMRunFrame{DataFrame: DataFrame{Base: NewBase()}}
}

// LLMMessagesUpdateFrame replaces current context messages.
type LLMMessagesUpdateFrame struct {
	DataFrame
	Messages []map[string]any `json:"messages"`
	RunLLM   *bool             `json:"run_llm,omitempty"`
}

func (*LLMMessagesUpdateFrame) FrameType() string { return "LLMMessagesUpdateFrame" }

// LLMMessagesAppendFrame appends messages to context.
type LLMMessagesAppendFrame struct {
	DataFrame
	Messages []map[string]any `json:"messages"`
	RunLLM   *bool             `json:"run_llm,omitempty"`
}

func (*LLMMessagesAppendFrame) FrameType() string { return "LLMMessagesAppendFrame" }

// LLMSetToolsFrame sets tools for function calling.
type LLMSetToolsFrame struct {
	DataFrame
	Tools []map[string]any `json:"tools"`
}

func (*LLMSetToolsFrame) FrameType() string { return "LLMSetToolsFrame" }

// LLMSetToolChoiceFrame sets tool choice (none, auto, required, or specific).
type LLMSetToolChoiceFrame struct {
	DataFrame
	ToolChoice string `json:"tool_choice"` // "none", "auto", "required", or JSON object
}

func (*LLMSetToolChoiceFrame) FrameType() string { return "LLMSetToolChoiceFrame" }

// FunctionCallResultFrame is the result of a tool/function call.
type FunctionCallResultFrame struct {
	DataFrame
	FunctionName string `json:"function_name"`
	ToolCallID   string `json:"tool_call_id"`
	Arguments    string `json:"arguments,omitempty"`
	Result       any    `json:"result"`
	RunLLM       *bool  `json:"run_llm,omitempty"`
}

func (*FunctionCallResultFrame) FrameType() string { return "FunctionCallResultFrame" }

// LLMTextFrame is text emitted by the LLM.
type LLMTextFrame struct {
	TextFrame
}

func (*LLMTextFrame) FrameType() string { return "LLMTextFrame" }

// TTSSpeakFrame asks TTS to speak the given text.
type TTSSpeakFrame struct {
	DataFrame
	Text string `json:"text"`
}

func (*TTSSpeakFrame) FrameType() string { return "TTSSpeakFrame" }

// NewTTSSpeakFrame creates a TTSSpeakFrame.
func NewTTSSpeakFrame(text string) *TTSSpeakFrame {
	return &TTSSpeakFrame{DataFrame: DataFrame{Base: NewBase()}, Text: text}
}
