// Package serialize provides JSON (and optional protobuf) encoding/decoding for frames.
package serialize

import (
	"encoding/json"
	"fmt"

	"voxray-go/pkg/frames"
)

// Envelope is the JSON envelope for wire format (type + payload).
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Encoder encodes a Frame to JSON with a type discriminator.
// Wire format: no trailing newline; returned slice is a copy suitable for sending.
func Encoder(f frames.Frame) ([]byte, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	env := Envelope{Type: f.FrameType(), Data: json.RawMessage(payload)}
	data, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), data...), nil
}

// Decoder decodes JSON (envelope format) into a Frame using the type field.
func Decoder(data []byte) (frames.Frame, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return DecodeByType(env.Type, env.Data)
}

// DecodeByType decodes the payload into the concrete frame type.
func DecodeByType(typ string, data []byte) (frames.Frame, error) {
	switch typ {
	case "StartFrame":
		var f frames.StartFrame
		f.SystemFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "CancelFrame":
		var f frames.CancelFrame
		f.SystemFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "ErrorFrame":
		var f frames.ErrorFrame
		f.SystemFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "TextFrame":
		var f frames.TextFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "TranscriptionFrame":
		var f frames.TranscriptionFrame
		f.TextFrame.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMRunFrame":
		var f frames.LLMRunFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMTextFrame":
		var f frames.LLMTextFrame
		f.TextFrame.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "TTSSpeakFrame":
		var f frames.TTSSpeakFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMContextFrame":
		var f frames.LLMContextFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMMessagesUpdateFrame":
		var f frames.LLMMessagesUpdateFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMMessagesAppendFrame":
		var f frames.LLMMessagesAppendFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMSetToolsFrame":
		var f frames.LLMSetToolsFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "LLMSetToolChoiceFrame":
		var f frames.LLMSetToolChoiceFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "FunctionCallResultFrame":
		var f frames.FunctionCallResultFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "TransportMessageFrame", "MessageFrame":
		var f frames.TransportMessageFrame
		f.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "BotStartedSpeakingFrame":
		var f frames.BotStartedSpeakingFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "BotStoppedSpeakingFrame":
		var f frames.BotStoppedSpeakingFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "VADParamsUpdateFrame":
		var f frames.VADParamsUpdateFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "VADUserStartedSpeakingFrame":
		var f frames.VADUserStartedSpeakingFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "VADUserStoppedSpeakingFrame":
		var f frames.VADUserStoppedSpeakingFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "UserSpeakingFrame":
		var f frames.UserSpeakingFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "AggregatedTextFrame":
		var f frames.AggregatedTextFrame
		f.TextFrame.DataFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "OutputDTMFUrgentFrame":
		var f frames.OutputDTMFUrgentFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "InterruptionFrame":
		var f frames.InterruptionFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "InputDTMFFrame":
		var f frames.InputDTMFFrame
		f.ControlFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "RTVIClientMessageFrame":
		var f frames.RTVIClientMessageFrame
		f.SystemFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case "RTVIServerMessageFrame":
		var f frames.RTVIServerMessageFrame
		f.SystemFrame.Base = frames.NewBase()
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("unknown frame type: %s", typ)
	}
}
