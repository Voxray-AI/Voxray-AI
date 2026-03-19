// Package smallwebrtc provides a WebRTC transport for Voxray using pion/webrtc.
// Signaling (SDP offer/answer) is handled via HandleOffer; the pipeline connects via Input/Output channels.
// Inbound: RTP/Opus from the client is decoded to PCM 16 kHz and pushed to Input.
// Outbound: TTSAudioRawFrame from the pipeline is resampled to 48 kHz, encoded to Opus, and sent over a local track.
package smallwebrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	opusdec "github.com/pion/opus"

	"voxray-go/pkg/audio"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
)

// inboundOpusDecoder decodes a single Opus RTP payload to 48 kHz mono PCM (S16LE).
// When built with cgo, gopus is used and supports all Opus config modes; otherwise pion/opus is used (limited modes).
type inboundOpusDecoder interface {
	Decode(payload []byte) (pcm48kMono []byte, err error)
}

// newInboundOpusDecoder is set by opus_inbound_cgo.go when cgo is available (gopus, full Opus support).
var newInboundOpusDecoder func() (inboundOpusDecoder, error) = newPionInboundOpusDecoder

func newPionInboundOpusDecoder() (inboundOpusDecoder, error) {
	return &pionInboundDecoder{dec: opusdec.NewDecoder()}, nil
}

type pionInboundDecoder struct {
	dec opusdec.Decoder
}

func (p *pionInboundDecoder) Decode(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	buf := make([]byte, 640)
	_, _, err := p.dec.Decode(payload, buf)
	if err != nil {
		return nil, err
	}
	return buf[:640], nil
}

const (
	// Pipeline expects 16 kHz mono PCM for STT.
	sttSampleRate = 16000
	// WebRTC Opus typically uses 48 kHz; we resample TTS output to this for encoding.
	opusSampleRate = 48000
	// Opus frame duration used for outbound (20 ms at 48 kHz = 960 samples).
	opusFrameSamples = 960
	opusFrameSize    = opusFrameSamples * 2 // 16-bit
)

// Transport implements transport.Transport for WebRTC.
// Input receives frames from the remote peer (e.g. decoded mic audio); Output sends frames to the remote peer (e.g. TTS).
// THREAD SAFETY: handleInboundTrack and runOutbound are the only writers to inCh/outCh respectively; Close is idempotent; do not send on Output after Close.
type Transport struct {
	cfg              *Config
	pc               *webrtc.PeerConnection
	inCh             chan frames.Frame
	outCh            chan frames.Frame
	closed           chan struct{}
	once             sync.Once
	firstInboundLog  sync.Once
	maxDurationOnce  sync.Once
	inboundChunkCount uint64 // total audio chunks pushed to pipeline (for STT)
	activeCounted    bool
}

// Config holds SmallWebRTC options.
// ICEServers is the list of STUN/TURN server URLs; if empty, a default STUN server is used.
type Config struct {
	ICEServers []string

	// MaxDuration enforces a maximum lifetime for this RTC connection after the
	// first inbound audio is observed (based on first successful Opus decode).
	// 0 or negative disables the enforcement.
	MaxDuration time.Duration

	// OnMaxDurationTimeout is invoked once when MaxDuration elapses.
	// It should cancel the per-connection session context to terminate the transport.
	OnMaxDurationTimeout func()

	// OnClosed is invoked (once) when Close() is called, typically used to cancel
	// the per-connection session context even if the peer closes first.
	OnClosed func()
}

// NewTransport creates a new WebRTC transport.
// Call HandleOffer with the client SDP offer to establish the connection, then Start.
// The transport is not connected until HandleOffer succeeds.
func NewTransport(cfg *Config) *Transport {
	return &Transport{
		cfg:    cfg,
		inCh:   make(chan frames.Frame, 64),
		outCh:  make(chan frames.Frame, 64),
		closed: make(chan struct{}),
	}
}

