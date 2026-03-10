// Package whatsapp provides WhatsApp Cloud API transport for Voxray.
package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
)

// WebhookPayload is the structure of a WhatsApp Cloud API webhook POST body.
type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Audio *struct {
						ID string `json:"id"`
					} `json:"audio,omitempty"`
				} `json:"messages,omitempty"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// Transport implements transport.Transport for WhatsApp: incoming webhook messages are pushed to Input,
// and frames from Output are sent via the WhatsApp API to the current conversation.
type Transport struct {
	api    *Client
	inCh   chan frames.Frame
	outCh  chan frames.Frame
	to     string
	toMu   sync.RWMutex
	closed chan struct{}
	once   sync.Once
}

// NewTransport creates a WhatsApp transport. The recipient (to) is set when the first webhook message is received.
func NewTransport(api *Client) *Transport {
	return &Transport{
		api:    api,
		inCh:   make(chan frames.Frame, 64),
		outCh:  make(chan frames.Frame, 64),
		closed: make(chan struct{}),
	}
}

// Done returns a channel that is closed when the transport is closed.
func (t *Transport) Done() <-chan struct{} { return t.closed }

// Input returns the channel of frames from incoming WhatsApp messages (e.g. text -> TextFrame/TranscriptionFrame).
func (t *Transport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the user (e.g. TextFrame/TTSAudioRawFrame -> send message).
func (t *Transport) Output() chan<- frames.Frame { return t.outCh }

// Start starts the output loop that reads from Output() and sends to WhatsApp. Call after pipeline is running.
func (t *Transport) Start(ctx context.Context) error {
	go t.outputLoop(ctx)
	go func() {
		select {
		case <-ctx.Done():
			_ = t.Close()
		case <-t.closed:
		}
	}()
	return nil
}

// Close closes the transport channels.
func (t *Transport) Close() error {
	t.once.Do(func() {
		close(t.closed)
		close(t.inCh)
		close(t.outCh)
	})
	return nil
}

func (t *Transport) outputLoop(ctx context.Context) {
	for {
		select {
		case <-t.closed:
			return
		case <-ctx.Done():
			return
		case f, ok := <-t.outCh:
			if !ok {
				return
			}
			t.toMu.RLock()
			to := t.to
			t.toMu.RUnlock()
			if to == "" {
				continue
			}
			switch v := f.(type) {
			case *frames.TextFrame:
				if err := t.api.SendText(ctx, to, v.Text); err != nil {
					logger.Error("whatsapp send text: %v", err)
				}
			case *frames.LLMTextFrame:
				if err := t.api.SendText(ctx, to, v.Text); err != nil {
					logger.Error("whatsapp send text: %v", err)
				}
			case *frames.TTSAudioRawFrame:
				// Audio reply would require media upload; for minimal we skip
				_ = v
			}
		}
	}
}

// MaxWebhookBodyBytes is the maximum POST body size for WhatsApp webhook (256KB).
const MaxWebhookBodyBytes = 256 * 1024

// HandleWebhook handles the WhatsApp webhook verification (GET) and incoming messages (POST).
// For GET with hub.mode=subscribe, respond with hub.challenge if verify_token matches.
// For POST, when appSecret is non-empty, X-Hub-Signature-256 (HMAC-SHA256 of body) is verified; then the payload is parsed.
func (t *Transport) HandleWebhook(verifyToken, appSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mode := r.URL.Query().Get("hub.mode")
			token := r.URL.Query().Get("hub.verify_token")
			challenge := r.URL.Query().Get("hub.challenge")
			if mode == "subscribe" && token == verifyToken {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(challenge))
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxWebhookBodyBytes))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if appSecret != "" {
			sigHeader := r.Header.Get("X-Hub-Signature-256")
			if !strings.HasPrefix(sigHeader, "sha256=") {
				http.Error(w, "missing or invalid signature", http.StatusUnauthorized)
				return
			}
			expectedSig := strings.TrimSpace(sigHeader[7:])
			mac := hmac.New(sha256.New, []byte(appSecret))
			mac.Write(body)
			computedSig := hex.EncodeToString(mac.Sum(nil))
			if len(expectedSig) != len(computedSig) || !hmac.Equal([]byte(computedSig), []byte(expectedSig)) {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}
		var payload WebhookPayload
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Object != "whatsapp_business_account" {
			w.WriteHeader(http.StatusOK)
			return
		}
		for _, entry := range payload.Entry {
			for _, change := range entry.Changes {
				msgs := change.Value.Messages
				for _, msg := range msgs {
					t.toMu.Lock()
					if t.to == "" {
						t.to = msg.From
					}
					t.toMu.Unlock()
					if msg.Text != nil {
						tf := frames.NewTranscriptionFrame(msg.Text.Body, msg.From, msg.Timestamp, true)
						select {
						case <-t.closed:
							return
						case t.inCh <- tf:
						}
					}
				}
			}
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}
}
