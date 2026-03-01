package rtvi

import (
	"encoding/json"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/frames/serialize"
)

// Serializer converts between pipeline frames and RTVI protocol JSON. Use when the transport speaks RTVI (e.g. WebSocket with ?rtvi=1).
type Serializer struct{}

// Serialize converts a pipeline frame to RTVI server message JSON. Returns nil, nil for frames that should not be sent as RTVI.
func (Serializer) Serialize(f frames.Frame) ([]byte, error) {
	msg := frameToRTVIMessage(f)
	if msg == nil {
		return nil, nil
	}
	return json.Marshal(msg)
}

// Deserialize parses RTVI client message JSON and returns an RTVIClientMessageFrame.
func (Serializer) Deserialize(data []byte) (frames.Frame, error) {
	var raw struct {
		Label string         `json:"label"`
		Type  string         `json:"type"`
		ID    string         `json:"id"`
		Data  map[string]any `json:"data"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Label != MessageLabel && raw.Label != "" {
		return nil, nil // not RTVI, skip
	}
	return frames.NewRTVIClientMessageFrame(raw.ID, raw.Type, raw.Data), nil
}

func frameToRTVIMessage(f frames.Frame) *Message {
	switch t := f.(type) {
	case *frames.RTVIServerMessageFrame:
		return &Message{Label: MessageLabel, Type: t.Type, ID: t.MsgID, Data: t.Data}
	case *frames.LLMTextFrame:
		return &Message{
			Label: MessageLabel,
			Type:  "bot-output",
			ID:    "",
			Data:  map[string]any{"text": t.Text, "spoken": false},
		}
	case *frames.TranscriptionFrame:
		return &Message{
			Label: MessageLabel,
			Type:  "user-transcription",
			ID:    "",
			Data:  map[string]any{"text": t.Text, "final": t.Finalized},
		}
	case *frames.ErrorFrame:
		return &Message{
			Label: MessageLabel,
			Type:  TypeError,
			ID:    "",
			Data:  map[string]any{"error": t.Error, "fatal": t.Fatal},
		}
	case *frames.BotStartedSpeakingFrame:
		return &Message{Label: MessageLabel, Type: "bot-started-speaking", ID: "", Data: nil}
	case *frames.BotStoppedSpeakingFrame:
		return &Message{Label: MessageLabel, Type: "bot-stopped-speaking", ID: "", Data: nil}
	default:
		return nil
	}
}

var _ serialize.Serializer = (*Serializer)(nil)
