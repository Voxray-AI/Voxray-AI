# Extensions

This document describes the **IVR** and **Voicemail** extensions and how to use them in voila-go pipelines.

---

## IVR Extension

The IVR (Interactive Voice Response) extension automates navigation of phone IVR menus using an LLM to decide when to send DTMF keypresses and when to speak.

### Components

- **`pkg/extensions/ivr`**
  - **IVRProcessor**: Frame processor that consumes LLM text, detects XML-style tags (e.g. `` ` 1 ` `` for DTMF, `` ` completed ` `` for status), and emits `OutputDTMFUrgentFrame`, `VADParamsUpdateFrame`, and `LLMMessagesUpdateFrame` as needed.
  - **IVRNavigator**: Pipeline helper that builds a chain `[LLM, IVRProcessor]` with default classifier and IVR prompts, and wires context updates for mode switching.

### Pipeline order

IVR sits **downstream of the LLM**. When the IVR processor needs to switch mode (e.g. to IVR navigation) or update the LLM context, it pushes frames **upstream** (e.g. `LLMMessagesUpdateFrame`, `VADParamsUpdateFrame`). The **Turn** processor (upstream of the LLM) handles `VADParamsUpdateFrame` to adjust silence timeouts (e.g. shorter for IVR).

### Usage

```go
import (
    "voila-go/pkg/extensions/ivr"
    "voila-go/pkg/pipeline"
    "voila-go/pkg/processors/voice"
    "voila-go/pkg/services"
)

// Build a pipeline: Turn -> STT -> IVRNavigator -> TTS -> Sink
llm := ... // services.LLMService
navigator := ivr.NewIVRNavigator(llm, "Reach billing department", 2.0)
navigator.OnConversationDetected(func(history []map[string]any) {
    // Human detected; handle conversation (e.g. hand off to normal bot).
})
navigator.OnIVRStatusChanged(func(status ivr.IVRStatus) {
    // status is ivr.IVRStatusDetected, IVRStatusCompleted, IVRStatusStuck, or IVRStatusWait.
})

pl := pipeline.New()
pl.Add(turnProc)
pl.Add(sttProc)
pl.Add(navigator.Processor) // single processor that runs [LLM, IVRProcessor]
pl.Add(ttsProc)
pl.Add(sink)
```

### LLM context capture

For mode switching, the IVR processor needs the current conversation history. Wire `LLMProcessor.OnContextUpdate` to `IVRProcessor.SetSavedMessages` when building the pipeline (IVRNavigator does this internally for its own LLM).

---

## Voicemail Extension

The Voicemail extension classifies outbound calls as **human (CONVERSATION)** or **voicemail (VOICEMAIL)** using a parallel pipeline and gates TTS output until the classification is done.

### Components

- **`pkg/sync/notifier`**: One-shot notifier used by gates.
- **`pkg/extensions/voicemail`**
  - **NotifierGate**, **ClassifierGate**, **ConversationGate**: Gates that open/close based on notifier signals.
  - **TTSGate**: Buffers TTS frames until conversation or voicemail is detected; on conversation it releases them, on voicemail it discards them.
  - **ClassificationProcessor**: Aggregates LLM output and triggers notifiers + callbacks when it sees "CONVERSATION" or "VOICEMAIL".
  - **VoicemailDetector**: Builds a `ParallelPipeline` (conversation gate branch + classification branch) and a TTS gate; exposes `Detector()` and `Gate()`.

### Pipeline order

Insert the **detector** (parallel pipeline) after STT and before the main LLM/context. Insert the **gate** after TTS and before transport output. Example:

```
Transport -> Turn -> STT -> [Detector] -> ... -> LLM -> TTS -> [Gate] -> Transport
```

The detector runs two branches in parallel: one that only applies the conversation gate (blocking when voicemail is detected), and one that runs the classifier LLM and ClassificationProcessor. The TTS gate holds TTS frames until the classifier decides CONVERSATION or VOICEMAIL.

### Usage

```go
import (
    "voila-go/pkg/extensions/voicemail"
    "voila-go/pkg/pipeline"
)

classifierLLM := ... // fast LLM for classification
det := voicemail.NewVoicemailDetector(classifierLLM, 2.0)
det.OnConversationDetected(func() {
    // Human answered; normal flow continues.
})
det.OnVoicemailDetected(func() {
    // Voicemail; e.g. push TTSSpeakFrame("Please leave a message after the beep.")
})

pl := pipeline.New()
pl.Add(transportSource)
pl.Add(turnProc)
pl.Add(sttProc)
pl.Add(det.Detector())   // parallel pipeline (classification + conversation gate)
pl.Add(contextAggregator)
pl.Add(mainLLM)
pl.Add(ttsProc)
pl.Add(det.Gate())       // TTS gate
pl.Add(transportSink)
```

### Custom classifier prompt

Use `NewVoicemailDetectorWithPrompt(llm, delaySecs, prompt)`. The prompt must instruct the LLM to respond with exactly **"CONVERSATION"** or **"VOICEMAIL"**. Append `voicemail.ClassifierResponseInstruction` if you customize the prompt.

---

## Frames used by extensions

| Frame | Purpose |
|-------|---------|
| `VADParamsUpdateFrame` | Update turn/VAD params (e.g. stop_secs for IVR). Handled by Turn processor. |
| `LLMMessagesUpdateFrame` | Replace LLM context and optionally run LLM. Handled by LLM processor. |
| `OutputDTMFUrgentFrame` | Emit a DTMF key (0-9, *, #). Transport or telephony layer plays it. |
| `AggregatedTextFrame` | Text emitted after pattern aggregation (e.g. by IVR). |
| `LLMFullResponseStartFrame` / `LLMFullResponseEndFrame` | Delimit full LLM response for aggregation (IVR, voicemail). |

---

## Event hooks summary

| Extension | Hook | When |
|-----------|------|------|
| IVR | `OnConversationDetected(history)` | Classifier decides human conversation. |
| IVR | `OnIVRStatusChanged(status)` | IVR status: Detected, Completed, Stuck, Wait. |
| Voicemail | `OnConversationDetected()` | Classifier decides human answered. |
| Voicemail | `OnVoicemailDetected()` | Classifier decides voicemail (after configurable delay). |
