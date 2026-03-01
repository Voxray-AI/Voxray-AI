//go:build cgo

package smallwebrtc

import (
	"encoding/binary"

	"layeh.com/gopus"
)

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
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(s))
	}
	return out, nil
}
