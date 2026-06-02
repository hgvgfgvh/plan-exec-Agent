package outputbus

import (
	"fmt"
	"sync"
)

// Entry 为推送到前端（或他处）的一条统一输出。
type Entry struct {
	Source    string `json:"source"`
	Text      string `json:"text"`
	TurnID    string `json:"turn_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Event     string `json:"event,omitempty"` // "" | delta | final
}

var (
	broadcastCh = make(chan Entry, 256)
	subsMu      sync.Mutex
	subs        []chan<- Entry
	startFanout sync.Once
)

func startFanoutOnce() {
	startFanout.Do(func() {
		go func() {
			for e := range broadcastCh {
				subsMu.Lock()
				cp := append([]chan<- Entry(nil), subs...)
				subsMu.Unlock()
				for _, ch := range cp {
					select {
					case ch <- e:
					default:
					}
				}
			}
		}()
	})
}

// Publish 将一条输出送入广播队列（非阻塞：队列满则丢弃）。
func Publish(source string, content interface{}) {
	publishEntry(Entry{Source: source, Text: fmt.Sprint(content)})
}

// PublishForTurn 带回合 ID 的广播（供 interaction 回执路由与 SSE 过滤）。
func PublishForTurn(source, turnID string, content interface{}) {
	publishEntry(Entry{Source: source, Text: fmt.Sprint(content), TurnID: turnID})
}

// Subscribe 注册一个接收广播的缓冲 channel；cancel 会从订阅列表移除并关闭 channel。
func Subscribe(buf int) (<-chan Entry, func()) {
	startFanoutOnce()
	ch := make(chan Entry, buf)
	var rch chan<- Entry = ch
	var once sync.Once
	subsMu.Lock()
	subs = append(subs, rch)
	subsMu.Unlock()
	cancel := func() {
		once.Do(func() {
			subsMu.Lock()
			defer subsMu.Unlock()
			for i, c := range subs {
				if c == rch {
					subs = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			close(ch)
		})
	}
	return ch, cancel
}
