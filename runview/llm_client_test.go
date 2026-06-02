package runview

import "testing"

func TestNormalizeChatCompletionsEndpoint(t *testing.T) {
	got := normalizeChatCompletionsEndpoint("https://api.deepseek.com")
	want := "https://api.deepseek.com/chat/completions"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2 := normalizeChatCompletionsEndpoint("https://api.openai.com/v1/chat/completions")
	if got2 != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("got %q", got2)
	}
}
