//go:build cgo

package smallwebrtc

import (
	"encoding/binary"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"layeh.com/gopus"

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
)

func init() {
	outboundEncoderAvailable = true
	outboundRunner = runOutboundEncode
}

func runOutboundEncode(track *webrtc.TrackLocalStaticSample, outCh <-chan frames.Frame, closed <-chan struct{}) {
	enc, err := gopus.NewEncoder(opusSampleRate, 1, gopus.Audio)
	if err != nil {
		logger.Error("smallwebrtc: opus encoder: %v", err)
		runOutboundDrain(track, outCh, closed)
		return
	}
	var pcm48 []byte
	for {
		select {
		case <-closed:
			return
		case f, ok := <-outCh:
			if !ok {
				return
			}
			tts, ok := f.(*frames.TTSAudioRawFrame)
			if !ok {
				continue
			}
			sr := tts.SampleRate
			if sr <= 0 {
				sr = 24000
			}
			pcm := tts.Audio
			if len(pcm) == 0 {
				continue
			}
			if sr != opusSampleRate {
				pcm48 = audio.Resample16Mono(pcm, sr, opusSampleRate, pcm48[:0])
				if cap(pcm48) < len(pcm48) {
					pcm48 = append([]byte(nil), pcm48...)
				}
				pcm = pcm48
			}
			for len(pcm) >= opusFrameSize {
				frame := pcm[:opusFrameSize]
				pcm = pcm[opusFrameSize:]
				samples := bytesToSamples(frame)
				encoded, err := enc.Encode(samples, opusFrameSamples, 1500)
				if err != nil {
					logger.Error("smallwebrtc: opus encode: %v", err)
					continue
				}
				if len(encoded) == 0 {
					continue
				}
				err = track.WriteSample(media.Sample{Data: encoded, Duration: 20 * time.Millisecond})
				if err != nil {
					select {
					case <-closed:
						return
					default:
						logger.Error("smallwebrtc: write sample: %v", err)
					}
				}
			}
		}
	}
}

func bytesToSamples(b []byte) []int16 {
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out
}
