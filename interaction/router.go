package interaction

import (
	"AgentTest/agent/runcontrol"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Router 统一入站标注、上下文注入与回执绑定。
type Router struct {
	registry *Registry
	bindings *bindingStore
	deliver  *Deliver
}

var (
	defaultRouter   *Router
	defaultRouterMu sync.Once
)

// Default 返回进程内默认 Router（含 Registry / Deliver）。
func Default() *Router {
	defaultRouterMu.Do(func() {
		reg := DefaultRegistry
		bindings := newBindingStore()
		deliver := newDeliver(bindings, defaultAdapters())
		defaultRouter = &Router{
			registry: reg,
			bindings: bindings,
			deliver:  deliver,
		}
		deliver.Start()
		go func() {
			t := time.NewTicker(10 * time.Minute)
			defer t.Stop()
			for range t.C {
				bindings.gc()
			}
		}()
	})
	return defaultRouter
}

// HandleTurn 设备消息进入 Agent 主链前的统一入口。
func (r *Router) HandleTurn(parent context.Context, req TurnRequest) error {
	if r == nil {
		return fmt.Errorf("interaction router 未初始化")
	}
	req.Normalize()
	src := req.SourceEndpoint()
	r.registry.Touch(src)

	turnCtx := runcontrol.BeginTurn(parent, req.Message)
	turnID := runcontrol.CurrentTurnID()
	if turnID == "" {
		return fmt.Errorf("interaction: 无 turn_id")
	}

	reply := req.ReplyTo
	if reply.Channel == "" {
		reply = src
	}
	binding := ReplyBinding{
		TurnID: turnID,
		Source: src,
		Reply:  reply,
	}
	r.bindings.Put(binding)
	turnCtx = runcontrol.WithInteractionMeta(turnCtx, runcontrol.InteractionMeta{
		Channel:       src.Channel,
		DeviceID:      src.DeviceID,
		SessionID:     src.SessionID,
		ReplyChannel:  reply.Channel,
		ReplyDeviceID: reply.DeviceID,
	})

	peers := r.registry.SliceForTurn(src, 6)
	routingBlock := FormatRoutingBlock(src, peers)
	log.Printf("[interaction] turn=%s source=%s/%s reply=%s/%s",
		turnID, src.Channel, src.DeviceID, reply.Channel, reply.DeviceID)

	err := invokeProcessTurn(turnCtx, req.Message, req.StagingID, routingBlock)
	r.deliver.AfterTurn(turnID)
	return err
}

// Bindings 暴露绑定表（测试 / device MCP）。
func (r *Router) Bindings() *bindingStore {
	return r.bindings
}

// RegistryRef 暴露注册表。
func (r *Router) RegistryRef() *Registry {
	return r.registry
}
