// Package serialize provides binary envelope encoding using wire.FrameEnvelope (wire_frames.proto).
// ProtoEncoder/ProtoDecoder use standard protobuf wire format; ReadProtoEnvelope reads length-prefixed envelopes from a stream.
package serialize

import (
	"encoding/json"
	"io"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/frames/proto/wire"
)

// ProtoEncoder encodes a Frame to binary envelope format (wire.FrameEnvelope: type + payload).
// Payload is the JSON-encoded frame body.
func ProtoEncoder(f frames.Frame) ([]byte, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	env := &wire.FrameEnvelope{Type: f.FrameType(), Payload: payload}
	return env.Marshal()
}

// ProtoDecoder decodes a binary envelope (wire.FrameEnvelope) into a Frame.
func ProtoDecoder(data []byte) (frames.Frame, error) {
	var env wire.FrameEnvelope
	if err := env.Unmarshal(data); err != nil {
		return nil, err
	}
	return DecodeByType(env.Type, env.Payload)
}

// ReadProtoEnvelope reads one length-prefixed envelope from r (varint length + FrameEnvelope bytes) and decodes it to a Frame.
func ReadProtoEnvelope(r io.Reader) (frames.Frame, error) {
	data, err := readLengthPrefixed(r)
	if err != nil {
		return nil, err
	}
	return ProtoDecoder(data)
}

// readLengthPrefixed reads a varint length then that many bytes.
func readLengthPrefixed(r io.Reader) ([]byte, error) {
	length, err := readVarint(r)
	if err != nil {
		return nil, err
	}
	if length > 1e9 {
		return nil, io.ErrShortBuffer
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

