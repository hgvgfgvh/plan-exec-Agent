package memoryhook

import "testing"

func TestExtractHintsField_andParseMatch(t *testing.T) {
	raw := `{"hints":"[exec_simple_match=yes confidence=0.88]\n【记忆命中】\n","skipped":"false","phase":"test-fixture"}`
	body := extractHintsField(raw)
	exp := parseExperienceFromHints(body)
	if !exp.Matched {
		t.Fatalf("want matched, body=%q", body)
	}
	if exp.Confidence < 0.87 {
		t.Fatalf("want conf>=0.87, got %v", exp.Confidence)
	}
}
