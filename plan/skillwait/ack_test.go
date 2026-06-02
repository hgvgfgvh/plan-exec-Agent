package skillwait

import (
	"strings"
	"testing"
)

func TestMustWaitAfterToolBatch_batchedObservation(t *testing.T) {
	obs := "【多个工具并行返回结果】:\n1. 工具 [SetExecutorStep] 结果: 已接收：后台异步执行内置技能 \"SeeCameraAndDescribe\"。请通过 exec 状态/结果观察进展。\n"
	if !MustWaitAfterToolBatch([]string{"SetExecutorStep"}, obs) {
		t.Fatal("batched SetExecutorStep ack must trigger wait")
	}
}

func TestMustWaitAfterToolBatch_afterSkillResult(t *testing.T) {
	obs := "【内置技能执行结果】\n画面中有显示器\n"
	if MustWaitAfterToolBatch([]string{"SetExecutorStep"}, obs) {
		t.Fatal("must not wait when skill result already injected")
	}
}

func TestIsPlaceholderSkillSummary(t *testing.T) {
	ack := `已接收：后台异步执行内置技能 "SeeCameraAndDescribe"。请通过 exec 状态/结果观察进展。`
	if !IsPlaceholderSkillSummary(ack) {
		t.Fatal("tool ack should be placeholder")
	}
	real := strings.Repeat("画面描述", 40)
	if IsPlaceholderSkillSummary(real) {
		t.Fatal("long real summary should not be placeholder")
	}
}
