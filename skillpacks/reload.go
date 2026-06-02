package skillpacks

import (
	"AgentTest/config"
	"fmt"
	"sort"
	"strings"
)

// ReloadReport 描述一次热更新相对上一快照的差异（仅 SKILL 目录与 L1/L2，不合并 mcp.yaml）。
type ReloadReport struct {
	Total   int
	Added   []string
	Removed []string
	Updated []string
}

// String 便于日志输出。
func (r ReloadReport) String() string {
	return fmt.Sprintf("total=%d added=%v removed=%v updated=%v",
		r.Total, r.Added, r.Removed, r.Updated)
}

// Reload 重新扫描 skill_packs roots 并原子替换内存快照；失败时保留旧快照。
// 不调用 mergePackMCPServers：pack 内 mcp.yaml 变更须重启主进程后生效。
func Reload(cfg *config.App) (ReloadReport, error) {
	var rep ReloadReport
	if cfg == nil || !cfg.Capabilities.SkillPacks.Enabled {
		return rep, nil
	}

	newPacks, _ := ScanRoots(cfg)
	oldPacks := snapshotPacks()

	oldMap := make(map[string]Pack, len(oldPacks))
	for _, p := range oldPacks {
		oldMap[p.ID] = p
	}
	newMap := make(map[string]Pack, len(newPacks))
	for _, p := range newPacks {
		newMap[p.ID] = p
	}

	for id, p := range newMap {
		if o, ok := oldMap[id]; !ok {
			rep.Added = append(rep.Added, id)
		} else if packContentChanged(o, p) {
			rep.Updated = append(rep.Updated, id)
		}
	}
	for id := range oldMap {
		if _, ok := newMap[id]; !ok {
			rep.Removed = append(rep.Removed, id)
		}
	}
	sort.Strings(rep.Added)
	sort.Strings(rep.Removed)
	sort.Strings(rep.Updated)
	rep.Total = len(newPacks)

	mu.Lock()
	loaded = append([]Pack(nil), newPacks...)
	mu.Unlock()

	return rep, nil
}

func packContentChanged(a, b Pack) bool {
	if a.Title != b.Title || a.Description != b.Description || a.BodySummary != b.BodySummary {
		return true
	}
	return strings.TrimSpace(a.FullMarkdown) != strings.TrimSpace(b.FullMarkdown)
}
