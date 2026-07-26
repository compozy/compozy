package main

import (
	"encoding/json"

	"flag"
	"fmt"

	"log"
	"net/http"
	"net/url"

	"strings"
	"sync"

	"time"

	"github.com/gorilla/websocket"
)

func (p *managedProcess) claimStream() bool {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	if p.streamClaimed {
		return false
	}
	p.streamClaimed = true
	return true
}

func (p *managedProcess) releaseUnstartedStream() {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	p.streamClaimed = false
}

type processStore struct {
	mu        sync.Mutex
	processes map[string]*managedProcess
}

func newProcessStore() *processStore {
	return &processStore{processes: make(map[string]*managedProcess)}
}

func (s *processStore) Put(process *managedProcess) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processes[process.id] = process
}

func (s *processStore) Get(id string) (*managedProcess, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	process, ok := s.processes[id]
	return process, ok
}

func (s *processStore) Take(id string) (*managedProcess, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	process, ok := s.processes[id]
	if ok {
		delete(s.processes, id)
	}
	return process, ok
}

func sidecarListenAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func allowWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func newHandler(store *processStore, upgrader *websocket.Upgrader) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{OK: true, Version: version})
	})
	mux.HandleFunc("/v1/launch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request launchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("decode launch request: %v", err), http.StatusBadRequest)
			return
		}
		process, err := newManagedProcess(request.Command)
		if err != nil {
			http.Error(w, fmt.Sprintf("launch command: %v", err), http.StatusBadRequest)
			return
		}
		store.Put(process)
		writeJSON(w, http.StatusCreated, launchResponse{ID: process.id})
	})
	mux.HandleFunc("/v1/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessionID, suffix, ok := splitSessionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodDelete && suffix == "":
			process, found := store.Take(sessionID)
			if !found {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			if err := process.Stop(); err != nil {
				http.Error(w, fmt.Sprintf("stop session: %v", err), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && suffix == "/stream":
			process, found := store.Get(sessionID)
			if !found {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			handleStream(w, r, process, upgrader)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func main() {
	port := flag.Int("port", 0, "listen port")
	flag.Parse()
	if *port <= 0 {
		log.Fatal("port is required")
	}

	store := newProcessStore()
	upgrader := websocket.Upgrader{
		CheckOrigin: allowWebSocketOrigin,
	}

	server := &http.Server{
		Addr:              sidecarListenAddr(*port),
		Handler:           newHandler(store, &upgrader),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
