// Package serialize provides JSON (and optional protobuf) encoding/decoding for frames.
package serialize

import (
	"encoding/json"
	"fmt"

	"voila-go/pkg/frames"
)

// Envelope is the JSON envelope for wire format (type + payload).
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Encoder encodes a Frame to JSON with a type discriminator.
func Encoder(f frames.Frame) ([]byte, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	env := Envelope{Type: f.FrameType(), Data: payload}
	return json.Marshal(env)
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
	default:
		return nil, fmt.Errorf("unknown frame type: %s", typ)
	}
}
