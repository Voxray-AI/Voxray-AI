// Package serialize implements binary frame wire encoding/decoding
// without depending on generated descriptor (avoids init issues when protoc is not available).
package serialize

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"voxray-go/pkg/frames"
)

// Wire field numbers (from wire_frames.proto).
const (
	wireFrameText          = 1
	wireFrameAudio         = 2
	wireFrameTranscription = 3
	wireFrameMessage       = 4

	wireTextFrameId   = 1
	wireTextFrameName = 2
	wireTextFrameText = 3

	wireAudioFrameId          = 1
	wireAudioFrameName        = 2
	wireAudioFrameAudio       = 3
	wireAudioFrameSampleRate  = 4
	wireAudioFrameNumChannels = 5
	wireAudioFramePts         = 6

	wireTranscriptionFrameId        = 1
	wireTranscriptionFrameName      = 2
	wireTranscriptionFrameText      = 3
	wireTranscriptionFrameUserId    = 4
	wireTranscriptionFrameTimestamp = 5

	wireMessageFrameData = 1

	wireVarint         = 0
	wireLengthDelimited = 2
)

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func appendTag(buf []byte, fieldNum int, wireType int) []byte {
	return appendVarint(buf, uint64(fieldNum<<3|wireType))
}

func appendLengthDelimited(buf []byte, fieldNum int, data []byte) []byte {
	buf = appendTag(buf, fieldNum, wireLengthDelimited)
	buf = appendVarint(buf, uint64(len(data)))
	return append(buf, data...)
}

func appendString(buf []byte, fieldNum int, s string) []byte {
	return appendLengthDelimited(buf, fieldNum, []byte(s))
}

func appendBytes(buf []byte, fieldNum int, b []byte) []byte {
	return appendLengthDelimited(buf, fieldNum, b)
}

func readVarint(r io.Reader) (uint64, error) {
	var x uint64
	var s uint
	for i := 0; i < 10; i++ {
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, err
		}
		x |= uint64(b[0]&0x7f) << s
		if b[0] < 0x80 {
			return x, nil
		}
		s += 7
	}
	return 0, fmt.Errorf("varint overflow")
}

func readTag(r io.Reader) (fieldNum int, wireType int, err error) {
	v, err := readVarint(r)
	if err != nil {
		return 0, 0, err
	}
	return int(v >> 3), int(v & 7), nil
}

