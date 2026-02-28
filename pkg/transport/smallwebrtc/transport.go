// Package smallwebrtc provides a WebRTC transport for Voila using pion/webrtc.
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

	"voila-go/pkg/audio"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
)

const (
	// Pipeline expects 16 kHz mono PCM for STT.
	sttSampleRate = 16000
	// WebRTC Opus typically uses 48 kHz; we resample TTS output to this for encoding.
	opusSampleRate = 48000
	// Opus frame duration used for outbound (20 ms at 48 kHz = 960 samples).
	opusFrameSamples = 960
	opusFrameSize    = opusFrameSamples * 2 // 16-bit
)

// Transport implements transport.Transport for WebRTC: frames from the pipeline are sent over a local track,
// and frames received from the remote track are pushed to Input.
type Transport struct {
	cfg    *Config
	pc     *webrtc.PeerConnection
	inCh   chan frames.Frame
	outCh  chan frames.Frame
	closed chan struct{}
	once   sync.Once
}

// Config holds SmallWebRTC transport configuration.
type Config struct {
	ICEServers []string
}

// NewTransport creates a new SmallWebRTC transport. Call HandleOffer with the client's SDP offer to establish the connection.
func NewTransport(cfg *Config) *Transport {
	return &Transport{
		cfg:    cfg,
		inCh:   make(chan frames.Frame, 64),
		outCh:  make(chan frames.Frame, 64),
		closed: make(chan struct{}),
	}
}

// Input returns the channel of frames received from the remote peer (e.g. audio decoded to AudioRawFrame).
func (t *Transport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the remote peer (e.g. TTSAudioRawFrame).
func (t *Transport) Output() chan<- frames.Frame { return t.outCh }

// HandleOffer sets the remote description from the client's offer, creates an answer, and sets up the peer connection.
// Returns the SDP answer to send back to the client. Must be called before Start.
func (t *Transport) HandleOffer(offerSDP string) (answerSDP string, err error) {
	logger.Info("webrtc: offer received from client")
	if !OutboundEncoderAvailable() {
		return "", fmt.Errorf("opus encoder unavailable (build without cgo); TTS audio cannot be sent. Rebuild with CGO_ENABLED=1 and a C compiler (e.g. MinGW on Windows) for voice output")
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
		"voila",
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
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			_ = t.Close()
		}
	})

	// Start goroutine that reads TTS frames from outCh and sends over the outbound track.
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
func (t *Transport) handleInboundTrack(track *webrtc.TrackRemote) {
	decoder := opusdec.NewDecoder()
	// Decoder output: 320 samples per frame (S16LE) = 640 bytes; Opus can be 8/12/16/24/48 kHz bandwidth.
	// We resample to 16 kHz for the pipeline.
	pcmBuf := make([]byte, 640)
	var pcmAccum []byte
	const opusOutSampleRate = 48000 // decoder output is typically 48k or less; treat as 48k for resample

	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
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
		_, _, decodeErr := decoder.Decode(pkt.Payload, pcmBuf)
		if decodeErr != nil {
			// Unsupported frame (e.g. non-SILK); skip
			continue
		}
		// Pion opus Decoder outputs 320 samples = 640 bytes S16LE per frame.
		decoded := pcmBuf[:640]
		resampled := audio.Resample16MonoAlloc(decoded, opusOutSampleRate, sttSampleRate)
		pcmAccum = append(pcmAccum, resampled...)
		// Push in reasonable chunks to avoid tiny frames (e.g. 20ms at 16kHz = 640 bytes)
		if len(pcmAccum) >= 640 {
			toSend := pcmAccum
			pcmAccum = nil
			ar := frames.NewAudioRawFrame(toSend, sttSampleRate, 1, 0)
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

// OutboundEncoderAvailable reports whether TTS can be sent over the outbound track (requires cgo build).
func OutboundEncoderAvailable() bool { return outboundEncoderAvailable }

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

// Start starts the transport. HandleOffer must have been called first. Start returns once the connection is set up.
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

// Close closes the peer connection and channels.
func (t *Transport) Close() error {
	var err error
	t.once.Do(func() {
		close(t.closed)
		if t.pc != nil {
			err = t.pc.Close()
		}
		close(t.inCh)
		close(t.outCh)
	})
	return err
}
