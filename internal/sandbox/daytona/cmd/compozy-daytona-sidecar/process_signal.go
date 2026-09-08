package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"syscall"

	"github.com/compozy/compozy/internal/procutil"
)

func serveProcessSignal(w http.ResponseWriter, r *http.Request, store *processStore, id string) {
	process, found := store.Get(id)
	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	var request struct {
		Signal string `json:"signal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid process signal request", http.StatusBadRequest)
		return
	}
	if request.Signal != "close-input" && request.Signal != "terminate" && request.Signal != "kill" {
		http.Error(w, "unknown process signal", http.StatusBadRequest)
		return
	}
	select {
	case <-process.done:
		if !process.exitVerified {
			http.Error(w, "completed process group exit remains unverified", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	default:
	}
	var err error
	switch request.Signal {
	case "close-input":
		err = process.CloseStdin()
	case "terminate":
		err = procutil.SignalCommandProcessGroup(process.cmd, syscall.SIGTERM)
	case "kill":
		err = procutil.SignalCommandProcessGroup(process.cmd, syscall.SIGKILL)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("signal process: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
