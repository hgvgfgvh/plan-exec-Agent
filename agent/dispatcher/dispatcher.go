// Package dispatcher 是「丘脑路由」的确定性分流层。
//
// 设计动机：
//
//	原 RouterAgent 把「该交给谁」这件事完全交给 LLM Action 输出来决定，导致：
//	  - 业务正确性绑定到概率系统（模型抖动 → 路由错 / 漏 / 重）；
//	  - 必须靠 prompt 兜底 + 空输出 retry 补救（见 routerAgent.go 的「系统纠偏」逻辑）；
//	  - 无法被回放/单测覆盖。
//
// 改造思路：
//
//	大多数用户输入「该路由到哪个脑分区」是确定的——"打开记事本"显然是 Behavior，
//	"什么是 RAG" 显然是 Affective。只有少数模糊语句需要 LLM 来判。
//	因此把分流拆为两层：
//	  1. 规则分类器（本包）：高置信度直接出 Decision，无 LLM 调用；
//	  2. LLM 兜底（RouterAgent）：仅当本层判定为 Ambiguous 才启用现有 Executor.Run 路径。
//
// 保守原则：宁可误报为 Ambiguous（多一次 LLM 调用），也不可误报为高置信（路由错）。
// 因此当某条输入同时撞中两侧规则时，返回 Both 而不是 Ambiguous——这等价于
// LLM 路径下「同时调用两个 Action」的行为，业务等价但更便宜。
package dispatcher

import (
	"regexp"
	"strings"
)

// Target 是分流目标。
type Target string

const (
	TargetAffective Target = "affective" // 情感直接交互脑分区
	TargetBehavior  Target = "behavior"  // 行为编排调度脑分区
)

// Confidence 表示本次判定的置信度；低置信由 LLM 兜底。
type Confidence int

const (
	LowConfidence  Confidence = iota // 触发 LLM 兜底
	HighConfidence                   // 直接走快路径
)

// Decision 是分类器的输出。
//
// Targets 在 HighConfidence 时给出 1 或 2 个目标；LowConfidence 时为空。
type Decision struct {
	Targets    []Target
	Confidence Confidence
	Reason     string
}

// 行为类（执行/操作/IO）关键词。命中即视为 Behavior 信号。
//
// 收录原则：
//   - 高频且歧义低的命令式动词；
//   - 与 abilities.yml 中已注册能力（Office_Domain / Active_Domain / See_Domain / Communication_Domain / WebApi_Domain）直接对应；
//   - PowerShell / Get-* / Set-* / Invoke-* 等 cmdlet 命名模式。
//
// 新增能力时应同步在此追加正则。
var behaviorPatterns = []*regexp.Regexp{
	// 英文动词
	regexp.MustCompile(`(?i)\b(open|run|launch|execute|kill|stop|close|click|drag|drop|type|press|screenshot|capture|search\s+web|send\s+(an?\s+)?email)\b`),
	// shell 与 cmdlet
	regexp.MustCompile(`(?i)\b(powershell|cmd|bash|sh|invoke-\w+|get-\w+|set-\w+|new-item|remove-item|start-process)\b`),
	// 中文动词：打开/运行/启动/关闭/执行/点击/双击/拖动/输入/按键/截图/截屏
	regexp.MustCompile(`(?:打开|运行|启动|执行|跑一下|关闭|杀掉|结束进程|点一下|点击|双击|拖动|拖拽|按下|输入文本|截图|截屏|看一下屏幕|查看屏幕)`),
	// 办公自动化
	regexp.MustCompile(`(?:生成|写一份|帮我做)\s*(?:Word|PPT|PPTX|文档|幻灯片|报告)`),
	// 邮件
	regexp.MustCompile(`(?:发(?:送)?(?:一封)?邮件|发邮件|发送邮件)`),
	// 文件系统
	regexp.MustCompile(`(?:读取文件|创建文件|新建文件|删除文件|移动文件|查找文件|找一下文件|查一下文件|列出文件)`),
	// 联网检索（注意：用户问"什么是 X"不在此范围；这里专指明确的"搜一下/上网查"）
	regexp.MustCompile(`(?:上网搜|联网搜|帮我搜一下|搜索一下|查一下网上|去网上)`),
}

// 对话类（情感/解释/问候/知识问答）关键词。命中即视为 Affective 信号。
var affectivePatterns = []*regexp.Regexp{
	// 问候与寒暄
	regexp.MustCompile(`(?i)^\s*(hi|hello|hey|yo|thanks|thank\s+you|bye)\b`),
	regexp.MustCompile(`^\s*(你好|嗨|哈喽|哈罗|谢谢|多谢|拜拜|再见|早安|晚安)`),
	// 知识问答词头
	regexp.MustCompile(`(?:什么是|啥是|是什么意思|为什么|怎么(?:回事|理解|看待)|如何看待|介绍一下|解释一下|讲讲|说说)`),
	// 主观提问
	regexp.MustCompile(`(?:你觉得|你认为|你怎么看|你喜欢|你讨厌|有什么(?:看法|想法|建议))`),
	// 情绪 / 关系
	regexp.MustCompile(`(?:我好(?:开心|难过|累|烦)|心情|想聊聊|陪我说说话)`),
}

// Classify 对 raw 用户输入做确定性分类。
//
// 返回值语义：
//
//	HighConfidence + 1 target  →  直接路由
//	HighConfidence + 2 targets →  同时路由（命中两侧规则，由两个分区各自处理）
//	LowConfidence              →  Targets 为空；调用方应回退到 LLM 路由
func Classify(raw string) Decision {
	input := strings.TrimSpace(raw)
	if input == "" {
		return Decision{Confidence: LowConfidence, Reason: "empty input"}
	}

	beh := matchAny(input, behaviorPatterns)
	aff := matchAny(input, affectivePatterns)

	switch {
	case beh && aff:
		return Decision{
			Targets:    []Target{TargetBehavior, TargetAffective},
			Confidence: HighConfidence,
			Reason:     "命中执行 + 对话双信号",
		}
	case beh:
		return Decision{
			Targets:    []Target{TargetBehavior},
			Confidence: HighConfidence,
			Reason:     "命中执行类关键词",
		}
	case aff:
		return Decision{
			Targets:    []Target{TargetAffective},
			Confidence: HighConfidence,
			Reason:     "命中对话/知识类关键词",
		}
	default:
		return Decision{
			Confidence: LowConfidence,
			Reason:     "未命中规则，需 LLM 兜底",
		}
	}
}

func matchAny(s string, ps []*regexp.Regexp) bool {
	for _, p := range ps {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}
