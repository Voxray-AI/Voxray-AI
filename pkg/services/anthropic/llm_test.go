package anthropic

import "testing"

func TestNewLLMService_UsesDefaultModelWhenEmpty(t *testing.T) {
	svc := NewLLMService("test-key", "")
	if svc == nil {
		t.Fatal("NewLLMService returned nil")
	}
	if svc.model != DefaultLLMModel {
		t.Fatalf("expected default model %q, got %q", DefaultLLMModel, svc.model)
	}
}

func TestNewLLMService_UsesProvidedModel(t *testing.T) {
	const customModel = "claude-3-5-haiku-latest"
	svc := NewLLMService("test-key", customModel)
	if svc == nil {
		t.Fatal("NewLLMService returned nil")
	}
	if svc.model != customModel {
		t.Fatalf("expected model %q, got %q", customModel, svc.model)
	}
}

