// Package ivr provides IVRNavigator pipeline helper.
package ivr

import (
	"fmt"

	"voxray-go/pkg/pipeline"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/processors/voice"
	"voxray-go/pkg/services"
)

// Default classifier prompt for IVR vs conversation detection.
const ClassifierPrompt = `You are an IVR detection classifier. Analyze the transcribed text to determine if it's an automated IVR system or a live human conversation.

IVR SYSTEM (respond ` + "` ivr `" + `):
- Menu options: "Press 1 for billing", "Press 2 for technical support", "Press 0 to speak to an agent"
- Automated instructions: "Please enter your account number", "Say or press your selection", "Enter your phone number followed by the pound key"
- System prompts: "Thank you for calling [company]", "Your call is important to us", "Please hold while we connect you"
- Scripted introductions: "Welcome to [company] customer service", "For faster service, have your account number ready"
- Navigation phrases: "To return to the main menu", "Press star to repeat", "Say 'agent' or press 0"
- Hold messages: "Please continue to hold", "Your estimated wait time is", "Thank you for your patience"
- Carrier messages: "All circuits are busy", "Due to high call volume"

HUMAN CONVERSATION (respond ` + "` conversation `" + `):
- Personal greetings: "Hello, this is Sarah", "Good morning, how can I help you?", "Customer service, this is Mike"
- Interactive responses: "Who am I speaking with?", "What can I do for you today?", "How are you calling about?"
- Natural speech patterns: hesitations, informal language, conversational flow
- Direct engagement: "I see you're calling about...", "Let me look that up for you", "Can you spell that for me?"
- Spontaneous responses: "Oh, I can help with that", "Sure, no problem", "Hmm, let me check"

RESPOND ONLY with either:
- ` + "` ivr `" + ` for IVR system
- ` + "` conversation `" + ` for human conversation`

// IVRNavigationBase is the template for the IVR navigation prompt; use fmt with goal.
const IVRNavigationBase = `You are navigating an Interactive Voice Response (IVR) system to accomplish a specific goal. You receive text transcriptions of the IVR system's audio prompts and menu options.

YOUR NAVIGATION GOAL:
%s

NAVIGATION RULES:
1. When you see menu options with keypress instructions (e.g., "Press 1 for...", "Press 2 for..."), ONLY respond with a keypress if one of the options aligns with your navigation goal
2. If an option closely matches your goal, respond with: ` + "` NUMBER `" + ` (e.g. ` + "` 1 `" + `)
3. For sequences of numbers (dates, account numbers, phone numbers), enter each digit separately: ` + "` 1 2 3 `" + ` for "123"
4. When the system asks for verbal responses (e.g. "Say Yes or No", "Please state your name", "What department?"), respond with natural language text ending with punctuation
5. If multiple options seem relevant, choose the most specific or direct path
6. If NO options are relevant to your goal, respond with ` + "` wait `" + ` - the system may present more options
7. If the transcription is incomplete or unclear, respond with ` + "` wait `" + ` to indicate you need more information

COMPLETION CRITERIA - Respond with ` + "` completed `" + ` when:
- You see "Please hold while I transfer you" or similar transfer language
- You see "You're being connected to..." or "Connecting you to..."
- The system says "One moment please" after selecting your final option
- The system indicates you've reached the target department/service
- You've successfully navigated to your goal and are being transferred to a human

WAIT CRITERIA - Respond with ` + "` wait `" + ` when:
- NONE of the presented options are relevant to your navigation goal
- The transcription appears to be cut off mid-sentence
- You can see partial menu options but the list seems incomplete
- The transcription is unclear or garbled
- You suspect there are more options that weren't captured in the transcription
- The system presents options for specific user types that don't apply to your goal

STUCK CRITERIA - Respond with ` + "` stuck `" + ` when:
- You've been through the same menu options 3+ times without progress
- No available options relate to your goal after careful consideration
- You encounter an error message or "invalid selection" repeatedly
- The system asks for information you don't have (account numbers, PINs, etc.)
- You reach a dead end with no relevant options and no way back

Remember: Respond with ` + "` NUMBER `" + ` (single or multiple for sequences), ` + "` completed `" + `, ` + "` stuck `" + `, ` + "` wait `" + `, OR natural language text when verbal responses are requested. No other response types.`

// IVRNavigator wraps a pipeline of LLM + IVRProcessor for automated IVR navigation.
type IVRNavigator struct {
	Processor processors.Processor
	ivr       *IVRProcessor
}

// NewIVRNavigator creates an IVR navigator: pipeline [LLM, IVRProcessor] with default prompts.
// Goal is interpolated into IVRNavigationBase. ivrVADStopSecs is the VAD stop_secs in IVR mode (0 = 2.0).
func NewIVRNavigator(llm services.LLMService, goal string, ivrVADStopSecs float64) *IVRNavigator {
	ivrPrompt := fmt.Sprintf(IVRNavigationBase, goal)
	ivrProc := NewIVRProcessor("IVR", ClassifierPrompt, ivrPrompt, ivrVADStopSecs)
	llmProc := voice.NewLLMProcessorWithSystemPrompt("LLM", llm, "")
	llmProc.OnContextUpdate = func(msgs []map[string]any) {
		ivrProc.SetSavedMessages(msgs)
	}
	pl := pipeline.New()
	pl.Add(llmProc)
	pl.Add(ivrProc)
	pp := pipeline.NewPipelineProcessor("IVRNavigator", pl)
	return &IVRNavigator{Processor: pp, ivr: ivrProc}
}

// OnConversationDetected registers the callback on the underlying IVRProcessor.
func (n *IVRNavigator) OnConversationDetected(fn func(conversationHistory []map[string]any)) {
	n.ivr.OnConversationDetected(fn)
}

// OnIVRStatusChanged registers the callback on the underlying IVRProcessor.
func (n *IVRNavigator) OnIVRStatusChanged(fn func(status IVRStatus)) {
	n.ivr.OnIVRStatusChanged(fn)
}