// Input returns the channel of frames received from the remote peer (e.g. audio decoded to AudioRawFrame).
// Closed when the transport is closed.
func (t *Transport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the remote peer (e.g. TTSAudioRawFrame).
// Do not send after Close.
func (t *Transport) Output() chan<- frames.Frame { return t.outCh }

// HandleOffer sets the remote description from the client SDP offer, creates an answer, and sets up the peer connection.
// It returns the SDP answer to send back to the client. Must be called before Start.
// If the Opus encoder is unavailable (e.g. build without cgo), the connection is still accepted and mic audio is processed, but TTS audio is not sent to the client.
// Not safe to call concurrently with other methods on the same Transport.
func (t *Transport) HandleOffer(offerSDP string) (answerSDP string, err error) {
	logger.Info("webrtc: offer received from client")
	if !OutboundEncoderAvailable() {
		logger.Info("webrtc: Opus encoder unavailable (build without cgo); accepting connection but TTS audio will not be sent. Rebuild with CGO_ENABLED=1 and a C compiler for voice output.")
	}
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}
	if cfg := t.getConfig(); cfg != nil && len(cfg.ICEServers) > 0 {
		servers := make([]webrtc.ICEServer, len(cfg.ICEServers))
		for i, u := range cfg.ICEServers {
			servers[i] = webrtc.ICEServer{URLs: []string{u}}
		}
		config.ICEServers = servers
	}
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return "", fmt.Errorf("smallwebrtc: new peer connection: %w", err)
	}
	t.pc = pc

	// Metrics: new peer connection created.
	metrics.WebRTCPeerConnectionsTotal.WithLabelValues("new", "", "webrtc").Inc()

	var offer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(offerSDP), &offer); err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: unmarshal offer: %w", err)
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: set remote description: %w", err)
	}

	// Add outbound audio track so the answer includes it; client can play TTS.
	outboundTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: opusSampleRate, Channels: 1},
		"audio",
		"voxray",
	)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: new track: %w", err)
	}
	if _, err := pc.AddTrack(outboundTrack); err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: add track: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: set local description: %w", err)
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		t.handleInboundTrack(track)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		logger.Info("webrtc: connection state: %s", s.String())
		state := s.String()
		switch s {
		case webrtc.PeerConnectionStateConnecting:
			metrics.WebRTCPeerConnectionsTotal.WithLabelValues("connecting", "", "webrtc").Inc()
		case webrtc.PeerConnectionStateConnected:
			metrics.WebRTCPeerConnectionsTotal.WithLabelValues("connected", "", "webrtc").Inc()
			if !t.activeCounted {
				t.activeCounted = true
				metrics.WebRTCPeerConnectionsActive.WithLabelValues("webrtc", "").Inc()
			}
		case webrtc.PeerConnectionStateFailed:
			metrics.WebRTCPeerConnectionsTotal.WithLabelValues("failed", "", "webrtc").Inc()
			metrics.WebRTCConnectionFailuresTotal.WithLabelValues("unknown", "webrtc").Inc()
			if t.activeCounted {
				t.activeCounted = false
				metrics.WebRTCPeerConnectionsActive.WithLabelValues("webrtc", "").Dec()
			}
			_ = t.Close()
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateClosed:
			metrics.WebRTCPeerConnectionsTotal.WithLabelValues("closed", "", "webrtc").Inc()
			if t.activeCounted {
				t.activeCounted = false
				metrics.WebRTCPeerConnectionsActive.WithLabelValues("webrtc", "").Dec()
			}
			_ = t.Close()
		default:
			// record raw state string for unexpected values
			metrics.WebRTCPeerConnectionsTotal.WithLabelValues(state, "", "webrtc").Inc()
		}
	})

	// CONCURRENCY: single writer goroutine for outbound; only it sends on the track.
	go t.runOutbound(outboundTrack)

	// Wait for ICE candidate gathering so the answer SDP includes server candidates.
	// Without this, the client never receives our candidates and ICE can fail (connection state: failed).
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
	case <-time.After(10 * time.Second):
		// Return current SDP even if gathering not complete (e.g. no STUN)
	}

	answerBytes, _ := json.Marshal(pc.LocalDescription())
	logger.Info("webrtc: answer created, session ready")
	return string(answerBytes), nil
}

// handleInboundTrack reads RTP from the remote track, decodes Opus to PCM 16 kHz, and pushes AudioRawFrame to inCh.
// CONCURRENCY: single reader goroutine for this track; only it sends on inCh.
func (t *Transport) handleInboundTrack(track *webrtc.TrackRemote) {
	logger.Info("webrtc: inbound track started (reading mic RTP)")
	decoder, err := newInboundOpusDecoder()
	if err != nil {
		logger.Info("webrtc: inbound Opus decoder init failed: %v", err)
		return
	}
	const opusOutSampleRate = 48000
	var pcmAccum []byte
	var rtpCount, decodeCount uint64
	firstRTP, firstDecode := true, true
	var firstDecodeFail sync.Once

	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			logger.Info("webrtc: inbound track ReadRTP ended: %v", err)
			return
		}
		select {
		case <-t.closed:
			return
		default:
		}
		if len(pkt.Payload) == 0 {
			continue
		}
		// PERF: pre-built label set would avoid map alloc per packet at very high rates; consider caching WithLabelValues result at connection start.
		metrics.WebRTCBytesReceivedTotal.WithLabelValues("ingress", "", "").Add(float64(len(pkt.Payload)))
		rtpCount++
		if firstRTP {
			logger.Info("webrtc: first RTP packet received from mic (%d bytes payload)", len(pkt.Payload))
			firstRTP = false
		}
		if rtpCount%50 == 0 {
			logger.Debug("webrtc: RTP packets received so far: %d", rtpCount)
		}
		decoded, decodeErr := decoder.Decode(pkt.Payload)
		if decodeErr != nil || len(decoded) == 0 {
			firstDecodeFail.Do(func() {
				if decodeErr != nil {
					logger.Info("webrtc: Opus decode failed (mic audio not reaching pipeline): %v", decodeErr)
				} else {
					logger.Info("webrtc: Opus decode returned no audio (mic audio not reaching pipeline)")
				}
			})
			continue
		}
		decodeCount++
		if firstDecode {
			logger.Info("webrtc: first Opus decode succeeded, buffering for pipeline")
			firstDecode = false
		}
		// Start RTC max-duration enforcement after the first successful decode.
		t.noteFirstInboundAudio()
		resampled := audio.Resample16MonoAlloc(decoded, opusOutSampleRate, sttSampleRate)
		pcmAccum = append(pcmAccum, resampled...)
		if len(pcmAccum) >= 640 {
			toSend := pcmAccum
			pcmAccum = nil
			ar := frames.NewAudioRawFrame(toSend, sttSampleRate, 1, 0)
			t.firstInboundLog.Do(func() {
				logger.Info("webrtc: first mic audio pushed to pipeline (for STT), %d bytes", len(toSend))
			})
			t.inboundChunkCount++
			if t.inboundChunkCount%25 == 0 {
				logger.Debug("webrtc: audio packets for STT: %d chunks received, latest %d bytes", t.inboundChunkCount, len(toSend))
			}
			select {
			case <-t.closed:
				return
			case t.inCh <- ar:
			}
		}
	}
}

