//go:build cgo

package smallwebrtc

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"layeh.com/gopus"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
)

// outboundSamplesPool reuses []int16 buffers for bytesToSamples to reduce allocs in the encode path.
var outboundSamplesPool = sync.Pool{
	New: func() interface{} {
		s := make([]int16, opusFrameSamples)
		return &s
	},
}

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
	firstLog := true
	var next frames.Frame
	for {
		var f frames.Frame
		if next != nil {
			f = next
			next = nil
		} else {
			select {
			case <-closed:
				return
			case recv, ok := <-outCh:
				if !ok {
					return
				}
				f = recv
			}
		}

		if _, isBargeIn := f.(*frames.UserStartedSpeakingFrame); isBargeIn {
			logger.Info("webrtc: barge-in: user started speaking, draining queued TTS")
			next = drainTTSFramesUntilNonTTS(outCh, closed)
			if next == nil {
				return
			}
			continue
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
		if firstLog {
			logger.Info("webrtc: first TTS audio frame received, sending to remote track, %d bytes", len(tts.Audio))
			firstLog = false
		}
		if sr != opusSampleRate {
			pcm48 = audio.Resample16Mono(pcm, sr, opusSampleRate, pcm48[:0])
			if cap(pcm48) < len(pcm48) {
				pcm48 = append([]byte(nil), pcm48...)
			}
			pcm = pcm48
		}
		const frameDuration = 20 * time.Millisecond
		for len(pcm) >= opusFrameSize {
			frame := pcm[:opusFrameSize]
			pcm = pcm[opusFrameSize:]
			withPooledSamples(frame, func(samples []int16) {
				encoded, err := enc.Encode(samples, opusFrameSamples, 1500)
				if err != nil {
					logger.Error("smallwebrtc: opus encode: %v", err)
					return
				}
				if len(encoded) == 0 {
					return
				}
				err = track.WriteSample(media.Sample{Data: encoded, Duration: frameDuration})
				if err != nil {
					select {
					case <-closed:
						return
					default:
						logger.Error("smallwebrtc: write sample: %v", err)
					}
					return
				}
				metrics.WebRTCBytesSentTotal.WithLabelValues("egress", "", "").Add(float64(len(encoded)))
			})
			// Pace at real-time so the client's jitter buffer doesn't underrun or get overwhelmed (reduces stutter)
			select {
			case <-closed:
				return
			case <-time.After(frameDuration):
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

// withPooledSamples gets a []int16 from the pool, fills it from b, calls fn, then Puts the buffer back.
func withPooledSamples(b []byte, fn func(samples []int16)) {
	n := len(b) / 2
	ptr := outboundSamplesPool.Get().(*[]int16)
	out := *ptr
	if cap(out) < n {
		out = make([]int16, n)
		*ptr = out
	} else {
		out = out[:n]
	}
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	fn(out)
	outboundSamplesPool.Put(ptr)
}
