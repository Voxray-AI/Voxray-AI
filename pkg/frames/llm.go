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

// LLMContext holds messages, optional tools, and tool choice for an LLM.
type LLMContext struct {
	Messages   []map[string]any `json:"messages"`
	Tools      []map[string]any `json:"tools,omitempty"`
	ToolChoice string           `json:"tool_choice,omitempty"` // "none", "auto", "required", or JSON object
}

// AddImageMessage appends a user message with image_url content (e.g. for vision). url is data URL or HTTP URL.
func (c *LLMContext) AddImageMessage(text, url string) {
	content := []map[string]any{}
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
	c.Messages = append(c.Messages, map[string]any{"role": "user", "content": content})
}

// AddAudioMessage appends a user message with input_audio content. data is base64-encoded audio; format e.g. "wav".
func (c *LLMContext) AddAudioMessage(text, data, format string) {
	content := []map[string]any{}
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	content = append(content, map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": data, "format": format}})
	c.Messages = append(c.Messages, map[string]any{"role": "user", "content": content})
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

// LLMFullResponseStartFrame marks the start of a complete LLM response stream.
// Used by extensions (e.g. voicemail classifier, IVR) to know when to begin
// aggregating tokens.
type LLMFullResponseStartFrame struct {
	DataFrame
}

func (*LLMFullResponseStartFrame) FrameType() string { return "LLMFullResponseStartFrame" }

// NewLLMFullResponseStartFrame creates a LLMFullResponseStartFrame.
func NewLLMFullResponseStartFrame() *LLMFullResponseStartFrame {
	return &LLMFullResponseStartFrame{DataFrame: DataFrame{Base: NewBase()}}
}

// LLMFullResponseEndFrame marks the end of a complete LLM response stream.
// Processors that aggregate LLM text should flush on this frame.
type LLMFullResponseEndFrame struct {
	DataFrame
}

func (*LLMFullResponseEndFrame) FrameType() string { return "LLMFullResponseEndFrame" }

// NewLLMFullResponseEndFrame creates a LLMFullResponseEndFrame.
func NewLLMFullResponseEndFrame() *LLMFullResponseEndFrame {
	return &LLMFullResponseEndFrame{DataFrame: DataFrame{Base: NewBase()}}
}

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

// LLMContextSummaryRequestFrame requests context summarization (e.g. when token/message limit reached).
type LLMContextSummaryRequestFrame struct {
	DataFrame
	RequestID             string       `json:"request_id"`
	Context               *LLMContext  `json:"context"`
	MinMessagesToKeep     int          `json:"min_messages_to_keep"`
	TargetContextTokens   int          `json:"target_context_tokens"`
	SummarizationPrompt   string       `json:"summarization_prompt,omitempty"`
	SummarizationTimeout  int          `json:"summarization_timeout,omitempty"` // seconds
}

func (*LLMContextSummaryRequestFrame) FrameType() string { return "LLMContextSummaryRequestFrame" }

// LLMContextSummaryResultFrame carries the result of context summarization.
type LLMContextSummaryResultFrame struct {
	DataFrame
	RequestID            string `json:"request_id"`
	Summary              string `json:"summary"`
	LastSummarizedIndex  int    `json:"last_summarized_index"`
	Error                string `json:"error,omitempty"`
}

func (*LLMContextSummaryResultFrame) FrameType() string { return "LLMContextSummaryResultFrame" }
