package qwen

import "testing"

func TestBuild(t *testing.T) {
	// Ensures the qwen package compiles and NewLLMService can be called.
	_ = NewLLMService("", "qwen-plus")
	_ = NewClient("")
}