// outboundRunner sends TTS audio from outCh to the track. Set by opus_outbound_cgo.go when cgo is available.
var outboundRunner = runOutboundDrain
// outboundEncoderAvailable is set true by opus_outbound_cgo.go when the Opus encoder is built in.
var outboundEncoderAvailable bool

// OutboundEncoderAvailable reports whether TTS audio can be sent over the outbound track.
// It is false when built without cgo (Opus encoder unavailable); in that case HandleOffer still succeeds but TTS is drained locally.
func OutboundEncoderAvailable() bool { return outboundEncoderAvailable }

// drainTTSFramesUntilNonTTS reads from outCh and discards TTSAudioRawFrame (and duplicate UserStartedSpeakingFrame).
// Returns the first non-TTS frame so the caller can process it; returns nil if channel closed or closed signal.
func drainTTSFramesUntilNonTTS(outCh <-chan frames.Frame, closed <-chan struct{}) frames.Frame {
	for {
		select {
		case <-closed:
			return nil
		case f, ok := <-outCh:
			if !ok {
				return nil
			}
			if _, isTTS := f.(*frames.TTSAudioRawFrame); isTTS {
				continue
			}
			if _, isBargeIn := f.(*frames.UserStartedSpeakingFrame); isBargeIn {
				continue
			}
			return f
		}
	}
}

// runOutboundDrain drains outCh without sending (used when Opus encoder is not available, e.g. without cgo).
func runOutboundDrain(_ *webrtc.TrackLocalStaticSample, outCh <-chan frames.Frame, closed <-chan struct{}) {
	logger.Info("webrtc: Opus encoder unavailable (build without cgo); TTS audio will not be sent. Rebuild with CGO_ENABLED=1 and a C compiler (e.g. MinGW on Windows) for voice output.")
	for {
		select {
		case <-closed:
			return
		case _, ok := <-outCh:
			if !ok {
				return
			}
		}
	}
}

// runOutbound reads TTSAudioRawFrame from outCh and sends them to the track (via outboundRunner).
func (t *Transport) runOutbound(track *webrtc.TrackLocalStaticSample) {
	outboundRunner(track, t.outCh, t.closed)
}

func (t *Transport) getConfig() *Config { return t.cfg }

// Start starts the transport. HandleOffer must have been called first.
// Returns an error if the peer connection was not initialized.
// When ctx is canceled, the transport is closed.
func (t *Transport) Start(ctx context.Context) error {
	if t.pc == nil {
		return fmt.Errorf("smallwebrtc: HandleOffer must be called before Start")
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = t.Close()
		case <-t.closed:
		}
	}()
	return nil
}

// Close closes the peer connection and the Input/Output channels.
// Idempotent; safe to call from any goroutine.
func (t *Transport) Close() error {
	var err error
	t.once.Do(func() {
		close(t.closed)
		if t.pc != nil {
			err = t.pc.Close()
		}
		close(t.inCh)
		close(t.outCh)
		if t.cfg != nil && t.cfg.OnClosed != nil {
			t.cfg.OnClosed()
		}
	})
	return err
}

func (t *Transport) noteFirstInboundAudio() {
	cfg := t.cfg
	if cfg == nil || cfg.MaxDuration <= 0 || cfg.OnMaxDurationTimeout == nil {
		return
	}
	t.maxDurationOnce.Do(func() {
		dur := cfg.MaxDuration
		cb := cfg.OnMaxDurationTimeout
		time.AfterFunc(dur, func() {
			select {
			case <-t.closed:
				return
			default:
			}
			if cb != nil {
				cb()
			}
		})
	})
}
