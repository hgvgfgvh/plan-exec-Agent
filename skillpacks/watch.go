package skillpacks

import (
	"AgentTest/config"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// StartWatcher 监视 skill_packs roots 下文件变更，防抖后触发热更新（更新 AGENTS 外挂 SKILL 地图）。
// 在进程 ctx 取消时退出。cfg 须已 Load；通常与 Apply 之后、主循环启动前调用。
func StartWatcher(ctx context.Context, cfg *config.App) {
	if cfg == nil || !cfg.Capabilities.SkillPacks.Enabled {
		return
	}
	if !cfg.Capabilities.SkillPacks.Watch {
		return
	}
	roots := resolvedWatchRoots(cfg)
	if len(roots) == 0 {
		return
	}
	debounce := time.Duration(cfg.Capabilities.SkillPacks.WatchDebounceMs) * time.Millisecond
	if debounce < 200*time.Millisecond {
		debounce = 1500 * time.Millisecond
	}
	go runWatcher(ctx, cfg, roots, debounce)
}

func resolvedWatchRoots(cfg *config.App) []string {
	var out []string
	for _, rel := range cfg.Capabilities.SkillPacks.Roots {
		root := resolvePackRoot(cfg, rel)
		if root == "" {
			continue
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			fmt.Printf("[skill_packs] 监视：无法创建根目录 %q: %v\n", root, err)
			continue
		}
		out = append(out, root)
	}
	return out
}

func runWatcher(ctx context.Context, cfg *config.App, roots []string, debounce time.Duration) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("[skill_packs] 监视启动失败: %v\n", err)
		return
	}
	defer w.Close()

	for _, root := range roots {
		if err := addWatchTree(w, root); err != nil {
			fmt.Printf("[skill_packs] 监视注册路径失败 %q: %v\n", root, err)
		}
	}
	fmt.Printf("[skill_packs] 已启动文件监视（防抖 %s）: %v\n", debounce, roots)

	var (
		debounceMu    sync.Mutex
		debounceTimer *time.Timer
	)

	scheduleReload := func(reason string) {
		debounceMu.Lock()
		defer debounceMu.Unlock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(debounce, func() {
			rep, err := Reload(cfg)
			if err != nil {
				fmt.Printf("[skill_packs] 热更新失败 (%s): %v\n", reason, err)
				return
			}
			if len(rep.Added)+len(rep.Removed)+len(rep.Updated) == 0 && rep.Total > 0 {
				fmt.Printf("[skill_packs] 热更新完成 (%s): %d 包（无 id 级变化）\n", reason, rep.Total)
				return
			}
			fmt.Printf("[skill_packs] 热更新完成 (%s): %s\n", reason, rep.String())
		})
	}

	for {
		select {
		case <-ctx.Done():
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceMu.Unlock()
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Create != 0 {
				if fi, statErr := os.Stat(ev.Name); statErr == nil && fi.IsDir() {
					_ = addWatchTree(w, ev.Name)
				}
			}
			if watchEventTriggersReload(ev) {
				scheduleReload(ev.Op.String() + " " + filepath.Base(ev.Name))
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			fmt.Printf("[skill_packs] 监视错误: %v\n", err)
		}
	}
}

func addWatchTree(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if addErr := w.Add(path); addErr != nil {
			return addErr
		}
		return nil
	})
}

func watchEventTriggersReload(ev fsnotify.Event) bool {
	if ev.Op&fsnotify.Chmod == ev.Op && ev.Op&^(fsnotify.Chmod) == 0 {
		return false
	}
	return ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}
