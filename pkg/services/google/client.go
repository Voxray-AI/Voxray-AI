// Package google provides Google Gemini LLM, Vertex AI LLM, and Google Cloud STT/TTS services.
package google

import (
	"context"

	"voila-go/pkg/config"

	"google.golang.org/genai"
)

// NewClient creates a Gemini API client with the given API key.
// If apiKey is empty, config.GetEnv("GOOGLE_API_KEY", "") is used.
func NewClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	if apiKey == "" {
		apiKey = config.GetEnv("GOOGLE_API_KEY", "")
	}
	if apiKey == "" {
		apiKey = config.GetEnv("GEMINI_API_KEY", "")
	}
	return genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
}

// NewVertexClient creates a Vertex AI client using Application Default Credentials.
// project and location must be set (e.g. "my-project", "us-central1").
// If project or location is empty, config env GOOGLE_CLOUD_PROJECT / GOOGLE_CLOUD_LOCATION are used.
func NewVertexClient(ctx context.Context, project, location string) (*genai.Client, error) {
	if project == "" {
		project = config.GetEnv("GOOGLE_CLOUD_PROJECT", "")
	}
	if location == "" {
		location = config.GetEnv("GOOGLE_CLOUD_LOCATION", "us-central1")
	}
	return genai.NewClient(ctx, &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
}
