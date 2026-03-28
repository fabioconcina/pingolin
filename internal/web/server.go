package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

type Server struct {
	store   *store.Store
	targets []string
	version string
	listen  string
}

func NewServer(s *store.Store, targets []string, version, listen string) *Server {
	return &Server{
		store:   s,
		targets: targets,
		version: version,
		listen:  listen,
	}
}

func (srv *Server) Start() error {
	mux := http.NewServeMux()

	// Serve static files from embedded FS
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("preparing static files: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticSub))

	mux.HandleFunc("/events", srv.handleEvents)
	mux.HandleFunc("/api/data", srv.handleAPIData)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve index.html with version injected
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	log.Printf("Starting web dashboard at http://%s", srv.listen)
	return http.ListenAndServe(srv.listen, mux)
}

func (srv *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial data immediately
	srv.sendEvent(w, flusher)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			srv.sendEvent(w, flusher)
		case <-r.Context().Done():
			return
		}
	}
}

func (srv *Server) sendEvent(w http.ResponseWriter, flusher http.Flusher) {
	data := FetchDashboardData(srv.store, srv.targets)
	data.UpdatedAt = time.Now().UnixMilli()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

func (srv *Server) handleAPIData(w http.ResponseWriter, r *http.Request) {
	data := FetchDashboardData(srv.store, srv.targets)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
