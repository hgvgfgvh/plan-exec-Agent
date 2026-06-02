package runview

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Start 旁路启动：监听回合日志目录，异步生成运行视图 HTML（不改 Plan/portal 主链）。
func Start(ctx context.Context) {
	settings := loadSettings()
	if !settings.Enabled {
		fmtDisabled()
		return
	}
	if err := ensureOutputDir(settings.OutputDir); err != nil {
		log.Printf("[runview] output dir: %v", err)
		return
	}
	if err := ensureOutputDir(settings.TurnLogDir); err != nil {
		log.Printf("[runview] turn log dir: %v", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[runview] fsnotify: %v", err)
		return
	}
	if err := w.Add(settings.TurnLogDir); err != nil {
		log.Printf("[runview] watch %s: %v", settings.TurnLogDir, err)
		_ = w.Close()
		return
	}

	if !settings.llmConfigured() {
		log.Printf("[runview] 已启用但未配置 llm_api_base/key/model，生成将失败并回退模板 HTML")
	}
	fmt.Printf("[runview] 已监听回合日志目录: %s → 输出: %s (llm=%s @ %s)\n",
		settings.TurnLogDir, settings.OutputDir, settings.LLMModel, settings.LLMAPIBase)

	var (
		mu      sync.Mutex
		pending = make(map[string]*time.Timer)
		done    = make(map[string]bool)
	)
	schedule := func(turnID, fullPath string) {
		if err := validateTurnID(turnID); err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if done[turnID] {
			return
		}
		if t, ok := pending[turnID]; ok {
			t.Stop()
		}
		debounce := time.Duration(settings.DebounceMs) * time.Millisecond
		pending[turnID] = time.AfterFunc(debounce, func() {
			mu.Lock()
			delete(pending, turnID)
			if done[turnID] {
				mu.Unlock()
				return
			}
			done[turnID] = true
			mu.Unlock()
			s := loadSettings()
			if !s.Enabled {
				return
			}
			Generate(ctx, s, fullPath)
		})
	}

	go func() {
		defer w.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if filepath.Ext(ev.Name) != ".json" {
					continue
				}
				turnID, ok := turnIDFromLogFilename(filepath.Base(ev.Name))
				if !ok {
					continue
				}
				schedule(turnID, ev.Name)
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Printf("[runview] watcher: %v", err)
			}
		}
	}()
}

func fmtDisabled() {
	fmt.Printf("[runview] 未启用（config run_view.enabled=false）\n")
}
