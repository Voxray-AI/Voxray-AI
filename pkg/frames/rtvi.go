// Package frames: RTVI-specific frame types for client/server messaging.

package frames

// RTVIClientMessageFrame carries a parsed RTVI client message from the transport into the pipeline.
// The transport/serializer parses RTVI JSON and pushes this frame; RTVIProcessor handles it
// (e.g. send-text -> TranscriptionFrame, client-ready -> store version).
type RTVIClientMessageFrame struct {
	SystemFrame
	MsgID string         `json:"msg_id"`
	Type  string         `json:"type"`  // e.g. "client-ready", "send-text"
	Data  map[string]any `json:"data"`  // message payload
}

func (*RTVIClientMessageFrame) FrameType() string { return "RTVIClientMessageFrame" }

// NewRTVIClientMessageFrame creates an RTVIClientMessageFrame.
func NewRTVIClientMessageFrame(msgID, typ string, data map[string]any) *RTVIClientMessageFrame {
	if data == nil {
		data = make(map[string]any)
	}
	return &RTVIClientMessageFrame{
		SystemFrame: SystemFrame{Base: NewBase()},
		MsgID:       msgID,
		Type:        typ,
		Data:        data,
	}
}

// RTVIServerMessageFrame carries an RTVI server message to be sent to the client. When the pipeline
// uses an RTVI serializer, this frame is serialized as RTVI JSON (label, type, id, data).
// RTVIProcessor pushes this for bot-ready and error so they go out over the transport.
type RTVIServerMessageFrame struct {
	SystemFrame
	Type   string         `json:"type"`   // e.g. "bot-ready", "error"
	MsgID  string         `json:"id"`    // RTVI message id (wire field "id")
	Data   map[string]any `json:"data"`
}

func (*RTVIServerMessageFrame) FrameType() string { return "RTVIServerMessageFrame" }

// NewRTVIServerMessageFrame creates an RTVIServerMessageFrame.
func NewRTVIServerMessageFrame(typ, msgID string, data map[string]any) *RTVIServerMessageFrame {
	if data == nil {
		data = make(map[string]any)
	}
	return &RTVIServerMessageFrame{
		SystemFrame: SystemFrame{Base: NewBase()},
		Type:        typ,
		MsgID:       msgID,
		Data:        data,
	}
}
