// Package voicemail provides VoicemailDetector for outbound call classification.
package voicemail

import (
	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services"
	"voxray-go/pkg/sync/notifier"
)

// ClassifierResponseInstruction is the required suffix for custom prompts.
const ClassifierResponseInstruction = `Respond with ONLY "CONVERSATION" if a person answered, or "VOICEMAIL" if it's voicemail/recording.`

// DefaultClassifierPrompt is the default system prompt for voicemail classification.
const DefaultClassifierPrompt = `You are a voicemail detection classifier for an OUTBOUND calling system. A bot has called a phone number and you need to determine if a human answered or if the call went to voicemail based on the provided text.

HUMAN ANSWERED - LIVE CONVERSATION (respond "CONVERSATION"):
- Personal greetings: "Hello?", "Hi", "Yeah?", "John speaking"
- Interactive responses: "Who is this?", "What do you want?", "Can I help you?"
- Conversational tone expecting back-and-forth dialogue
- Questions directed at the caller: "Hello? Anyone there?"
- Informal responses: "Yep", "What's up?", "Speaking"
- Natural, spontaneous speech patterns
- Immediate acknowledgment of the call

VOICEMAIL SYSTEM (respond "VOICEMAIL"):
- Automated voicemail greetings: "Hi, you've reached [name], please leave a message"
- Phone carrier messages: "The number you have dialed is not in service", "Please leave a message", "All circuits are busy"
- Professional voicemail: "This is [name], I'm not available right now"
- Instructions about leaving messages: "leave a message", "leave your name and number"
- References to callback or messaging: "call me back", "I'll get back to you"
- Carrier system messages: "mailbox is full", "has not been set up"
- Business hours messages: "our office is currently closed"

` + ClassifierResponseInstruction

// VoicemailDetector builds a parallel pipeline for voicemail vs conversation classification
// and a TTS gate. Use Detector() in the main pipeline (e.g. after STT) and Gate() after TTS.
type VoicemailDetector struct {
	// DetectorBranch is the parallel pipeline to insert after STT (classification branch + conversation gate branch).
	DetectorBranch *pipeline.ParallelPipeline
	// GateProcessor is the TTS gate to insert after TTS in the main pipeline.
	GateProcessor *TTSGate
	classification *ClassificationProcessor
}

// NewVoicemailDetector creates a detector that uses the given LLM for classification.
// voicemailResponseDelaySecs is the delay before firing on_voicemail_detected (0 = 2.0).
func NewVoicemailDetector(llm services.LLMService, voicemailResponseDelaySecs float64) *VoicemailDetector {
	return NewVoicemailDetectorWithPrompt(llm, voicemailResponseDelaySecs, DefaultClassifierPrompt)
}

// NewVoicemailDetectorWithPrompt creates a detector with a custom classifier prompt.
// The prompt should instruct the LLM to respond with exactly "CONVERSATION" or "VOICEMAIL".
func NewVoicemailDetectorWithPrompt(llm services.LLMService, voicemailResponseDelaySecs float64, systemPrompt string) *VoicemailDetector {
	gateNotifier := notifier.New()
	conversationNotifier := notifier.New()
	voicemailNotifier := notifier.New()

	classifierGate := NewClassifierGate("ClassifierGate", gateNotifier, conversationNotifier)
	llmProc := voice.NewLLMProcessorWithSystemPrompt("VoicemailClassifierLLM", llm, systemPrompt)
	classProc := NewClassificationProcessor("ClassificationProcessor", gateNotifier, conversationNotifier, voicemailNotifier, voicemailResponseDelaySecs)

	conversationGate := NewConversationGate("ConversationGate", voicemailNotifier)
	ttsGate := NewTTSGate("TTSGate", conversationNotifier, voicemailNotifier)

	// Branch 1: conversation gate (blocks when voicemail detected)
	// Branch 2: classifier gate -> LLM -> classification processor
	parallel, err := pipeline.NewParallelPipeline([][]processors.Processor{
		{conversationGate},
		{classifierGate, llmProc, classProc},
	})
	if err != nil {
		panic(err)
	}

	return &VoicemailDetector{
		DetectorBranch:  parallel,
		GateProcessor:   ttsGate,
		classification:  classProc,
	}
}

// OnConversationDetected sets the callback when CONVERSATION is classified.
func (d *VoicemailDetector) OnConversationDetected(fn func()) {
	if d.classification != nil {
		d.classification.OnConversationDetected(fn)
	}
}

// OnVoicemailDetected sets the callback when VOICEMAIL is classified (after delay).
func (d *VoicemailDetector) OnVoicemailDetected(fn func()) {
	if d.classification != nil {
		d.classification.OnVoicemailDetected(fn)
	}
}

// Detector returns the parallel pipeline processor to add to the main pipeline (e.g. after STT).
func (d *VoicemailDetector) Detector() *pipeline.ParallelPipeline {
	return d.DetectorBranch
}

// Gate returns the TTS gate processor to add after TTS in the main pipeline.
func (d *VoicemailDetector) Gate() *TTSGate {
	return d.GateProcessor
}
