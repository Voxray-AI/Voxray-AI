package google

import "testing"

func TestMessagesToContents(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hello"},
		{"role": "assistant", "content": "Hi there"},
		{"role": "user", "content": "Bye"},
	}
	contents, system := messagesToContents(messages)
	if system != "" {
		t.Errorf("expected no system instruction, got %q", system)
	}
	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}
	if contents[0].Role != "user" || contents[0].Parts[0].Text != "Hello" {
		t.Errorf("first content: role=%q text=%q", contents[0].Role, contents[0].Parts[0].Text)
	}
	if contents[1].Role != "model" || contents[1].Parts[0].Text != "Hi there" {
		t.Errorf("second content: role=%q text=%q", contents[1].Role, contents[1].Parts[0].Text)
	}
}

func TestMessagesToContents_System(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "You are helpful."},
		{"role": "user", "content": "Hi"},
	}
	contents, system := messagesToContents(messages)
	if system != "You are helpful." {
		t.Errorf("expected system instruction %q, got %q", "You are helpful.", system)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content (system excluded), got %d", len(contents))
	}
	if contents[0].Parts[0].Text != "Hi" {
		t.Errorf("content text=%q", contents[0].Parts[0].Text)
	}
}

func TestRecognizerName(t *testing.T) {
	s := &STTService{project: "my-project", location: "us-central1"}
	name := s.recognizerName()
	expected := "projects/my-project/locations/us-central1/recognizers/_"
	if name != expected {
		t.Errorf("recognizer name: got %q, want %q", name, expected)
	}
}
