package main

import (
	"encoding/json"
	"errors"

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
	usedIDs   map[string]struct{}
}

func newProcessStore() *processStore {
	return &processStore{processes: make(map[string]*managedProcess), usedIDs: make(map[string]struct{})}
}

func (s *processStore) Put(process *managedProcess) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processes[process.id] = process
	s.usedIDs[process.id] = struct{}{}
}

var errProcessIDUsed = errors.New("process identity already used")

func (s *processStore) LaunchIdentified(command, id string) (*managedProcess, error) {
	if len(id) < 16 || len(id) > 128 {
		return nil, errors.New("process identity must contain 16 to 128 URL-safe characters")
	}
	for _, char := range id {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '-' || char == '_'
		if !valid {
			return nil, errors.New("process identity contains invalid characters")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, used := s.usedIDs[id]; used {
		return nil, errProcessIDUsed
	}
	process, err := startManagedProcess(command, id)
	if err != nil {
		return nil, err
	}
	s.processes[id] = process
	s.usedIDs[id] = struct{}{}
	return process, nil
}

func (s *processStore) Get(id string) (*managedProcess, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	process, ok := s.processes[id]
	return process, ok
}

func (s *processStore) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.processes, id)
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
		serveLaunch(w, r, store, false)
	})
	mux.HandleFunc("POST /v1/launch/identified", func(w http.ResponseWriter, r *http.Request) {
		serveLaunch(w, r, store, true)
	})
	mux.HandleFunc("/v1/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessionID, suffix, ok := splitSessionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodPost && suffix == "/signal":
			serveProcessSignal(w, r, store, sessionID)
		case r.Method == http.MethodGet && suffix == "":
			serveProcessStatus(w, store, sessionID)
		case r.Method == http.MethodDelete && suffix == "":
			process, found := store.Get(sessionID)
			if !found {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			if err := process.Stop(); err != nil {
				http.Error(w, fmt.Sprintf("stop session: %v", err), http.StatusInternalServerError)
				return
			}
			store.Remove(sessionID)
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

func serveLaunch(w http.ResponseWriter, r *http.Request, store *processStore, identified bool) {
	var request launchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("decode launch request: %v", err), http.StatusBadRequest)
		return
	}
	var process *managedProcess
	var err error
	if identified {
		process, err = store.LaunchIdentified(request.Command, request.ID)
	} else {
		process, err = newManagedProcess(request.Command)
		if err == nil {
			store.Put(process)
		}
	}
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errProcessIDUsed) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("launch command: %v", err), status)
		return
	}
	writeJSON(w, http.StatusCreated, launchResponse{ID: process.id})
}

type processStatusResponse struct {
	ID           string `json:"id"`
	Exited       bool   `json:"exited"`
	ExitVerified bool   `json:"exitVerified"`
	ExitCode     *int   `json:"exitCode,omitempty"`
}

func serveProcessStatus(w http.ResponseWriter, store *processStore, sessionID string) {
	process, found := store.Get(sessionID)
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	status := processStatusResponse{ID: process.id}
	select {
	case <-process.done:
		// Closing done publishes the command result and process-group verification together.
		status.Exited = true
		status.ExitVerified = process.exitVerified
		status.ExitCode = &process.exitCode
	default:
	}
	writeJSON(w, http.StatusOK, status)
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
