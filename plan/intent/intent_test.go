package intent

import "testing"

func TestPromptMaxStepsPositive(t *testing.T) {
	if DefaultPromptMaxSteps < 1 {
		t.Fatal("DefaultPromptMaxSteps must be positive")
	}
}
