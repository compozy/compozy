package main

import (
	"encoding/json"

	"log"
	"net/http"

	"sync"

	"github.com/gorilla/websocket"
)

func handleStream(
	w http.ResponseWriter,
	r *http.Request,
	process *managedProcess,
	upgrader *websocket.Upgrader,
) {
	if !process.claimStream() {
		http.Error(w, "session stream already attached", http.StatusConflict)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		process.releaseUnstartedStream()
		return
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("close sidecar websocket: %v", closeErr)
		}
	}()

	var writeMu sync.Mutex
	writeBinary := func(payload []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.BinaryMessage, payload)
	}
	go streamStdoutFrames(process, writeBinary)
	exitDone := make(chan struct{})
	go streamExitFrame(process, writeBinary, exitDone)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if shouldClose := handleClientFrame(process, payload, writeBinary); shouldClose {
			return
		}
		select {
		case <-exitDone:
			return
		default:
		}
	}
}

func streamStdoutFrames(process *managedProcess, writeBinary frameWriter) {
	for {
		chunk, ok := process.stdout.Pop()
		if !ok {
			return
		}
		if !writeServerFrame(writeBinary, serverStdoutFrame, chunk, "write sidecar stdout frame") {
			return
		}
	}
}

func streamExitFrame(process *managedProcess, writeBinary frameWriter, exitDone chan<- struct{}) {
	defer close(exitDone)
	<-process.done
	payload, err := json.Marshal(exitPayload{
		ExitCode: process.exitCode,
		Stderr:   process.stderrText(),
	})
	if err != nil {
		writeServerFrame(writeBinary, serverErrorFrame, []byte(err.Error()), "write sidecar error frame")
		return
	}
	writeServerFrame(writeBinary, serverExitFrame, payload, "write sidecar exit frame")
}

func handleClientFrame(process *managedProcess, payload []byte, writeBinary frameWriter) bool {
	if len(payload) == 0 {
		return false
	}
	var err error
	logMessage := ""
	switch payload[0] {
	case clientStdinFrame:
		err = process.WriteStdin(payload[1:])
		logMessage = "write sidecar stdin error frame"
	case clientCloseStdinFrame:
		err = process.CloseStdin()
		logMessage = "write sidecar close-stdin error frame"
	case clientStopFrame:
		err = process.Stop()
		logMessage = "write sidecar stop error frame"
	default:
		return false
	}
	if err == nil {
		return false
	}
	writeServerFrame(writeBinary, serverErrorFrame, []byte(err.Error()), logMessage)
	return true
}

func writeServerFrame(writeBinary frameWriter, frame byte, payload []byte, logMessage string) bool {
	framed := append([]byte{frame}, payload...)
	if err := writeBinary(framed); err != nil {
		log.Printf("%s: %v", logMessage, err)
		return false
	}
	return true
}
