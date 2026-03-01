// Package serialize provides frame serialization interfaces and implementations.
package serialize

import "voxray-go/pkg/frames"

// Serializer converts frames to/from wire format (e.g. JSON envelope or binary protobuf).
type Serializer interface {
	Serialize(f frames.Frame) ([]byte, error)
	Deserialize(data []byte) (frames.Frame, error)
}

// SerializerWithSetup is an optional interface. When implemented, Setup is called
// when a StartFrame is received so the serializer can read AudioInSampleRate/AudioOutSampleRate.
type SerializerWithSetup interface {
	Setup(start *frames.StartFrame)
}

// SerializerWithMessageType is an optional interface. When implemented, SerializeWithType
// is used instead of Serialize, and the returned binary flag selects WebSocket text vs binary
// (e.g. Vonage: audio = binary, JSON events = text).
type SerializerWithMessageType interface {
	Serializer
	SerializeWithType(f frames.Frame) (data []byte, binary bool, err error)
}

// JSONSerializer uses the JSON envelope format (type + data).
type JSONSerializer struct{}

func (JSONSerializer) Serialize(f frames.Frame) ([]byte, error) {
	return Encoder(f)
}

func (JSONSerializer) Deserialize(data []byte) (frames.Frame, error) {
	return Decoder(data)
}

// ProtobufSerializer uses binary protobuf frame format.
// Unserializable frame types are skipped (Serialize returns nil, nil).
type ProtobufSerializer struct{}

func (ProtobufSerializer) Serialize(f frames.Frame) ([]byte, error) {
	return ProtoEncode(f)
}

func (ProtobufSerializer) Deserialize(data []byte) (frames.Frame, error) {
	return ProtoDecode(data)
}
