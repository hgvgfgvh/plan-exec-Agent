package blackboard

import (
	"sync"
)

// Message 为黑板内传递的事件。
//
// 元数据字段（TurnID / Hop）用于追踪「同一用户回合内 Agent 之间的反射跳数」，
// Router 据此对反思回路施加跳数预算，避免 Affective ↔ Router ↔ Behavior 之间
// 通过 LLM 工具调用形成的无界放大。
//
//   - TurnID：用户当前回合的稳定 ID（由 runcontrol.BeginTurn 生成）；
//     不属于任何回合的事件（启动期、外部信号等）可为空串。
//   - Hop：本消息已经历的「Agent→Agent」反射跳数；初次入口为 0，
//     每次 Router 反思转发时 +1。
type Message struct {
	Topic   string
	Payload interface{}

	// 元信息（向下兼容：旧 Publish 不填则均为零值）。
	TurnID string
	Hop    int
}

type Blackboard struct {
	mu sync.RWMutex
	// 主题与通道列表的映射：Topic -> []SubscriberChannels
	subscribers map[string][]chan Message
}

var (
	instance *Blackboard
	once     sync.Once
)

// GetInstance 获取全局单例
func GetInstance() *Blackboard {
	once.Do(func() {
		instance = &Blackboard{
			subscribers: make(map[string][]chan Message),
		}
	})
	return instance
}

// Subscribe 订阅某个主题，返回一个只读通道
func (b *Blackboard) Subscribe(topic string, bufferSize int) <-chan Message {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, bufferSize)
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}

// Publish 向某个主题发布消息（无元信息；TurnID="" / Hop=0）。
// 兼容旧调用方；新代码请优先使用 PublishMsg，以便携带跳数预算所需的元信息。
func (b *Blackboard) Publish(topic string, payload interface{}) {
	b.dispatch(Message{Topic: topic, Payload: payload})
}

// PublishMsg 发布一条带元信息的消息（推荐路径，用于跨 Agent 反射链）。
func (b *Blackboard) PublishMsg(topic string, payload interface{}, turnID string, hop int) {
	b.dispatch(Message{Topic: topic, Payload: payload, TurnID: turnID, Hop: hop})
}

func (b *Blackboard) dispatch(msg Message) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	subs, ok := b.subscribers[msg.Topic]
	if !ok {
		return
	}
	for _, ch := range subs {
		// 使用 select 防止某个订阅者阻塞导致全局卡死；通道满时记为丢包。
		select {
		case ch <- msg:
		default:
		}
	}
}

// 目前已使用的通道见 topics.go；所有发布/订阅请使用其中定义的常量，禁止字面量。
// 主要语义：
//   - TopicExecStepEvent / TopicExecStatus / TopicExecResult : skill 执行链上报
//   - TopicEnvChange                                         : 环境信息上报
//   - TopicFacadeOutput                                      : 情感直接交互agent 在异步调度中 回复给用户的消息
//   - TopicAffective{Dispatch,Input,Output} / TopicBehavior{Input,Output}
//                                                            : 多 agent 协作消息路由
//   - TopicAgentAbort                                        : 控制中止
//
// 反思路径示意：
//   Affective.output -> Router.handleFeedback -> Affective.input (旧 .iutput，已统一)
//   Behavior.output  -> Router.handleFeedback -> Behavior.input
