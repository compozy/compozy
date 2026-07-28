package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"log"
	"net/http"

	"strings"
)

func splitSessionPath(raw string) (string, string, bool) {
	const prefix = "/v1/sessions/"
	if !strings.HasPrefix(raw, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(raw, prefix)
	if remainder == "" {
		return "", "", false
	}
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], "/" + parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func randomID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes[:])
}
