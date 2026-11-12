package daemon

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

const bootSessionWatermarkFile = "boot-session-watermarks.json"

// bootSessionWatermark is the durable proof that one retained session needs no boot work while
// its files carry the same evidence (store.FileStamp: size, modification time, inode identity
// and change time, and the database header digest): Verified covers the events database upgrade and owner check,
// Settled covers the restart capture and history repair pass.
type bootSessionWatermark struct {
	Files    store.SessionFilesStamp `json:"files"`
	Verified bool                    `json:"verified"`
	Settled  bool                    `json:"settled"`
}

// bootSessionWatermarks carries proofs from the previous boot and collects this boot's. A proof
// is only ever written after the work it stands for succeeded, and only ever honored when the
// session's files, the running build, and its embedded schema head all still match; a file whose
// identity cannot be established, and everything else, takes the full path. A nil receiver disables the optimization.
type bootSessionWatermarks struct {
	path        string
	profile     string
	sessionsDir string

	mu       sync.Mutex
	previous map[string]bootSessionWatermark
	next     map[string]bootSessionWatermark
}

func loadBootSessionWatermarks(
	homePaths compozyconfig.HomePaths,
	logger *slog.Logger,
) *bootSessionWatermarks {
	homeDir := strings.TrimSpace(homePaths.HomeDir)
	sessionsDir := strings.TrimSpace(homePaths.SessionsDir)
	if homeDir == "" || sessionsDir == "" {
		return nil
	}
	head, err := store.MigrationStreamHead(sessiondb.MigrationStream())
	if err != nil {
		logger.Warn("daemon: boot session watermarks disabled", "error", err)
		return nil
	}
	binary, err := store.BootWatermarkBinary()
	if err != nil {
		logger.Warn("daemon: boot session watermarks disabled", "error", err)
		return nil
	}
	path := filepath.Join(homeDir, bootSessionWatermarkFile)
	profile := fmt.Sprintf("%s|%s", binary, head)
	return &bootSessionWatermarks{
		path:        path,
		profile:     profile,
		sessionsDir: sessionsDir,
		previous:    store.ReadBootWatermarks[bootSessionWatermark](path, profile),
		next:        make(map[string]bootSessionWatermark),
	}
}

func (w *bootSessionWatermarks) stamp(sessionID string) (store.SessionFilesStamp, bool) {
	stamp, found, err := store.StatSessionFiles(filepath.Join(w.sessionsDir, sessionID))
	if err != nil || !found {
		return store.SessionFilesStamp{}, false
	}
	return stamp, true
}

// verified reports whether the previous boot's proof still covers the session's files. A proof
// that still holds is carried into this boot's index.
func (w *bootSessionWatermarks) verified(sessionID string) bool {
	if w == nil {
		return false
	}
	stamp, found := w.stamp(sessionID)
	if !found {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	proof, ok := w.previous[sessionID]
	if !ok || !proof.Verified || !proof.Files.Equal(stamp) {
		return false
	}
	w.next[sessionID] = proof
	return true
}

// recordVerified stamps a session whose database this boot just verified or upgraded. Files
// modified too recently for their timestamps to be trusted stay unrecorded.
func (w *bootSessionWatermarks) recordVerified(sessionID string) {
	if w == nil {
		return
	}
	stamp, found := w.stamp(sessionID)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !found || !stamp.Settled(time.Now()) {
		delete(w.next, sessionID)
		return
	}
	w.next[sessionID] = bootSessionWatermark{Files: stamp, Verified: true}
}

// settled reports whether restart capture and history repair already proved to be a no-op for
// the session's current files.
func (w *bootSessionWatermarks) settled(sessionID string) bool {
	if w == nil {
		return false
	}
	stamp, found := w.stamp(sessionID)
	if !found {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	proof, ok := w.next[sessionID]
	return ok && proof.Settled && proof.Files.Equal(stamp)
}

// recordSettled marks a session whose restart capture and history repair pass completed without
// changing its files. A pass that wrote anything leaves the session for the next boot to verify.
func (w *bootSessionWatermarks) recordSettled(sessionID string) {
	if w == nil {
		return
	}
	stamp, found := w.stamp(sessionID)
	w.mu.Lock()
	defer w.mu.Unlock()
	proof, ok := w.next[sessionID]
	if !ok || !found || !proof.Files.Equal(stamp) {
		return
	}
	proof.Settled = true
	w.next[sessionID] = proof
}

// save persists this boot's proofs. A failed write only costs the next boot its shortcuts.
func (w *bootSessionWatermarks) save(logger *slog.Logger) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := store.WriteBootWatermarks(w.path, w.profile, w.next); err != nil {
		logger.Warn("daemon: persist boot session watermarks failed", "error", err)
	}
}
