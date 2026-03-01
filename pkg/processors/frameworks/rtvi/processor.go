package rtvi

import (
	"context"
	"encoding/json"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// RTVISender sends RTVI protocol messages to the client (e.g. bot-ready, error). Optional; when nil, processor does not send.
type RTVISender interface {
	SendRTVIMessage(msg *Message) error
}

// RTVIProcessorOptions is the JSON shape for plugin_options["rtvi"].
type RTVIProcessorOptions struct {
	ProtocolVersion string `json:"protocol_version"` // optional override
}

// RTVIProcessor handles RTVI protocol: StartFrame -> send bot-ready; RTVIClientMessageFrame -> client-ready/send-text; forwards other frames.
type RTVIProcessor struct {
	*processors.BaseProcessor
	sender  RTVISender
	version string

	mu           sync.Mutex
	clientReady  bool
	botReadySent bool
}

// NewRTVIProcessor returns an RTVI processor. If sender is nil, bot-ready and error responses are not sent.
func NewRTVIProcessor(name string, sender RTVISender, version string) *RTVIProcessor {
	if name == "" {
		name = "RTVI"
	}
	if version == "" {
		version = ProtocolVersion
	}
	return &RTVIProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		sender:        sender,
		version:       version,
	}
}

// NewRTVIProcessorFromOptions builds an RTVIProcessor from plugin_options. Sender must be set by the runner when using RTVI transport.
func NewRTVIProcessorFromOptions(name string, opts json.RawMessage) *RTVIProcessor {
	var o RTVIProcessorOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	version := o.ProtocolVersion
	if version == "" {
		version = ProtocolVersion
	}
	return NewRTVIProcessor(name, nil, version)
}

// SetSender sets the RTVI sender (e.g. by the server when building the pipeline for an RTVI connection).
func (p *RTVIProcessor) SetSender(s RTVISender) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sender = s
}

// ProcessFrame implements processors.Processor.
func (p *RTVIProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.StartFrame:
		p.mu.Lock()
		sender := p.sender
		version := p.version
		p.mu.Unlock()
		botReady := frames.NewRTVIServerMessageFrame(TypeBotReady, "", map[string]any{
			"version": version,
			"about":   map[string]any{"library": "voxray-go"},
		})
		if sender != nil {
			_ = sender.SendRTVIMessage(&Message{Label: MessageLabel, Type: TypeBotReady, ID: "", Data: botReady.Data})
		} else {
			_ = p.PushDownstream(ctx, botReady)
		}
		return p.PushDownstream(ctx, f)

	case *frames.RTVIClientMessageFrame:
		return p.handleClientMessage(ctx, t)

	case *frames.EndFrame, *frames.CancelFrame:
		return p.PushDownstream(ctx, f)

	case *frames.ErrorFrame:
		p.mu.Lock()
		sender := p.sender
		p.mu.Unlock()
		errData := map[string]any{"error": t.Error, "fatal": t.Fatal}
		if sender != nil {
			_ = sender.SendRTVIMessage(&Message{Label: MessageLabel, Type: TypeError, ID: "", Data: errData})
		} else {
			_ = p.PushDownstream(ctx, frames.NewRTVIServerMessageFrame(TypeError, "", errData))
		}
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *RTVIProcessor) handleClientMessage(ctx context.Context, m *frames.RTVIClientMessageFrame) error {
	switch m.Type {
	case TypeClientReady:
		p.mu.Lock()
		p.clientReady = true
		p.mu.Unlock()
		return nil
	case TypeSendText:
		content, _ := m.Data["content"].(string)
		if content == "" {
			return nil
		}
		tf := frames.NewTranscriptionFrame(content, "", "", true)
		return p.PushDownstream(ctx, tf)
	default:
		// custom client message: forward for other processors or respond via sender if needed
		return p.PushDownstream(ctx, m)
	}
}

var _ processors.Processor = (*RTVIProcessor)(nil)
