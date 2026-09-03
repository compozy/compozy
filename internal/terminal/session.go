package terminal

import (
	"context"
	"errors"
	"sync"
	"time"

	terminalvt "github.com/compozy/compozy/internal/terminal/vt"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

type session struct {
	manager     *Service
	ctx         context.Context
	cancel      context.CancelFunc
	proc        Proc
	filter      outputFilter
	ring        *Ring
	vt          *terminalvt.Actor
	lease       *leaseMachine
	audit       *auditGate
	flow        *terminalwire.Group
	nonce       string
	profileName string
	titlePinned bool

	mu                  sync.RWMutex
	info                Info
	lastActivity        time.Time
	revision            uint64
	revisionReady       chan struct{}
	readerEnded         bool
	reaping             bool
	exit                *Exit
	closeReason         string
	closeActor          Actor
	closedEmitted       bool
	vtCarry             []byte
	subscribers         map[uint64]*subscription
	nextSubID           uint64
	cols                uint16
	rows                uint16
	processRecord       processCheckpoint
	policy              Settings
	recordingMu         sync.Mutex
	recordingWG         sync.WaitGroup
	recordingSealed     bool
	recording           *activeRecording
	failedRecording     *activeRecording
	authorityMu         sync.Mutex
	finalizationMu      sync.Mutex
	journalClosePending bool
	streamMu            sync.Mutex
	captureMu           sync.Mutex
	capture             []byte
	captureBytes        int64
	captureTruncated    bool
	captureOutput       bool
	beforeExitPublish   func(context.Context, Info) error
	done                chan struct{}
	closeOnce           sync.Once
}

func newSession(
	parent context.Context,
	manager *Service,
	proc Proc,
	info Info,
	settings Settings,
	nonce string,
	profileName string,
	cols uint16,
	rows uint16,
	titlePinned bool,
) *session {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	item := &session{
		manager:       manager,
		ctx:           ctx,
		cancel:        cancel,
		proc:          proc,
		flow:          terminalwire.NewGroup(),
		ring:          NewRing(settings.ScrollbackBytes),
		audit:         &auditGate{},
		nonce:         nonce,
		profileName:   profileName,
		titlePinned:   titlePinned,
		info:          info,
		lastActivity:  manager.now(),
		policy:        settings,
		revisionReady: make(chan struct{}),
		subscribers:   make(map[uint64]*subscription),
		done:          make(chan struct{}),
		cols:          cols,
		rows:          rows,
	}
	item.filter = newOSCSecurityFilter(nonce, item.programTitleChanged)
	item.vt = terminalvt.New(int(cols), int(rows), func() ([]byte, uint64) {
		return item.ring.Snapshot()
	})
	item.lease = newLeaseMachine(infoController(info), proc, defaultControllerGrace, item.leaseChanged)
	return item
}

func (s *session) settings(ctx context.Context) Settings {
	s.mu.RLock()
	workspaceID := s.info.WS
	profileID := s.info.ProfileID
	current := s.policy
	s.mu.RUnlock()
	settings, err := s.manager.settings(ctx, workspaceID, profileID)
	validationErr := validateSettings(settings)
	if err != nil || validationErr != nil {
		s.manager.logger.Warn(
			"terminal: retain last valid settings",
			"terminal_id", s.info.ID,
			"error", errors.Join(err, validationErr),
		)
		return current
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = settings
	return s.policy
}

func infoController(info Info) Actor {
	if info.Controller == nil {
		return Actor{}
	}
	return *info.Controller
}

func (s *session) MarkerNonce() string { return s.nonce }

func (s *session) start() {
	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		s.readOutput()
	}()
	go s.waitProcess(outputDone)
}
