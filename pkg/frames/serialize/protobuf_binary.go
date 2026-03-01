// Package serialize provides binary frame protobuf encode/decode for frames.
package serialize

import "voxray-go/pkg/frames"

// ProtoEncode encodes a Frame to binary protobuf format (wire-compatible with common frame schemas).
// Returns nil, nil if the frame type is not serializable in this format.
func ProtoEncode(f frames.Frame) ([]byte, error) {
	return encodeFrameWire(f)
}

// ProtoDecode decodes binary protobuf frame data into a Frame.
func ProtoDecode(data []byte) (frames.Frame, error) {
	return decodeFrameWire(data)
}
