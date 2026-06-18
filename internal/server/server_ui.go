package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/yansircc/llm-broker/internal/ui"
)

func (s *Server) mountUIRoutes(mux *http.ServeMux) {
	distFS, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		slog.Warn("ui dist not found, root UI disabled", "error", err)
		return
	}
	indexHTML, _ := fs.ReadFile(distFS, "index.html")
	fileServer := http.FileServer(http.FS(distFS))

	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		if isReservedUIPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		serveUI(distFS, indexHTML, fileServer, w, r)
	})
}

func serveUI(distFS fs.FS, indexHTML []byte, fileServer http.Handler, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(indexHTML)
		return
	}
	if strings.HasPrefix(path, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if _, err := fs.Stat(distFS, path); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(indexHTML)
		return
	}
	fileServer.ServeHTTP(w, r)
}

func isReservedUIPath(path string) bool {
	switch {
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return true
	case path == "/console/login" || strings.HasPrefix(path, "/console/login/"):
		return true
	case path == "/dashboard" || strings.HasPrefix(path, "/dashboard/"):
		return true
	case path == "/accounts" || strings.HasPrefix(path, "/accounts/"):
		return true
	case path == "/users" || strings.HasPrefix(path, "/users/"):
		return true
	case path == "/activity" || strings.HasPrefix(path, "/activity/"):
		return true
	case path == "/migrations" || strings.HasPrefix(path, "/migrations/"):
		return true
	case path == "/cells" || strings.HasPrefix(path, "/cells/"):
		return true
	case path == "/admin-billing" || strings.HasPrefix(path, "/admin-billing/"):
		return true
	case path == "/login" || strings.HasPrefix(path, "/login/"):
		return true
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return true
	case path == "/v1" || strings.HasPrefix(path, "/v1/"):
		return true
	case path == "/compat" || strings.HasPrefix(path, "/compat/"):
		return true
	case path == "/openai" || strings.HasPrefix(path, "/openai/"):
		return true
	case path == "/ui" || strings.HasPrefix(path, "/ui/"):
		return true
	case path == "/add-account" || strings.HasPrefix(path, "/add-account/"):
		return true
	case path == "/health":
		return true
	default:
		return false
	}
}
