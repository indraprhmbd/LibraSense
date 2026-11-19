package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"librasense/internal/config"
	"librasense/internal/store"
)

type Server struct {
	cfg   *config.Config
	store *store.Store
	web   fs.FS
	keys  *jwksCache
}

func New(cfg *config.Config, database *store.Store, web fs.FS) *Server {
	server := &Server{cfg: cfg, store: database, web: web}

	if cfg.Access != nil {
		server.keys = &jwksCache{teamURL: cfg.Access.TeamDomain}
	}

	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	api := http.NewServeMux()
	api.HandleFunc("GET /api/overview", s.handleOverview)
	api.HandleFunc("GET /api/history", s.handleHistory)
	api.HandleFunc("PUT /api/settings", s.handleSettings)
	api.HandleFunc("POST /api/commands", s.handleCommand)

	device := http.NewServeMux()
	device.HandleFunc("POST /device/readings", s.handleReadings)
	device.HandleFunc("POST /device/ack", s.handleAck)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ts": time.Now().UnixMilli()})
	})
	mux.Handle("/api/", s.accessGuard(api))
	mux.Handle("/device/", s.deviceGuard(device))
	mux.Handle("/", s.static())

	return mux
}

func (s *Server) static() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.web == nil {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(s.web, path); err != nil {
			path = "index.html"
		}

		content, err := fs.ReadFile(s.web, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		switch {
		case strings.HasSuffix(path, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case strings.HasSuffix(path, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(path, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(content)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 256*1024))
	return decoder.Decode(target)
}
