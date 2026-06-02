package webui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const handoffTTL = 2 * time.Minute

type handoffEntry struct {
	username string
	password string
	expires  time.Time
}

var (
	handoffMu sync.Mutex
	handoffs  = make(map[string]handoffEntry)
)

func newHandoffID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isLoopbackRemoteAddr(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func registerHandoffRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/session-handoff", handleSessionHandoffCreate)
	mux.HandleFunc("GET /auth/handoff", handleSessionHandoffConsume)
}

func handleSessionHandoffCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRemoteAddr(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
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
	id, err := newHandoffID()
	if err != nil {
		http.Error(w, "handoff", http.StatusInternalServerError)
		return
	}
	handoffMu.Lock()
	handoffs[id] = handoffEntry{
		username: strings.TrimSpace(req.Username),
		password: req.Password,
		expires:  time.Now().Add(handoffTTL),
	}
	handoffMu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "handoff": id})
}

func handleSessionHandoffConsume(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("h"))
	if id == "" {
		http.Error(w, "missing handoff", http.StatusBadRequest)
		return
	}
	handoffMu.Lock()
	ent, ok := handoffs[id]
	if ok {
		delete(handoffs, id)
	}
	handoffMu.Unlock()
	if !ok || time.Now().After(ent.expires) {
		http.Error(w, "handoff expired", http.StatusGone)
		return
	}
	if !authOK(ent.username, ent.password) {
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
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	log.Printf("[webui] 浏览器已通过 handoff 登录 (%s)", ent.username)
	http.Redirect(w, r, "/", http.StatusFound)
}
