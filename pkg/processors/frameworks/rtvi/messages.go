// Package rtvi implements the RTVI (Real-Time Voice Interface) protocol processor and message types.
package rtvi

// RTVI protocol version sent in bot-ready.
const ProtocolVersion = "1.2.0"

// MessageLabel is the RTVI message label on the wire.
const MessageLabel = "rtvi-ai"

// Message is the generic RTVI wire message (client and server).
type Message struct {
	Label string         `json:"label"`
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Data  map[string]any `json:"data,omitempty"`
}

// Client message types.
const (
	TypeClientReady = "client-ready"
	TypeSendText    = "send-text"
)

// Server message types.
const (
	TypeBotReady       = "bot-ready"
	TypeError          = "error"
	TypeErrorResponse  = "error-response"
	TypeServerResponse = "server-response"
)

// SendTextData is the data payload for send-text (client).
type SendTextData struct {
	Content string        `json:"content"`
	Options *SendTextOpts  `json:"options,omitempty"`
}

// SendTextOpts options for send-text.
type SendTextOpts struct {
	RunImmediately  bool `json:"run_immediately"`
	AudioResponse   bool `json:"audio_response"`
}

// BotReadyData is the data payload for bot-ready (server).
type BotReadyData struct {
	Version string         `json:"version"`
	About   map[string]any `json:"about,omitempty"`
}

// ErrorData is the data payload for error (server).
type ErrorData struct {
	Error string `json:"error"`
	Fatal bool   `json:"fatal"`
}

// ErrorResponseData is the data payload for error-response (server).
type ErrorResponseData struct {
	Error string `json:"error"`
}

// ServerResponseData is the data payload for server-response (server).
type ServerResponseData struct {
	T string         `json:"t"` // type from client
	D map[string]any `json:"d,omitempty"`
}
