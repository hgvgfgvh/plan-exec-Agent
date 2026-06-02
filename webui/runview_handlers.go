package webui

import (
	"AgentTest/runview"
	"net/http"
	"path/filepath"
	"strings"
)

func handleRunViewHTML(w http.ResponseWriter, r *http.Request) {
	if !sessionValid(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	turnID := strings.TrimSpace(r.URL.Query().Get("turn_id"))
	path, err := runview.HTMLFileForTurn(turnID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; form-action 'none'")
	http.ServeFile(w, r, path)
}

func handleRunViewFile(w http.ResponseWriter, r *http.Request) {
	if !sessionValid(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	turnID := strings.TrimSpace(r.URL.Query().Get("turn_id"))
	artifactID := strings.TrimSpace(r.URL.Query().Get("artifact_id"))
	path, err := runview.ResolveArtifactFile(turnID, artifactID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// 按扩展名粗设 Content-Type；未知则 octet-stream
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".md", ".txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeFile(w, r, path)
}
