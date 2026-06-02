package interaction

import (
	"AgentTest/outputbus"
	"AgentTest/userupload"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Deliver 订阅 outputbus，按回合绑定回执到设备（LLM 无感）。
type Deliver struct {
	bindings *bindingStore
	adapters []DeliverAdapter
	cancel   func()
	once     sync.Once
}

func newDeliver(bindings *bindingStore, adapters []DeliverAdapter) *Deliver {
	if bindings == nil {
		bindings = newBindingStore()
	}
	if len(adapters) == 0 {
		adapters = defaultAdapters()
	}
	return &Deliver{bindings: bindings, adapters: adapters}
}

// Start 注册 outputbus 订阅（进程内一次）。
func (d *Deliver) Start() {
	if d == nil {
		return
	}
	d.once.Do(func() {
		ch, cancel := outputbus.Subscribe(256)
		d.cancel = cancel
		go d.loop(ch)
		log.Printf("[interaction] deliver 已订阅 outputbus")
	})
}

func (d *Deliver) loop(ch <-chan outputbus.Entry) {
	for e := range ch {
		d.onEntry(e)
	}
}

func (d *Deliver) onEntry(e outputbus.Entry) {
	turnID := strings.TrimSpace(e.TurnID)
	if turnID == "" {
		return
	}
	b, ok := d.bindings.Get(turnID)
	if !ok {
		return
	}
	payload := DeliveryPayload{
		TurnID: turnID,
		Event:  e.Event,
		Source: e.Source,
		Text:   e.Text,
	}
	if err := PushToEndpoint(d.adapters, b.Reply, payload); err != nil {
		log.Printf("[interaction/deliver] push turn=%s: %v", turnID, err)
	}
}

// AfterTurn 回合结束后推送 inbox 产物路径（可选回执）。
func (d *Deliver) AfterTurn(turnID string) {
	if d == nil {
		return
	}
	b, ok := d.bindings.Get(turnID)
	if !ok {
		return
	}
	arts := listTurnArtifactRelPaths(turnID)
	if len(arts) == 0 {
		return
	}
	payload := DeliveryPayload{
		TurnID:    turnID,
		Event:     "artifacts",
		Source:    "interaction",
		Text:      fmt.Sprintf("本回合附件 %d 个", len(arts)),
		Artifacts: arts,
	}
	if err := PushToEndpoint(d.adapters, b.Reply, payload); err != nil {
		log.Printf("[interaction/deliver] artifacts turn=%s: %v", turnID, err)
	}
}

func listTurnArtifactRelPaths(turnID string) []string {
	abs, err := userupload.TurnInboxAbs(turnID)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil
	}
	prefix := userupload.WorkspaceRelPrefix()
	var out []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(prefix, userupload.InboxSubdir, turnID, ent.Name()))
		out = append(out, rel)
	}
	return out
}

// Stop 取消 outputbus 订阅。
func (d *Deliver) Stop() {
	if d != nil && d.cancel != nil {
		d.cancel()
	}
}
