// Package wire provides frame wire formats. FrameEnvelope is generated from wire_frames.proto
// (type + payload). Marshal/Unmarshal use standard protobuf wire encoding.
package wire

// FrameEnvelope wraps a frame with type discriminator and payload (JSON or binary).
// Matches wire_frames.proto message FrameEnvelope.
type FrameEnvelope struct {
	Type    string
	Payload []byte
}

// Marshal encodes m to protobuf wire format (field 1: type, field 2: payload).
func (m *FrameEnvelope) Marshal() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return appendEnvelope(nil, m.Type, m.Payload), nil
}

// Unmarshal decodes protobuf wire bytes into m.
func (m *FrameEnvelope) Unmarshal(b []byte) error {
	if m == nil {
		return nil
	}
	typ, payload, err := parseEnvelope(b)
	if err != nil {
		return err
	}
	m.Type = typ
	m.Payload = payload
	return nil
}

func appendEnvelope(buf []byte, typ string, payload []byte) []byte {
	// Field 1: string type (wire type 2, length-delimited)
	if len(typ) > 0 {
		buf = append(buf, 0x0a) // tag: field 1, wire 2
		buf = appendVarint(buf, uint64(len(typ)))
		buf = append(buf, typ...)
	}
	// Field 2: bytes payload (wire type 2)
	if len(payload) > 0 {
		buf = append(buf, 0x12) // tag: field 2, wire 2
		buf = appendVarint(buf, uint64(len(payload)))
		buf = append(buf, payload...)
	}
	return buf
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func parseEnvelope(b []byte) (typ string, payload []byte, err error) {
	for len(b) > 0 {
		if len(b) < 2 {
			break
		}
		tag := b[0]
		b = b[1:]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 7)
		if wireType != 2 { // length-delimited
			break
		}
		v, n := readVarint(b)
		if n <= 0 || uint64(len(b)) < uint64(n)+v {
			break
		}
		b = b[n:]
		val := b[:v]
		b = b[v:]
		switch fieldNum {
		case 1:
			typ = string(val)
		case 2:
			payload = append([]byte(nil), val...)
		}
	}
	return typ, payload, nil
}

func readVarint(b []byte) (uint64, int) {
	var x uint64
	var n int
	for ; n < len(b) && n < 10; n++ {
		x |= uint64(b[n]&0x7f) << (7 * n)
		if b[n] < 0x80 {
			return x, n + 1
		}
	}
	return 0, -1
}
