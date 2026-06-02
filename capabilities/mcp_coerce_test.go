package capabilities

import (
	"encoding/json"
	"testing"
)

func TestCoerceMCPArguments_ResendToString(t *testing.T) {
	args := map[string]any{
		"to":      "2563726816@qq.com",
		"from":    "onboarding@resend.dev",
		"subject": "测试",
	}
	CoerceMCPArguments("resend__send_email", args)
	raw, _ := json.Marshal(args["to"])
	if string(raw) != `["2563726816@qq.com"]` {
		t.Fatalf("to=%v", args["to"])
	}
}
