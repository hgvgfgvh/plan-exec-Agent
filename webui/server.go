package webui

import (
	"AgentTest/agent/runcontrol"
	"AgentTest/agentWorkSpace/portal"
	"AgentTest/config"
	"AgentTest/interaction"
	"AgentTest/outputbus"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	cookieName = "at_sess"
)

var (
	sessMu sync.Mutex
	// token -> 过期时间
	sessions = make(map[string]time.Time)
	chatMu   sync.Mutex

	authMu       sync.RWMutex
	authUsername string
	authPassword string
)

func sessionValid(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	sessMu.Lock()
	defer sessMu.Unlock()
	exp, ok := sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(sessions, c.Value)
		return false
	}
	return true
}

func newSessionToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func authOK(username, password string) bool {
	authMu.RLock()
	u := authUsername
	p := authPassword
	authMu.RUnlock()

	uu := []byte(strings.TrimSpace(username))
	pu := []byte(password)
	ue := []byte(u)
	pe := []byte(p)

	if len(uu) != len(ue) || len(pu) != len(pe) {
		return false
	}
	return subtle.ConstantTimeCompare(uu, ue) == 1 && subtle.ConstantTimeCompare(pu, pe) == 1
}

// Start 启动 Web UI（HTTP + SSE）；ctx 取消时关闭 http.Server。
func Start(ctx context.Context, cfg *config.App) {
	// 登录凭据（来自配置）；空值已由 config.applyDefaults 兜底。
	authMu.Lock()
	authUsername = strings.TrimSpace(cfg.Web.Username)
	authPassword = cfg.Web.Password
	authMu.Unlock()

	addr := strings.TrimSpace(cfg.Web.Listen)
	if addr == "" {
		addr = ":8765"
	}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", handleLogin)
	mux.HandleFunc("POST /api/logout", handleLogout)
	mux.HandleFunc("GET /api/events", handleEvents)
	// 聊天编排可能长于单次 HTTP：回合 context 必须挂在应用生命周期上，不能用 r.Context()
	//（请求结束会取消 r.Context()，导致黑板异步回调里 Submit 丢任务、前端收不到「反馈」）。
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, ctx)
	})
	mux.HandleFunc("POST /api/upload", handleUpload)
	mux.HandleFunc("GET /api/run-view/html", handleRunViewHTML)
	mux.HandleFunc("GET /api/run-view/file", handleRunViewFile)

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Printf("webui: embed static: %v", err)
		return
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// 初始化 interaction 路由（outputbus 回执订阅 + 设备表）。
	_ = interaction.Default()

	srv := &http.Server{
		Addr:              addr,
		Handler:           withCORSPanicLog(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		host := "127.0.0.1"
		if strings.HasPrefix(addr, ":") {
			fmt.Printf("\n[webui] 已启动 http://%s%s （登录 %s / 密码见 config/app.yaml 的 web.password）\n", host, addr, authUsername)
		} else {
			fmt.Printf("\n[webui] 已启动 http://%s （登录 %s）\n", addr, authUsername)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("webui ListenAndServe: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = srv.Shutdown(sh)
	}()
}

func withCORSPanicLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("webui panic: %v", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type loginReq struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Channel   string `json:"channel"`
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !authOK(req.Username, req.Password) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tok, err := newSessionToken()
	if err != nil {
		http.Error(w, "session", http.StatusInternalServerError)
		return
	}
	sessMu.Lock()
	sessions[tok] = time.Now().Add(7 * 24 * time.Hour)
	sessMu.Unlock()
	interaction.TouchPresence(interaction.Endpoint{
		Channel:   req.Channel,
		DeviceID:  req.DeviceID,
		SessionID: req.SessionID,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err == nil && c.Value != "" {
		sessMu.Lock()
		delete(sessions, c.Value)
		sessMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	if !sessionValid(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", http.StatusInternalServerError)
		return
	}

	ch, cancel := outputbus.Subscribe(128)
	defer cancel()

	presence := interaction.EndpointFromQuery(r.URL.Query())
	interaction.TouchPresence(presence)
	heartbeat := time.NewTicker(60 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			interaction.TouchPresence(presence)
		case e, ok := <-ch:
			if !ok {
				return
			}
			line, err := json.Marshal(e)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			fl.Flush()
		}
	}
}

type chatReq struct {
	Message   string `json:"message"`
	StagingID string `json:"staging_id"`
	Channel   string `json:"channel"`
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
	ReplyTo   *struct {
		Channel   string `json:"channel"`
		DeviceID  string `json:"device_id"`
		SessionID string `json:"session_id"`
	} `json:"reply_to"`
}

func handleChat(w http.ResponseWriter, r *http.Request, appCtx context.Context) {
	if !sessionValid(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	stagingID := strings.TrimSpace(req.StagingID)
	if msg == "" && stagingID == "" {
		http.Error(w, "empty", http.StatusBadRequest)
		return
	}
	if msg == "" {
		msg = "请根据我上传的附件进行分析。"
	}

	chatMu.Lock()
	defer chatMu.Unlock()
	portal.UnifiedOutputGateway("user", userDisplayMessage(msg, stagingID))
	turnReq := interaction.TurnRequest{
		Channel:   req.Channel,
		DeviceID:  req.DeviceID,
		SessionID: req.SessionID,
		Message:   msg,
		StagingID: stagingID,
	}
	if req.ReplyTo != nil {
		turnReq.ReplyTo = interaction.Endpoint{
			Channel:   req.ReplyTo.Channel,
			DeviceID:  req.ReplyTo.DeviceID,
			SessionID: req.ReplyTo.SessionID,
		}
	}
	if err := interaction.Default().HandleTurn(appCtx, turnReq); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"turn_id": runcontrol.CurrentTurnID(),
	})
}