func readLengthDelimited(r io.Reader) ([]byte, error) {
	n, err := readVarint(r)
	if err != nil {
		return nil, err
	}
	if n > 1e9 {
		return nil, fmt.Errorf("length-delimited too long")
	}
	buf := make([]byte, int(n))
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// encodeFrameWire encodes f into binary Frame wire format.
func encodeFrameWire(f frames.Frame) ([]byte, error) {
	var payload []byte
	var fieldNum int
	switch v := f.(type) {
	case *frames.TextFrame:
		fieldNum = wireFrameText
		payload = encodeTextFrameWire(v)
	case *frames.TranscriptionFrame:
		fieldNum = wireFrameTranscription
		payload = encodeTranscriptionFrameWire(v)
	case *frames.AudioRawFrame:
		fieldNum = wireFrameAudio
		payload = encodeAudioFrameWire(v, v.Audio, v.SampleRate, v.NumChannels, v.PTS())
	case *frames.OutputAudioRawFrame:
		fieldNum = wireFrameAudio
		payload = encodeAudioFrameWire(v, v.Audio, v.SampleRate, v.NumChannels, v.PTS())
	case *frames.TTSAudioRawFrame:
		fieldNum = wireFrameAudio
		payload = encodeAudioFrameWire(v, v.Audio, v.SampleRate, v.NumChannels, v.PTS())
	case *frames.TransportMessageFrame:
		fieldNum = wireFrameMessage
		data, err := json.Marshal(v.Message)
		if err != nil {
			return nil, err
		}
		payload = appendLengthDelimited(nil, wireMessageFrameData, data)
	default:
		return nil, nil
	}
	if payload == nil {
		return nil, nil
	}
	return appendLengthDelimited(nil, fieldNum, payload), nil
}

func encodeTextFrameWire(t *frames.TextFrame) []byte {
	var buf []byte
	buf = appendTag(buf, wireTextFrameId, wireVarint)
	buf = appendVarint(buf, t.ID())
	name := t.FrameType() + "#" + strconv.FormatUint(t.ID(), 10)
	buf = appendString(buf, wireTextFrameName, name)
	buf = appendString(buf, wireTextFrameText, t.Text)
	return buf
}

func encodeTranscriptionFrameWire(t *frames.TranscriptionFrame) []byte {
	var buf []byte
	buf = appendTag(buf, wireTranscriptionFrameId, wireVarint)
	buf = appendVarint(buf, t.ID())
	name := t.FrameType() + "#" + strconv.FormatUint(t.ID(), 10)
	buf = appendString(buf, wireTranscriptionFrameName, name)
	buf = appendString(buf, wireTranscriptionFrameText, t.Text)
	buf = appendString(buf, wireTranscriptionFrameUserId, t.UserID)
	buf = appendString(buf, wireTranscriptionFrameTimestamp, t.Timestamp)
	return buf
}

func encodeAudioFrameWire(f frames.Frame, audio []byte, sampleRate, numChannels int, pts *int64) []byte {
	var buf []byte
	buf = appendTag(buf, wireAudioFrameId, wireVarint)
	buf = appendVarint(buf, f.ID())
	name := f.FrameType() + "#" + strconv.FormatUint(f.ID(), 10)
	buf = appendString(buf, wireAudioFrameName, name)
	buf = appendBytes(buf, wireAudioFrameAudio, audio)
	buf = appendTag(buf, wireAudioFrameSampleRate, wireVarint)
	buf = appendVarint(buf, uint64(sampleRate))
	buf = appendTag(buf, wireAudioFrameNumChannels, wireVarint)
	buf = appendVarint(buf, uint64(numChannels))
	if pts != nil {
		buf = appendTag(buf, wireAudioFramePts, wireVarint)
		buf = appendVarint(buf, uint64(*pts))
	}
	return buf
}

// decodeFrameWire decodes binary Frame wire bytes into a frames.Frame.
func decodeFrameWire(data []byte) (frames.Frame, error) {
	r := &byteReader{data: data}
	fieldNum, wireType, err := readTag(r)
	if err != nil {
		return nil, err
	}
	if wireType != wireLengthDelimited {
		return nil, fmt.Errorf("wire frame: expected length-delimited, got wire type %d", wireType)
	}
	payload, err := readLengthDelimited(r)
	if err != nil {
		return nil, err
	}
	switch fieldNum {
	case wireFrameText:
		return decodeTextFrameWire(payload)
	case wireFrameTranscription:
		return decodeTranscriptionFrameWire(payload)
	case wireFrameAudio:
		return decodeAudioFrameWire(payload)
	case wireFrameMessage:
		return decodeMessageFrameWire(payload)
	default:
		return nil, fmt.Errorf("wire frame: unknown field %d", fieldNum)
	}
}

type byteReader struct {
	data []byte
}

func (b *byteReader) Read(p []byte) (n int, err error) {
	if len(b.data) == 0 {
		return 0, io.EOF
	}
	n = copy(p, b.data)
	b.data = b.data[n:]
	return n, nil
}

func decodeTextFrameWire(payload []byte) (frames.Frame, error) {
	var id uint64
	var text, name string
	r := &byteReader{data: payload}
	for {
		fieldNum, wireType, err := readTag(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if wireType == wireVarint {
			v, _ := readVarint(r)
			if fieldNum == wireTextFrameId {
				id = v
			}
			continue
		}
		if wireType == wireLengthDelimited {
			data, err := readLengthDelimited(r)
			if err != nil {
				return nil, err
			}
			switch fieldNum {
			case wireTextFrameText:
				text = string(data)
			case wireTextFrameName:
				name = string(data)
			}
		}
	}
	f := &frames.TextFrame{}
	if id != 0 {
		f.DataFrame.Base = frames.NewBaseWithID(id)
	} else {
		f.DataFrame.Base = frames.NewBase()
	}
	f.Text = text
	if name != "" {
		f.Metadata()["name"] = name
	}
	return f, nil
}

func decodeTranscriptionFrameWire(payload []byte) (frames.Frame, error) {
	var id uint64
	var text, userID, timestamp, name string
	r := &byteReader{data: payload}
	for {
		fieldNum, wireType, err := readTag(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if wireType == wireVarint {
			v, _ := readVarint(r)
			if fieldNum == wireTranscriptionFrameId {
				id = v
			}
			continue
		}
		if wireType == wireLengthDelimited {
			data, err := readLengthDelimited(r)
			if err != nil {
				return nil, err
			}
			switch fieldNum {
			case wireTranscriptionFrameText:
				text = string(data)
			case wireTranscriptionFrameUserId:
				userID = string(data)
			case wireTranscriptionFrameTimestamp:
				timestamp = string(data)
			case wireTranscriptionFrameName:
				name = string(data)
			}
		}
	}
	tf := &frames.TranscriptionFrame{}
	if id != 0 {
		tf.TextFrame.DataFrame.Base = frames.NewBaseWithID(id)
	} else {
		tf.TextFrame.DataFrame.Base = frames.NewBase()
	}
	tf.Text = text
	tf.UserID = userID
	tf.Timestamp = timestamp
	tf.AppendToContext = true
	if name != "" {
		tf.Metadata()["name"] = name
	}
	return tf, nil
}

func decodeAudioFrameWire(payload []byte) (frames.Frame, error) {
	var id uint64
	var pts uint64
	var audio []byte
	var name string
	var sampleRate, numChannels uint32
	r := &byteReader{data: payload}
	for {
		fieldNum, wireType, err := readTag(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if wireType == wireVarint {
			v, err := readVarint(r)
			if err != nil {
				return nil, err
			}
			switch fieldNum {
			case wireAudioFrameId:
				id = v
			case wireAudioFrameSampleRate:
				sampleRate = uint32(v)
			case wireAudioFrameNumChannels:
				numChannels = uint32(v)
			case wireAudioFramePts:
				pts = v
			}
			continue
		}
		if wireType == wireLengthDelimited {
			data, err := readLengthDelimited(r)
			if err != nil {
				return nil, err
			}
			switch fieldNum {
			case wireAudioFrameAudio:
				audio = data
			case wireAudioFrameName:
				name = string(data)
			}
		}
	}
	af := frames.NewAudioRawFrame(audio, int(sampleRate), int(numChannels), 0)
	if id != 0 {
		af.DataFrame.Base = frames.NewBaseWithID(id)
	}
	if pts != 0 {
		af.SetPTS(int64(pts))
	}
	if name != "" {
		af.Metadata()["name"] = name
	}
	return af, nil
}

func decodeMessageFrameWire(payload []byte) (frames.Frame, error) {
	r := &byteReader{data: payload}
	_, wireType, err := readTag(r)
	if err != nil {
		return nil, err
	}
	if wireType != wireLengthDelimited {
		return nil, fmt.Errorf("wire message frame: expected length-delimited")
	}
	data, err := readLengthDelimited(r)
	if err != nil {
		return nil, err
	}
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("wire message frame json: %w", err)
	}
	return frames.NewTransportMessageFrame(msg), nil
}

// Ensure byteReader implements io.Reader.
var _ io.Reader = (*byteReader)(nil)
