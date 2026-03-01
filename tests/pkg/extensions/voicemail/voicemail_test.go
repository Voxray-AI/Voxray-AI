package voicemail_test

import (
	"context"
	"testing"

	"voila-go/pkg/extensions/voicemail"
	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/services"
	"voila-go/pkg/sync/notifier"
)

// mockLLM implements services.LLMService for tests.
var _ services.LLMService = (*mockLLM)(nil)

type mockLLM struct {
	response string
}

func (m *mockLLM) Chat(ctx context.Context, messages []map[string]any, onToken func(*frames.LLMTextFrame)) error {
	for _, c := range m.response {
		onToken(&frames.LLMTextFrame{
			TextFrame: frames.TextFrame{
				DataFrame:       frames.DataFrame{Base: frames.NewBase()},
				Text:            string(c),
				AppendToContext: true,
			},
		})
	}
	return nil
}

func TestVoicemailDetector_DetectorAndGate(t *testing.T) {
	llm := &mockLLM{response: "CONVERSATION"}
	det := voicemail.NewVoicemailDetector(llm, 0.5)
	if det.Detector() == nil {
		t.Fatal("Detector() should not be nil")
	}
	if det.Gate() == nil {
		t.Fatal("Gate() should not be nil")
	}
}

func TestClassificationProcessor_Conversation(t *testing.T) {
	ctx := context.Background()
	gateNotifier := notifier.New()
	conversationNotifier := notifier.New()
	voicemailNotifier := notifier.New()
	proc := voicemail.NewClassificationProcessor("CP", gateNotifier, conversationNotifier, voicemailNotifier, 0.5)
	var convCalled bool
	proc.OnConversationDetected(func() { convCalled = true })
	pl := pipeline.New()
	pl.Add(proc)
	_ = pl.Setup(ctx)
	defer pl.Cleanup(ctx)
	_ = pl.Push(ctx, frames.NewLLMFullResponseStartFrame())
	_ = pl.Push(ctx, &frames.LLMTextFrame{TextFrame: frames.TextFrame{DataFrame: frames.DataFrame{Base: frames.NewBase()}, Text: "CONVERSATION", AppendToContext: true}})
	_ = pl.Push(ctx, frames.NewLLMFullResponseEndFrame())
	if !convCalled {
		t.Error("expected OnConversationDetected to be called")
	}
}
