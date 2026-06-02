package capabilities

import (
	"AgentTest/config"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

var auditMu sync.Mutex

// WriteAudit 若配置了 audit_log_path，则追加一行 JSON（与 stderr 日志互补）。
func WriteAudit(cfg *config.App, rec map[string]any) {
	if cfg == nil {
		return
	}
	path := strings.TrimSpace(cfg.Capabilities.Security.AuditLogPath)
	if path == "" {
		return
	}
	rec["ts"] = time.Now().UTC().Format(time.RFC3339Nano)

	auditMu.Lock()
	defer auditMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(rec)
}
