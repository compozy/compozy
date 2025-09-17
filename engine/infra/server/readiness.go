package server

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type reconciliationStatus struct {
	mu           sync.RWMutex
	completed    bool
	lastAttempt  time.Time
	lastError    error
	attemptCount int
}

func (rs *reconciliationStatus) isReady() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.completed
}

func (rs *reconciliationStatus) getStatus() (bool, time.Time, int, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.completed, rs.lastAttempt, rs.attemptCount, rs.lastError
}

func (rs *reconciliationStatus) setCompleted() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.completed = true
	rs.lastAttempt = time.Now()
	rs.lastError = nil
}

func (rs *reconciliationStatus) setError(err error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.lastAttempt = time.Now()
	rs.lastError = err
	rs.attemptCount++
}

func (s *Server) initReadinessMetrics() {
	if s.monitoring == nil || !s.monitoring.IsInitialized() {
		return
	}
	log := logger.FromContext(s.ctx)
	meter := s.monitoring.Meter()
	g, err := meter.Int64ObservableGauge(
		"compozy_server_ready",
		metric.WithDescription("Server readiness: 1 ready, 0 not_ready"),
	)
	if err == nil {
		s.readyGauge = g
		reg, regErr := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			val := int64(0)
			if s.isFullyReady() {
				val = 1
			}
			o.ObserveInt64(
				s.readyGauge,
				val,
				metric.WithAttributes(
					attribute.String("store_driver", s.storeDriverLabel),
					attribute.String("cache_driver", s.cacheDriverLabel),
					attribute.String("auth_repo_driver", s.authRepoDriverLabel),
					attribute.String("auth_cache_driver", s.authCacheDriverLabel),
				),
			)
			return nil
		}, s.readyGauge)
		if regErr != nil {
			log.Error("Failed to register readiness gauge callback", "error", regErr)
		} else {
			s.readyCallback = reg
		}
	} else {
		log.Error("Failed to create readiness gauge", "error", err)
	}
	c, err := meter.Int64Counter(
		"compozy_server_ready_transitions_total",
		metric.WithDescription("Count of readiness state transitions"),
	)
	if err == nil {
		s.readyTransitionsTotal = c
	} else {
		log.Error("Failed to create readiness transition counter", "error", err)
	}
}

func (s *Server) onReadinessMaybeChanged(reason string) {
	s.readinessMu.Lock()
	now := s.isFullyReadyLocked()
	changed := now != s.lastReady
	s.lastReady = now
	s.readinessMu.Unlock()
	if changed && s.monitoring != nil && s.monitoring.IsInitialized() && s.readyTransitionsTotal != nil {
		to := statusNotReady
		if now {
			to = statusReady
		}
		s.readyTransitionsTotal.Add(
			s.ctx,
			1,
			metric.WithAttributes(
				attribute.String("component", "server"),
				attribute.String("to", to),
				attribute.String("reason", reason),
				attribute.String("store_driver", s.storeDriverLabel),
				attribute.String("cache_driver", s.cacheDriverLabel),
				attribute.String("auth_repo_driver", s.authRepoDriverLabel),
				attribute.String("auth_cache_driver", s.authCacheDriverLabel),
			),
		)
	}
}

func (s *Server) isFullyReady() bool {
	s.readinessMu.RLock()
	ready := s.isFullyReadyLocked()
	s.readinessMu.RUnlock()
	return ready
}

func (s *Server) isFullyReadyLocked() bool {
	return s.temporalReady && s.workerReady && s.mcpReady && s.IsReconciliationReady()
}

func (s *Server) GetReconciliationStatus() (bool, time.Time, int, error) {
	return s.reconciliationState.getStatus()
}

func (s *Server) IsReconciliationReady() bool {
	return s.reconciliationState.isReady()
}

func (s *Server) setTemporalReady(v bool) {
	s.readinessMu.Lock()
	s.temporalReady = v
	s.readinessMu.Unlock()
}

func (s *Server) setWorkerReady(v bool) {
	s.readinessMu.Lock()
	s.workerReady = v
	s.readinessMu.Unlock()
}

func (s *Server) isTemporalReady() bool {
	s.readinessMu.RLock()
	defer s.readinessMu.RUnlock()
	return s.temporalReady
}

func (s *Server) isWorkerReady() bool {
	s.readinessMu.RLock()
	defer s.readinessMu.RUnlock()
	return s.workerReady
}

func (s *Server) setMCPReady(v bool) {
	s.readinessMu.Lock()
	s.mcpReady = v
	s.readinessMu.Unlock()
}

func (s *Server) isMCPReady() bool {
	s.readinessMu.RLock()
	ready := s.mcpReady
	s.readinessMu.RUnlock()
	return ready
}
