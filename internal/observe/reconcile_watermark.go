package observe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const reconcileWatermarkFile = "session-reconcile-watermarks.json"

// reconcileWatermark is the durable proof that a session's catalog row already mirrors its
// metadata: the session files it was reconciled from and the catalog row that reconcile left.
type reconcileWatermark struct {
	Files store.SessionFilesStamp `json:"files"`
	Row   string                  `json:"row"`
}

// reconcileWatermarks lets Reconcile leave a session unread while its files and its catalog row
// are both identical to what the last reconcile by this same executable saw. Any difference, a
// missing proof, or a missing catalog row sends the session down the full path. A nil receiver
// disables the optimization.
type reconcileWatermarks struct {
	path        string
	profile     string
	sessionsDir string
	rows        map[string]string
	// wrote holds the digest of each catalog row this reconcile wrote, taken from the row the
	// store read back inside its write transaction.
	wrote    map[string]string
	previous map[string]reconcileWatermark
	next     map[string]reconcileWatermark
	read     map[string]store.SessionFilesStamp
}

func (o *Observer) loadReconcileWatermarks(ctx context.Context) *reconcileWatermarks {
	homeDir := strings.TrimSpace(o.homePaths.HomeDir)
	sessionsDir := strings.TrimSpace(o.homePaths.SessionsDir)
	if homeDir == "" || sessionsDir == "" {
		return nil
	}
	if _, ok := o.registry.(store.SessionChangeReconciler); !ok {
		return nil
	}
	binary, err := store.BootWatermarkBinary()
	if err != nil {
		o.logger.Warn("observe: session reconcile watermarks disabled", "error", err)
		return nil
	}
	rows, err := o.sessionRowDigests(ctx)
	if err != nil {
		o.logger.Warn("observe: session reconcile watermarks disabled", "error", err)
		return nil
	}
	path := filepath.Join(homeDir, reconcileWatermarkFile)
	return &reconcileWatermarks{
		path:        path,
		profile:     binary,
		sessionsDir: sessionsDir,
		rows:        rows,
		wrote:       make(map[string]string),
		previous:    store.ReadBootWatermarks[reconcileWatermark](path, binary),
		next:        make(map[string]reconcileWatermark),
		read:        make(map[string]store.SessionFilesStamp),
	}
}

// sessionRowDigests fingerprints every catalog row, so a row changed by anything other than the
// reconcile that recorded it is noticed without comparing columns one by one.
func (o *Observer) sessionRowDigests(ctx context.Context) (map[string]string, error) {
	rows, err := o.registry.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveInclude,
	})
	if err != nil {
		return nil, err
	}
	return sessionRowDigestsOf(rows), nil
}

// sessionRowDigestsOf fingerprints catalog rows by id. A row that cannot be encoded gets no
// digest, which keeps its session out of every proof.
func sessionRowDigestsOf(rows []store.SessionInfo) map[string]string {
	digests := make(map[string]string, len(rows))
	for index := range rows {
		payload, err := json.Marshal(&rows[index])
		if err != nil {
			continue
		}
		sum := sha256.Sum256(payload)
		digests[rows[index].ID] = hex.EncodeToString(sum[:])
	}
	return digests
}

// reconciled remembers the rows the reconcile wrote, as its write transaction left them.
func (w *reconcileWatermarks) reconciled(written []store.SessionInfo) {
	if w == nil {
		return
	}
	w.wrote = sessionRowDigestsOf(written)
}

func (w *reconcileWatermarks) stamp(sessionID string) (store.SessionFilesStamp, bool) {
	stamp, found, err := store.StatSessionFiles(filepath.Join(w.sessionsDir, sessionID))
	if err != nil || !found {
		return store.SessionFilesStamp{}, false
	}
	return stamp, true
}

// unchanged reports whether the session directory can stay unread. A session that has to be read
// is stamped first, so a file replaced while it is being reconciled is never recorded.
func (w *reconcileWatermarks) unchanged(sessionID string) bool {
	if w == nil {
		return false
	}
	stamp, found := w.stamp(sessionID)
	if !found {
		return false
	}
	proof, ok := w.previous[sessionID]
	row := w.rows[sessionID]
	if ok && row != "" && proof.Row == row && proof.Files.Equal(stamp) {
		w.next[sessionID] = proof
		return true
	}
	w.read[sessionID] = stamp
	return false
}

// record stamps the sessions this reconcile read, left untouched on disk, and mirrored into the
// catalog. A proof pairs the files with the row the reconcile itself wrote: when the catalog row
// read now differs from that row, another writer changed it after the reconcile, and the session
// stays unrecorded so the next reconcile reads it again. Files modified too recently for their
// timestamps to be trusted stay unrecorded too.
func (w *reconcileWatermarks) record(ctx context.Context, o *Observer, sessionIDs []string) {
	if w == nil {
		return
	}
	if len(sessionIDs) > 0 {
		rows, err := o.sessionRowDigests(ctx)
		if err != nil {
			o.logger.Warn("observe: record session reconcile watermarks failed", "error", err)
			rows = nil
		}
		now := time.Now()
		for _, id := range sessionIDs {
			before, read := w.read[id]
			after, found := w.stamp(id)
			if !read || !found || !before.Equal(after) || !after.Settled(now) {
				continue
			}
			row := w.wrote[id]
			if row == "" || rows[id] != row {
				continue
			}
			w.next[id] = reconcileWatermark{Files: after, Row: row}
		}
	}
	if err := store.WriteBootWatermarks(w.path, w.profile, w.next); err != nil {
		o.logger.Warn("observe: persist session reconcile watermarks failed", "error", err)
	}
}
