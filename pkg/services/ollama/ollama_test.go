package ollama

import "testing"

func TestBuild(t *testing.T) {
	// Ensures the ollama package compiles and NewLLMService can be called.
	_ = NewLLMService("", "llama3.2")
	_ = NewClient("")
}
