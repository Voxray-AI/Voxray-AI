package frameworks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/processors"
)

// NewExternalChainProcessorFromOptions builds an ExternalChainProcessor from plugin_options. If opts is nil or empty, url is empty (no-op).
func NewExternalChainProcessorFromOptions(name string, opts json.RawMessage) *ExternalChainProcessor {
	var o ExternalChainOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	return NewExternalChainProcessor(name, o.URL, o)
}

// ExternalChainOptions is the JSON shape for plugin_options["external_chain"].
type ExternalChainOptions struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`        // default "POST"
	Headers      map[string]string `json:"headers"`      // optional
	TimeoutSec   int               `json:"timeout_sec"` // 0 = 30s default
	Stream       bool              `json:"stream"`        // true = SSE or chunked streaming
	TranscriptKey string           `json:"transcript_key"` // key for input text; default "input"
}

// ExternalChainProcessor calls an external HTTP endpoint (e.g. Langchain/Strands sidecar) with the
// last user message from LLMContextFrame and streams the response back as LLMTextFrame with
// LLMFullResponseStartFrame/LLMFullResponseEndFrame.
type ExternalChainProcessor struct {
	*processors.BaseProcessor
	client        *http.Client
	url           string
	method        string
	headers       map[string]string
	stream        bool
	transcriptKey string
}

// NewExternalChainProcessor returns a processor that forwards LLMContextFrame to the given URL.
// If url is empty, all frames are forwarded (no-op).
func NewExternalChainProcessor(name string, url string, opts ExternalChainOptions) *ExternalChainProcessor {
	if name == "" {
		name = "ExternalChain"
	}
	if opts.Method == "" {
		opts.Method = "POST"
	}
	if opts.TranscriptKey == "" {
		opts.TranscriptKey = "input"
	}
	timeout := 30 * time.Second
	if opts.TimeoutSec > 0 {
		timeout = time.Duration(opts.TimeoutSec) * time.Second
	}
	return &ExternalChainProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		client:        &http.Client{Timeout: timeout},
		url:           url,
		method:        opts.Method,
		headers:       opts.Headers,
		stream:        opts.Stream,
		transcriptKey: opts.TranscriptKey,
	}
}

// ProcessFrame implements processors.Processor. On LLMContextFrame, sends last user message to the
// external endpoint and streams response as LLMTextFrame; other frames are forwarded.
func (p *ExternalChainProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	if p.url == "" {
		return p.PushDownstream(ctx, f)
	}

	ctxFrame, ok := f.(*frames.LLMContextFrame)
	if !ok {
		return p.PushDownstream(ctx, f)
	}

	text := lastUserMessageText(ctxFrame.Context)
	if text == "" {
		return p.PushDownstream(ctx, f)
	}

	_ = p.PushDownstream(ctx, frames.NewLLMFullResponseStartFrame())
	defer func() { _ = p.PushDownstream(ctx, frames.NewLLMFullResponseEndFrame()) }()

	body := map[string]string{p.transcriptKey: strings.TrimSpace(text)}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, p.method, p.url, bytes.NewReader(bodyBytes))
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(fmt.Sprintf("external_chain: HTTP %d", resp.StatusCode), false, p.Name()))
		return nil
	}

	if p.stream {
		err = p.streamResponse(ctx, resp.Body)
	} else {
		err = p.readFullResponse(ctx, resp.Body)
	}
	if err != nil {
		logger.Info("external_chain: %v", err)
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
	}
	return nil
}

func (p *ExternalChainProcessor) streamResponse(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if data == "[DONE]" || data == "null" {
			continue
		}
		var chunk struct {
			Text string `json:"text"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// treat as plain text chunk
			if len(data) > 2 && data[0] == '"' && data[len(data)-1] == '"' {
				_ = json.Unmarshal([]byte(data), &chunk.Text)
			} else {
				chunk.Text = data
			}
		}
		text := chunk.Text
		if text == "" {
			text = chunk.Content
		}
		if text != "" {
			_ = p.PushDownstream(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: text, AppendToContext: true}})
		}
	}
	return scanner.Err()
}

func (p *ExternalChainProcessor) readFullResponse(ctx context.Context, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var out struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		// treat whole body as text
		_ = p.PushDownstream(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: string(b), AppendToContext: true}})
		return nil
	}
	text := out.Text
	if text == "" {
		text = out.Content
	}
	if text != "" {
		_ = p.PushDownstream(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: text, AppendToContext: true}})
	}
	return nil
}

// lastUserMessageText returns the content of the last user message in ctx as plain text.
func lastUserMessageText(ctx *frames.LLMContext) string {
	if ctx == nil || len(ctx.Messages) == 0 {
		return ""
	}
	for i := len(ctx.Messages) - 1; i >= 0; i-- {
		msg := ctx.Messages[i]
		if msg == nil {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}
		switch c := msg["content"].(type) {
		case string:
			return c
		case []any:
			var sb strings.Builder
			for _, block := range c {
				m, _ := block.(map[string]any)
				if m == nil {
					continue
				}
				if t, _ := m["type"].(string); t == "text" {
					if s, _ := m["text"].(string); s != "" {
						sb.WriteString(s)
					}
				}
			}
			return sb.String()
		}
		return ""
	}
	return ""
}

var _ processors.Processor = (*ExternalChainProcessor)(nil)
