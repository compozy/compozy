package orchestrator

import "time"

const (
	defaultMaxConcurrentTools      = 10
	defaultRetryAttempts           = 3
	defaultRetryBackoffBase        = 100 * time.Millisecond
	defaultRetryBackoffMax         = 10 * time.Second
	defaultStructuredOutputRetries = 2
	defaultMaxToolIterations       = 10
	defaultMaxSequentialToolErrors = 8
	defaultMaxConsecutiveSuccesses = 3
	defaultNoProgressThreshold     = 3
)

type settings struct {
	timeout                 time.Duration
	maxConcurrentTools      int
	maxToolIterations       int
	maxSequentialToolErrors int
	structuredOutputRetries int
	retryAttempts           int
	retryBackoffBase        time.Duration
	retryBackoffMax         time.Duration
	retryJitter             bool
	maxConsecutiveSuccesses int
	enableProgressTracking  bool
	noProgressThreshold     int
}

func buildSettings(cfg *Config) settings {
	if cfg == nil {
		cfg = &Config{}
	}
	s := settings{
		timeout:                 cfg.Timeout,
		maxConcurrentTools:      cfg.MaxConcurrentTools,
		maxToolIterations:       cfg.MaxToolIterations,
		maxSequentialToolErrors: cfg.MaxSequentialToolErrors,
		structuredOutputRetries: cfg.StructuredOutputRetryAttempts,
		retryAttempts:           cfg.RetryAttempts,
		retryBackoffBase:        cfg.RetryBackoffBase,
		retryBackoffMax:         cfg.RetryBackoffMax,
		retryJitter:             cfg.RetryJitter,
		maxConsecutiveSuccesses: cfg.MaxConsecutiveSuccesses,
		enableProgressTracking:  cfg.EnableProgressTracking,
		noProgressThreshold:     cfg.NoProgressThreshold,
	}

	if s.maxConcurrentTools <= 0 {
		s.maxConcurrentTools = defaultMaxConcurrentTools
	}
	if s.maxToolIterations <= 0 {
		s.maxToolIterations = defaultMaxToolIterations
	}
	if s.maxSequentialToolErrors <= 0 {
		s.maxSequentialToolErrors = defaultMaxSequentialToolErrors
	}
	if s.structuredOutputRetries <= 0 {
		s.structuredOutputRetries = defaultStructuredOutputRetries
	}
	if s.retryAttempts <= 0 {
		s.retryAttempts = defaultRetryAttempts
	}
	if s.retryBackoffBase <= 0 {
		s.retryBackoffBase = defaultRetryBackoffBase
	}
	if s.retryBackoffMax <= 0 {
		s.retryBackoffMax = defaultRetryBackoffMax
	}
	if s.maxConsecutiveSuccesses <= 0 {
		s.maxConsecutiveSuccesses = defaultMaxConsecutiveSuccesses
	}
	if s.noProgressThreshold <= 0 {
		s.noProgressThreshold = defaultNoProgressThreshold
	}
	return s
}
