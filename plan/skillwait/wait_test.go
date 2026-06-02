package skillwait

import "testing"

func TestNeedsWait_toolAckOnly(t *testing.T) {
	ack := `已接收：后台异步执行内置技能 "SeeCameraAndDescribe"。请通过 exec 状态/结果观察进展。`
	if !NeedsWait(ack) {
		t.Fatal("expected tool ack to need wait")
	}
}

func TestNeedsWait_modelMentionNotTrigger(t *testing.T) {
	model := `技能已提交到后台执行，但目前只收到了"已接收"的确认。
SetExecutorStep 为异步调用模式，返回的是"已接收后台执行"的确认，尚未返回摄像头画面。`
	if NeedsWait(model) {
		t.Fatal("model prose mentioning 已接收 must not trigger NeedsWait")
	}
}

func TestTakeCachedResult(t *testing.T) {
	RecordResult("t-test", "画面中有书桌与显示器")
	text, ok := TakeCachedResult("t-test")
	if !ok || text != "画面中有书桌与显示器" {
		t.Fatalf("cache miss: ok=%v text=%q", ok, text)
	}
	_, again := TakeCachedResult("t-test")
	if again {
		t.Fatal("expected cache consumed")
	}
}
