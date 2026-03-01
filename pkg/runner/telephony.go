// Package runner provides Pipecat-style telephony WebSocket parsing and serializer construction.
package runner

import (
	"encoding/json"
	"io"

	"voila-go/pkg/frames/serialize"
	"voila-go/pkg/frames/serialize/exotel"
	"voila-go/pkg/frames/serialize/plivo"
	"voila-go/pkg/frames/serialize/telnyx"
	"voila-go/pkg/frames/serialize/twilio"
)

// TelephonyCallData holds provider-specific call identifiers and options.
// Used after ParseTelephonyMessage to build the appropriate serializer.
type TelephonyCallData struct {
	Provider string
	// Twilio
	StreamSid string
	CallSid   string
	Body      map[string]interface{}
	// Telnyx
	StreamID        string
	CallControlID   string
	OutboundEnc     string
	InboundEnc      string
	From, To        string
	// Plivo
	StreamIDPlivo string
	CallID        string
	// Exotel
	AccountSid string
}

// ParseTelephonyMessage parses the first WebSocket message(s) and detects the telephony provider.
// It returns the provider name ("twilio", "telnyx", "plivo", "exotel") and call data.
// Message structure is used to detect: Twilio (event/start/streamSid/callSid), Telnyx (stream_id/call_control_id),
// Plivo (start/streamId/callId), Exotel (event/start/stream_sid/call_sid/account_sid).
func ParseTelephonyMessage(firstMessage []byte, secondMessage []byte) (data TelephonyCallData, ok bool) {
	try := func(raw []byte) (TelephonyCallData, bool) {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return data, false
		}
		// Twilio
		if event, _ := m["event"].(string); event == "start" {
			if start, ok := m["start"].(map[string]interface{}); ok {
				if streamSid, _ := start["streamSid"].(string); streamSid != "" {
					if callSid, _ := start["callSid"].(string); callSid != "" {
						data.Provider = "twilio"
						data.StreamSid = streamSid
						data.CallSid = callSid
						if cp, _ := start["customParameters"].(map[string]interface{}); cp != nil {
							data.Body = cp
						}
						return data, true
					}
				}
			}
		}
		// Telnyx
		if streamID, _ := m["stream_id"].(string); streamID != "" {
			if start, ok := m["start"].(map[string]interface{}); ok {
				if cc, _ := start["call_control_id"].(string); cc != "" {
					data.Provider = "telnyx"
					data.StreamID = streamID
					data.CallControlID = cc
					if mf, _ := start["media_format"].(map[string]interface{}); mf != nil {
						data.OutboundEnc, _ = mf["encoding"].(string)
					}
					data.From, _ = start["from"].(string)
					data.To, _ = start["to"].(string)
					data.InboundEnc = "PCMU"
					return data, true
				}
			}
		}
		// Plivo
		if start, ok := m["start"].(map[string]interface{}); ok {
			if streamID, _ := start["streamId"].(string); streamID != "" {
				if callID, _ := start["callId"].(string); callID != "" {
					data.Provider = "plivo"
					data.StreamIDPlivo = streamID
					data.CallID = callID
					return data, true
				}
			}
		}
		// Exotel
		if event, _ := m["event"].(string); event == "start" {
			if start, ok := m["start"].(map[string]interface{}); ok {
				if streamSid, _ := start["stream_sid"].(string); streamSid != "" {
					if callSid, _ := start["call_sid"].(string); callSid != "" {
						data.Provider = "exotel"
						data.StreamSid = streamSid
						data.CallSid = callSid
						data.AccountSid, _ = start["account_sid"].(string)
						data.From, _ = start["from"].(string)
						data.To, _ = start["to"].(string)
						return data, true
					}
				}
			}
		}
		return data, false
	}
	if d, ok := try(firstMessage); ok {
		return d, true
	}
	if len(secondMessage) > 0 {
		if d, ok := try(secondMessage); ok {
			return d, true
		}
	}
	return data, false
}

// GetAPIKeyFunc returns an API key for a service (e.g. from config or env).
type GetAPIKeyFunc func(service, envVar string) string

// BuildTelephonySerializer returns a Serializer for the given provider and call data.
// getKey is used to resolve account SID, auth token, API key, etc.
func BuildTelephonySerializer(data TelephonyCallData, getKey GetAPIKeyFunc) serialize.Serializer {
	switch data.Provider {
	case "twilio":
		return twilio.NewSerializer(
			data.StreamSid,
			data.CallSid,
			getKey("twilio_account_sid", "TWILIO_ACCOUNT_SID"),
			getKey("twilio_auth_token", "TWILIO_AUTH_TOKEN"),
			"",
			"",
			nil,
		)
	case "telnyx":
		outEnc := data.OutboundEnc
		if outEnc == "" {
			outEnc = "PCMU"
		}
		return telnyx.NewSerializer(
			data.StreamID,
			outEnc,
			data.InboundEnc,
			data.CallControlID,
			getKey("telnyx_api_key", "TELNYX_API_KEY"),
			nil,
		)
	case "plivo":
		return plivo.NewSerializer(
			data.StreamIDPlivo,
			data.CallID,
			getKey("plivo_auth_id", "PLIVO_AUTH_ID"),
			getKey("plivo_auth_token", "PLIVO_AUTH_TOKEN"),
			nil,
		)
	case "exotel":
		return exotel.NewSerializer(data.StreamSid, data.CallSid, nil)
	default:
		return nil
	}
}

// ReadFirstTwoTextMessages reads up to two text messages from the WebSocket-style reader.
// It is used to parse the telephony handshake. The readFunc should read one message and return (payload, nil) or (nil, err).
func ReadFirstTwoTextMessages(readFunc func() ([]byte, error)) (first, second []byte, err error) {
	first, err = readFunc()
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	second, _ = readFunc()
	return first, second, nil
}
