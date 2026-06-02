package sendTopic

import (
	"AgentTest/body/blackboard"
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
)

// 字节级 FacadeOutput 去重：当同一回合内若干通道（Affective 反思、Behavior 直送、
// FacadeInteraction 工具调用、错误兜底等）产生「内容完全相同」的用户可见消息时，
// 仅放第一条通过，其余静默吞掉。
//
// 这不是模糊语义去重——两条措辞不同但语义重复的反思（如「已收到…」vs「已确认…」）
// 由 hop 预算 + 反思模式 prompt 指令处理，不靠本组件。
//
// 选用滑动窗口而非永久 set：长会话累计内容会让 set 无限膨胀；窗口短而紧凑足够覆盖
// 「上一条刚说完，紧接着又冒出来一条同字面」这种短期重复模式。

const facadeDedupHistorySize = 16

var (
	facadeDedupMu     sync.Mutex
	facadeDedupRecent []string // 最近 N 条 (turnID|sha1(payload)) 签名
)

var reFacadeXMLTag = regexp.MustCompile(`(?is)</?\s*[A-Za-z][A-Za-z0-9_]*\s*>`)

// SanitizeFacadeText 清洗用户可见文本（控制符、模型回吐的 XML 标签等）。
func SanitizeFacadeText(s string) string {
	s = StripControlTokens(s)
	s = reFacadeXMLTag.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// ResetFacadeDedup 新用户回合开始时清空去重窗口，避免跨回合误吞。
func ResetFacadeDedup() {
	facadeDedupMu.Lock()
	facadeDedupRecent = nil
	facadeDedupMu.Unlock()
}

// PublishFacadeDedup 向 TopicFacadeOutput 发布 payload，命中近窗口同签名时静默丢弃。
// turnID 可为空（用于无回合元信息的兜底分支），此时仅以 payload 哈希去重。
// 返回 true 表示已实际发布，false 表示被去重吞掉。
func PublishFacadeDedup(turnID, payload string) bool {
	trimmed := SanitizeFacadeText(payload)
	if trimmed == "" {
		return false
	}
	sig := turnID + "|" + sha1Hex(trimmed)
	facadeDedupMu.Lock()
	for _, s := range facadeDedupRecent {
		if s == sig {
			facadeDedupMu.Unlock()
			return false
		}
	}
	facadeDedupRecent = append(facadeDedupRecent, sig)
	if len(facadeDedupRecent) > facadeDedupHistorySize {
		facadeDedupRecent = facadeDedupRecent[len(facadeDedupRecent)-facadeDedupHistorySize:]
	}
	facadeDedupMu.Unlock()
	blackboard.GetInstance().Publish(blackboard.TopicFacadeOutput, payload)
	return true
}

func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// StripControlTokens 移除模型可能回吐的特殊控制符（如 <｜endofsentence｜>、<｜Assistant｜>
// 等 DeepSeek/Qwen 系列在 buildPrompt 中用作分隔符的 token）。这些符号一旦出现在
// 用户可见文本里非常出戏。
//
// 之所以放在 sendTopic 包：所有面向用户的文本最终都走这里出口，集中清洗一次最稳。
func StripControlTokens(s string) string {
	// 这两种 Unicode 全宽竖线分别是 U+FF5C 与 U+007C（少数模型会输出 ASCII 竖线变体）。
	for _, open := range []string{"<｜", "<|"} {
		for {
			i := strings.Index(s, open)
			if i < 0 {
				break
			}
			j := strings.Index(s[i:], "｜>")
			k := strings.Index(s[i:], "|>")
			end := -1
			switch {
			case j >= 0 && k >= 0:
				if j < k {
					end = i + j + len("｜>")
				} else {
					end = i + k + len("|>")
				}
			case j >= 0:
				end = i + j + len("｜>")
			case k >= 0:
				end = i + k + len("|>")
			default:
				// 开了没关，剩余整段都剥掉以免污染用户屏幕
				s = s[:i]
				return s
			}
			s = s[:i] + s[end:]
		}
	}
	return s
}
