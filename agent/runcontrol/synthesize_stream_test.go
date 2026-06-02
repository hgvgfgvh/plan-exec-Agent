package runcontrol

import (
	"context"
	"testing"
)

func TestSynthesizeStreamTurnMap_visibleFromParentCtx(t *testing.T) {
	parent := BeginTurn(context.Background(), "hi")
	turnID, _ := TurnMetaFromContext(parent)
	if turnID == "" {
		t.Fatal("expected turn id on BeginTurn ctx")
	}
	child := BeginSynthesizeStream(parent, turnID, turnID+"-synthesize")
	_ = child
	if !SynthesizeStreamed(parent) {
		t.Fatal("portal turnCtx should see stream mark via turn map")
	}
	if got := SynthesizeStreamMessageID(parent); got != turnID+"-synthesize" {
		t.Fatalf("messageID: got %q", got)
	}
	ClearSynthesizeStream(turnID)
	if SynthesizeStreamed(parent) {
		t.Fatal("expected cleared")
	}
}

func TestBeginTurn_clearsPreviousStreamMark(t *testing.T) {
	ctx1 := BeginTurn(context.Background(), "a")
	id1, _ := TurnMetaFromContext(ctx1)
	MarkSynthesizeStreamActive(id1, id1+"-synthesize")
	ctx2 := BeginTurn(context.Background(), "b")
	if SynthesizeStreamed(ctx1) {
		t.Fatal("new BeginTurn should clear previous turn stream mark")
	}
	_ = ctx2
}
