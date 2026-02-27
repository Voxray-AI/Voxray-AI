// Package serialize provides optional binary envelope encoding (proto-shaped: type + payload).
// For full protobuf, run: protoc --go_out=. --go_opt=paths=source_relative pkg/frames/proto/frames.proto
// and switch to using the generated FrameEnvelope with proto.Marshal/Unmarshal.
package serialize

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"voila-go/pkg/frames"
)

// ProtoEncoder encodes a Frame to a binary envelope: 4-byte type length, type, 4-byte payload length, payload (JSON frame only).
// This matches the shape of frames.proto FrameEnvelope for interoperability.
func ProtoEncoder(f frames.Frame) ([]byte, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	typ := f.FrameType()
	buf := make([]byte, 0, 8+len(typ)+len(payload))
	lb := [4]byte{}
	binary.BigEndian.PutUint32(lb[:], uint32(len(typ)))
	buf = append(buf, lb[:]...)
	buf = append(buf, typ...)
	binary.BigEndian.PutUint32(lb[:], uint32(len(payload)))
	buf = append(buf, lb[:]...)
	buf = append(buf, payload...)
	return buf, nil
}

// ProtoDecoder decodes a binary envelope (length-prefixed type + payload) into a Frame.
// Payload is the JSON-encoded frame body; type discriminator is from the binary envelope.
func ProtoDecoder(data []byte) (frames.Frame, error) {
	if len(data) < 8 {
		return nil, errors.New("proto envelope too short")
	}
	tl := binary.BigEndian.Uint32(data[0:4])
	data = data[4:]
	if uint32(len(data)) < tl {
		return nil, errors.New("proto envelope truncated type")
	}
	typ := string(data[:tl])
	data = data[tl:]
	if len(data) < 4 {
		return nil, errors.New("proto envelope too short for payload length")
	}
	pl := binary.BigEndian.Uint32(data[0:4])
	data = data[4:]
	if uint32(len(data)) < pl {
		return nil, fmt.Errorf("proto envelope truncated payload (want %d have %d)", pl, len(data))
	}
	payload := data[:pl]
	return DecodeByType(typ, payload)
}

// ReadProtoEnvelope reads one binary envelope from r and decodes it to a Frame.
func ReadProtoEnvelope(r io.Reader) (frames.Frame, error) {
	var tl, pl uint32
	if err := binary.Read(r, binary.BigEndian, &tl); err != nil {
		return nil, err
	}
	typ := make([]byte, tl)
	if _, err := io.ReadFull(r, typ); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &pl); err != nil {
		return nil, err
	}
	payload := make([]byte, pl)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return DecodeByType(string(typ), payload)
}
