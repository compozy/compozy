package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

const (
	defaultWriteBufferSize = 256
	defaultDrainTimeout    = 5 * time.Second
)

const (
	sessionStateOpen int32 = iota
	sessionStateClosing
	sessionStateClosed
)

type sessionWriteRequest struct {
	ctx     context.Context
	kind    sessionWriteKind
	event   store.SessionEvent
	events  []store.SessionEvent
	usage   store.TokenUsage
	hook    hookspkg.HookRunRecord
	archive store.EventArchiveRequest
	result  chan sessionWriteResult
}

type sessionWriteResult struct {
	event   store.SessionEvent
	events  []store.SessionEvent
	archive store.EventArchiveResult
	err     error
}

type sessionShutdownRequest struct {
	ctx    context.Context
	result chan error
}

// SessionDB owns a per-session SQLite database and its dedicated writer loop.
type SessionDB struct {
	db         *sql.DB
	path       string
	sessionID  string
	writeCh    chan sessionWriteRequest
	shutdownCh chan sessionShutdownRequest
	writerDone chan struct{}
	writerCtx  context.Context
	cancel     context.CancelFunc

	acceptMu sync.RWMutex
	state    atomic.Int32

	drainTimeout time.Duration
	now          func() time.Time
}

var _ store.EventRecorder = (*SessionDB)(nil)
var _ store.EventArchiver = (*SessionDB)(nil)

// OpenSessionDB opens or creates the per-session events database at path.
func OpenSessionDB(ctx context.Context, sessionID string, path string) (*SessionDB, error) {
	if ctx == nil {
		return nil, errors.New("store: open session database context is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("store: session database session id is required")
	}

	db, err := openSessionSQLite(ctx, path)
	if err != nil {
		return nil, err
	}

	sessionDB := &SessionDB{
		db:           db,
		path:         strings.TrimSpace(path),
		sessionID:    strings.TrimSpace(sessionID),
		writeCh:      make(chan sessionWriteRequest, defaultWriteBufferSize),
		shutdownCh:   make(chan sessionShutdownRequest, 1),
		writerDone:   make(chan struct{}),
		drainTimeout: defaultDrainTimeout,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	sessionDB.writerCtx, sessionDB.cancel = context.WithCancel(context.Background())
	sessionDB.state.Store(sessionStateOpen)

	go func() {
		defer close(sessionDB.writerDone)
		sessionDB.writerLoop()
	}()

	return sessionDB, nil
}

// Path reports the on-disk path for the database file.
func (s *SessionDB) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// SessionID reports the owning session identifier for the database.
func (s *SessionDB) SessionID() string {
	if s == nil {
		return ""
	}
	return s.sessionID
}
