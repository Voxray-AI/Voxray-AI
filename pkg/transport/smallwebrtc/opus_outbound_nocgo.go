//go:build !cgo

package smallwebrtc

import (
	"encoding/binary"
	"time"

	"github.com/godeps/opus"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
)

func init() {
	outboundEncoderAvailable = true
	outboundRunner = runOutboundEncode
}

func runOutboundEncode(track *webrtc.TrackLocalStaticSample, outCh <-chan frames.Frame, closed <-chan struct{}) {
	enc, err := opus.NewEncoder(opusSampleRate, 1, opus.AppVoIP)
	if err != nil {
		logger.Error("smallwebrtc: opus encoder: %v", err)
		runOutboundDrain(track, outCh, closed)
		return
	}
	var pcm48 []byte
	encodeBuf := make([]byte, 1500) // max output size per frame
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
			logger.Info("webrtc: first TTS audio frame received, sending to remote track (godeps/opus), %d bytes", len(tts.Audio))
			firstLog = false
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
			n, err := enc.Encode(samples, encodeBuf)
			if err != nil {
				logger.Error("smallwebrtc: opus encode: %v", err)
				continue
			}
			if n == 0 {
				continue
			}
			encoded := make([]byte, n)
			copy(encoded, encodeBuf[:n])
			err = track.WriteSample(media.Sample{Data: encoded, Duration: 20 * time.Millisecond})
			if err != nil {
				select {
				case <-closed:
					return
				default:
					logger.Error("smallwebrtc: write sample: %v", err)
				}
			}
			metrics.WebRTCBytesSentTotal.WithLabelValues("egress", "", "").Add(float64(len(encoded)))
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
