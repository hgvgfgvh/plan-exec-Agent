package skillwait

import (
	"strings"
	"sync"
)

// 按回合缓存 TopicExecResult，避免「先 Publish、后 Subscribe」导致结果丢失。
var resultByTurn sync.Map

// RecordResult 在发布 TopicExecResult 时写入缓存（同 turnID 仅保留最新一次）。
func RecordResult(turnID string, payload interface{}) {
	if turnID == "" || payload == nil {
		return
	}
	resultByTurn.Store(turnID, payload)
}

// PeekCachedResult 读取本回合缓存的技能输出（不删除）。
func PeekCachedResult(turnID string) (text string, ok bool) {
	if turnID == "" {
		return "", false
	}
	v, loaded := resultByTurn.Load(turnID)
	if !loaded {
		return "", false
	}
	s := formatOutput(v)
	return s, strings.TrimSpace(s) != ""
}

// TakeCachedResult 取出并删除该回合缓存的技能输出；无缓存时 ok=false。
func TakeCachedResult(turnID string) (text string, ok bool) {
	if turnID == "" {
		return "", false
	}
	v, loaded := resultByTurn.LoadAndDelete(turnID)
	if !loaded {
		return "", false
	}
	s := formatOutput(v)
	return s, strings.TrimSpace(s) != ""
}
