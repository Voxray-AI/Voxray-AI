// Package smallwebrtc provides a WebRTC transport for Voila using pion/webrtc.
// Signaling (SDP offer/answer) is handled via HandleOffer; the pipeline connects via Input/Output channels.
package smallwebrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
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
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", fmt.Errorf("smallwebrtc: set local description: %w", err)
	}
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Inbound: read RTP and push to inCh. Full implementation would decode (e.g. Opus) to PCM and emit AudioRawFrame.
		buf := make([]byte, 1500)
		for {
			n, _, err := track.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				audio := make([]byte, n)
				copy(audio, buf[:n])
				// Placeholder: emit as raw; production would decode RTP/codec to PCM and use frames.NewAudioRawFrame or similar.
				ar := frames.NewAudioRawFrame(audio, 16000, 1, 0)
				select {
				case <-t.closed:
					return
				case t.inCh <- ar:
				}
			}
		}
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		logger.Error("smallwebrtc: connection state: %s", s.String())
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			_ = t.Close()
		}
	})
	answerBytes, _ := json.Marshal(pc.LocalDescription())
	return string(answerBytes), nil
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
