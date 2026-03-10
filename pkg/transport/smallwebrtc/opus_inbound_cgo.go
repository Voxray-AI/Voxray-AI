//go:build cgo

package smallwebrtc

import (
	"encoding/binary"
	"sync"

	"layeh.com/gopus"
)

const inboundDecodeBufSize = 960 * 2 // 20 ms at 48 kHz, 16-bit

var inboundDecodePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, inboundDecodeBufSize)
		return &b
	},
}

func init() {
	newInboundOpusDecoder = newGopusInboundOpusDecoder
}

func newGopusInboundOpusDecoder() (inboundOpusDecoder, error) {
	dec, err := gopus.NewDecoder(opusSampleRate, 1)
	if err != nil {
		return nil, err
	}
	return &gopusInboundDecoder{dec: dec}, nil
}

type gopusInboundDecoder struct {
	dec *gopus.Decoder
}

// Decode decodes one Opus RTP payload to 48 kHz mono PCM (S16LE).
// frameSize 960 = 20 ms at 48 kHz; gopus returns up to that many samples.
// Uses a pooled buffer and returns a copy so the caller may retain the result.
func (g *gopusInboundDecoder) Decode(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	const frameSize = 960 // 20 ms at 48 kHz
	samples, err := g.dec.Decode(payload, frameSize, false)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, nil
	}
	n := len(samples) * 2
	ptr := inboundDecodePool.Get().(*[]byte)
	buf := *ptr
	if cap(buf) < n {
		buf = make([]byte, n)
		*ptr = buf
	} else {
		buf = buf[:n]
	}
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	out := append([]byte(nil), buf...)
	inboundDecodePool.Put(ptr)
	return out, nil
}
