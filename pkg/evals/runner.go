package evals

import (
	"context"
	"os"
	"regexp"
	"strings"
	"time"

	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors/voice"
	"voila-go/pkg/services"
)

// DefaultEvalTimeout is used when scenario TimeoutSecs is zero.
const DefaultEvalTimeout = 60 * time.Second

// RunScenario runs a single eval scenario: builds an LLM-only pipeline, injects the prompt
// as a TranscriptionFrame, collects LLM text output, and asserts using ExpectedPattern (regex)
// or ExpectedContains (substring).
func RunScenario(ctx context.Context, voilaConfigPath string, scenario EvalScenario) EvalResult {
	start := time.Now()
	result := EvalResult{Name: scenario.Name}

	cfg, err := config.LoadConfig(voilaConfigPath)
	if err != nil {
		result.Pass = false
		result.Error = err.Error()
		result.Duration = time.Since(start).Seconds()
		return result
	}

	// Ensure we have an LLM provider and model
	provider := cfg.LLMProvider()
	if provider == "" {
		provider = services.ProviderOpenAI
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = make(map[string]string)
	}
	// Prefer env for common keys so evals work without editing config
	for _, kv := range []struct{ key, env string }{
		{"openai", "OPENAI_API_KEY"},
		{"groq", "GROQ_API_KEY"},
	} {
		if cfg.APIKeys[kv.key] == "" && os.Getenv(kv.env) != "" {
			cfg.APIKeys[kv.key] = os.Getenv(kv.env)
		}
	}

	llmSvc := services.NewLLMFromConfig(cfg, provider, model)
	systemPrompt := scenario.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant. Reply briefly and directly."
	}
	llmProc := voice.NewLLMProcessorWithSystemPrompt("llm", llmSvc, systemPrompt)

	outCh := make(chan frames.Frame, 64)
	sink := pipeline.NewSink("sink", outCh)

	pl := pipeline.New()
	pl.Link(llmProc, sink)

	timeout := DefaultEvalTimeout
	if scenario.TimeoutSecs > 0 {
		timeout = time.Duration(scenario.TimeoutSecs * float64(time.Second))
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := pl.Setup(runCtx); err != nil {
		result.Pass = false
		result.Error = err.Error()
		result.Duration = time.Since(start).Seconds()
		return result
	}
	defer pl.Cleanup(runCtx)

	if err := pl.Start(runCtx, frames.NewStartFrame()); err != nil {
		result.Pass = false
		result.Error = err.Error()
		result.Duration = time.Since(start).Seconds()
		return result
	}

	tf := frames.NewTranscriptionFrame(scenario.Prompt, "", "", true)
	if err := pl.Push(runCtx, tf); err != nil {
		result.Pass = false
		result.Error = err.Error()
		result.Duration = time.Since(start).Seconds()
		return result
	}

	var output strings.Builder
	for {
		select {
		case <-runCtx.Done():
			result.Pass = false
			result.Output = output.String()
			result.Error = "timeout waiting for LLM response"
			result.Duration = time.Since(start).Seconds()
			return result
		case f := <-outCh:
			switch v := f.(type) {
			case *frames.LLMTextFrame:
				output.WriteString(v.Text)
			case *frames.EndFrame, *frames.TTSSpeakFrame:
				// EndFrame or TTSSpeakFrame (empty flush) signals LLM turn complete
				result.Output = output.String()
				result.Pass = assertOutput(result.Output, scenario)
				result.Duration = time.Since(start).Seconds()
				return result
			case *frames.ErrorFrame:
				result.Pass = false
				result.Output = output.String()
				result.Error = v.Error
				result.Duration = time.Since(start).Seconds()
				return result
			case *frames.CancelFrame:
				result.Pass = false
				result.Output = output.String()
				result.Error = "cancelled"
				result.Duration = time.Since(start).Seconds()
				return result
			}
		}
	}
}

func assertOutput(output string, s EvalScenario) bool {
	output = strings.TrimSpace(strings.ToLower(output))
	if s.ExpectedContains != "" {
		return strings.Contains(output, strings.ToLower(s.ExpectedContains))
	}
	if s.ExpectedPattern != "" {
		re, err := regexp.Compile("(?i)" + s.ExpectedPattern)
		if err != nil {
			return false
		}
		return re.MatchString(output)
	}
	return false
}
