// Package runner provides Pipecat-style runner types and session argument types
// for development runner (aligned with Python pipecat/runner/types.py).
package runner

// RunnerArgs holds common session arguments for the runner.
type RunnerArgs struct {
	// HandleSigint indicates whether to handle SIGINT (default false).
	HandleSigint bool
	// HandleSigterm indicates whether to handle SIGTERM (default false).
	HandleSigterm bool
	// PipelineIdleTimeoutSecs is the pipeline idle timeout in seconds (default 300).
	PipelineIdleTimeoutSecs int
	// Body holds additional request data (e.g. custom parameters from /start or webhook).
	Body map[string]interface{}
}

// DailyRunnerArgs holds Daily transport session arguments.
type DailyRunnerArgs struct {
	RunnerArgs
	RoomURL string
	Token   string
}

// WebSocketRunnerArgs holds WebSocket (telephony) session arguments.
// The WebSocket connection and body are passed when handling /ws for telephony.
type WebSocketRunnerArgs struct {
	RunnerArgs
	// Body is typically set from parsed telephony call data (stream_id, call_id, etc.).
	Body map[string]interface{}
}

// SmallWebRTCRunnerArgs holds Small WebRTC session arguments (e.g. from /sessions/{id}/api/offer).
type SmallWebRTCRunnerArgs struct {
	RunnerArgs
	// SDP is the WebRTC SDP offer string.
	SDP string
	// Type is the SDP type (e.g. "offer").
	Type string
	// PCID is the peer connection ID from the client.
	PCID string
	// RestartPC indicates whether the client requested a peer connection restart.
	RestartPC bool
	// RequestData is the session/request payload from the client.
	RequestData map[string]interface{}
}

// LiveKitRunnerArgs holds LiveKit transport session arguments.
type LiveKitRunnerArgs struct {
	RunnerArgs
	RoomName string
	URL      string
	Token    string
}

// DialinSettings holds dial-in settings from the Daily webhook (PSTN/SIP).
// Matches Pipecat Cloud and Daily.co webhook payload structure (camelCase from webhook).
type DialinSettings struct {
	CallID      string            `json:"callId"`
	CallDomain  string            `json:"callDomain"`
	To          string            `json:"To,omitempty"`
	From        string            `json:"From,omitempty"`
	SIPHeaders  map[string]string `json:"sipHeaders,omitempty"`
}

// DailyDialinRequest is the request body for Daily PSTN dial-in webhook handler.
type DailyDialinRequest struct {
	DialinSettings DialinSettings `json:"dialin_settings"`
	DailyAPIKey    string         `json:"daily_api_key"`
	DailyAPIURL    string         `json:"daily_api_url"`
}
