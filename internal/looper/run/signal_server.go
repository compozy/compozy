package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

const (
	signalServerHost = "localhost"
)

// SignalEventTypeDone marks a job as completed.
const SignalEventTypeDone = "done"

// SignalEventTypeStatus carries an in-progress job status update.
const SignalEventTypeStatus = "status"

// SignalEvent is emitted by the signal server when an agent reports job state.
type SignalEvent struct {
	Type  string
	JobID string
	Data  map[string]string
}

// SignalServer receives job lifecycle signals from agents via localhost HTTP.
type SignalServer struct {
	app       *fiber.App
	eventCh   chan<- SignalEvent
	port      int
	listener  net.Listener
	knownJobs map[string]struct{}
	logger    *slog.Logger
	mu        sync.RWMutex
}

type signalJobRequest struct {
	ID string `json:"id"`
}

type signalStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// NewSignalServer constructs a signal server for the provided port and known job IDs.
func NewSignalServer(port int, eventCh chan<- SignalEvent, jobIDs []string) *SignalServer {
	server := &SignalServer{
		app:       fiber.New(),
		eventCh:   eventCh,
		port:      port,
		knownJobs: make(map[string]struct{}, len(jobIDs)),
		logger:    slog.Default(),
	}

	for _, jobID := range jobIDs {
		trimmed := strings.TrimSpace(jobID)
		if trimmed == "" {
			continue
		}
		server.knownJobs[trimmed] = struct{}{}
	}

	server.registerRoutes()
	return server
}

// Start begins serving requests and shuts down gracefully when ctx is canceled.
func (s *SignalServer) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.listener != nil {
		s.mu.Unlock()
		return errors.New("signal server already started")
	}
	port := s.port
	s.mu.Unlock()

	listenCtx := context.WithoutCancel(ctx)
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(listenCtx, "tcp", signalServerAddress(port))
	if err != nil {
		return fmt.Errorf("listen for signal server: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.port = signalServerPort(listener.Addr(), s.port)
	s.mu.Unlock()

	shutdownDone := make(chan struct{})
	defer close(shutdownDone)

	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gracefulShutdownTimeout)
			defer cancel()
			if err := s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, fiber.ErrNotRunning) {
				s.logger.Warn("signal server shutdown after context cancellation failed", "error", err)
			}
		case <-shutdownDone:
		}
	}()

	err = s.app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.mu.Lock()
		if s.listener == listener {
			s.listener = nil
		}
		s.mu.Unlock()
		return fmt.Errorf("serve signal server: %w", err)
	}
	return nil
}

// Shutdown stops the signal server gracefully.
func (s *SignalServer) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	err := s.app.ShutdownWithContext(ctx)
	if err != nil && !errors.Is(err, fiber.ErrNotRunning) {
		return fmt.Errorf("shutdown signal server: %w", err)
	}

	s.mu.Lock()
	s.listener = nil
	s.mu.Unlock()
	return nil
}

// Port returns the configured or bound port.
func (s *SignalServer) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

func (s *SignalServer) registerRoutes() {
	s.app.Post("/job/done", s.handleJobDone)
	s.app.Post("/job/status", s.handleJobStatus)
	s.app.Get("/health", s.handleHealth)
}

func (s *SignalServer) handleJobDone(ctx fiber.Ctx) error {
	payload, err := parseSignalJobRequest(ctx)
	if err != nil {
		return writeSignalError(ctx, fiber.StatusBadRequest, err.Error())
	}

	s.logger.Info("received job done signal", "job_id", payload.ID)

	if !s.isKnownJob(payload.ID) {
		s.logger.Warn("received signal for unknown job", "job_id", payload.ID)
		return writeSignalError(ctx, fiber.StatusNotFound, "unknown job id")
	}

	if !s.enqueueEvent(SignalEvent{Type: SignalEventTypeDone, JobID: payload.ID}) {
		return writeSignalError(ctx, fiber.StatusServiceUnavailable, "signal queue full")
	}

	return ctx.JSON(fiber.Map{"ok": true})
}

func (s *SignalServer) handleJobStatus(ctx fiber.Ctx) error {
	payload, err := parseSignalStatusRequest(ctx)
	if err != nil {
		return writeSignalError(ctx, fiber.StatusBadRequest, err.Error())
	}

	s.logger.Info("received job status signal", "job_id", payload.ID, "status", payload.Status)

	if !s.isKnownJob(payload.ID) {
		s.logger.Warn("received status for unknown job", "job_id", payload.ID)
		return writeSignalError(ctx, fiber.StatusNotFound, "unknown job id")
	}

	event := SignalEvent{
		Type:  SignalEventTypeStatus,
		JobID: payload.ID,
		Data: map[string]string{
			"status": payload.Status,
		},
	}
	if !s.enqueueEvent(event) {
		return writeSignalError(ctx, fiber.StatusServiceUnavailable, "signal queue full")
	}

	return ctx.JSON(fiber.Map{"ok": true})
}

func (s *SignalServer) handleHealth(ctx fiber.Ctx) error {
	return ctx.JSON(fiber.Map{"status": "ok"})
}

func (s *SignalServer) isKnownJob(jobID string) bool {
	_, ok := s.knownJobs[jobID]
	return ok
}

func (s *SignalServer) enqueueEvent(event SignalEvent) bool {
	select {
	case s.eventCh <- event:
		return true
	default:
		s.logger.Warn("signal queue full", "job_id", event.JobID, "type", event.Type)
		return false
	}
}

func parseSignalJobRequest(ctx fiber.Ctx) (*signalJobRequest, error) {
	body, err := signalRequestBody(ctx)
	if err != nil {
		return nil, err
	}

	var payload signalJobRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("malformed JSON")
	}

	payload.ID = strings.TrimSpace(payload.ID)
	if payload.ID == "" {
		return nil, errors.New("job id is required")
	}
	return &payload, nil
}

func parseSignalStatusRequest(ctx fiber.Ctx) (*signalStatusRequest, error) {
	body, err := signalRequestBody(ctx)
	if err != nil {
		return nil, err
	}

	var payload signalStatusRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("malformed JSON")
	}

	payload.ID = strings.TrimSpace(payload.ID)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.ID == "" {
		return nil, errors.New("job id is required")
	}
	if payload.Status == "" {
		return nil, errors.New("status is required")
	}
	return &payload, nil
}

func signalRequestBody(ctx fiber.Ctx) ([]byte, error) {
	body := ctx.Body()
	if len(body) == 0 {
		return nil, errors.New("request body is required")
	}

	return body, nil
}

func writeSignalError(ctx fiber.Ctx, status int, message string) error {
	return ctx.Status(status).JSON(fiber.Map{
		"error": message,
	})
}

func signalServerAddress(port int) string {
	return net.JoinHostPort(signalServerHost, strconv.Itoa(port))
}

func signalServerPort(addr net.Addr, fallback int) int {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.Port
	}
	return fallback
}
