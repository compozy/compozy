package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

const (
	defaultPreStartCacheTTL      = 30 * time.Second
	defaultPreStartCacheCapacity = 128
)

// PreStarter owns a bounded cache of provider pre-start probe reports for one
// daemon composition. Entries use LRU eviction; ties are resolved by the
// explicit comparable key order so eviction stays deterministic.
type PreStarter struct {
	mu         sync.Mutex
	entries    map[preStartCacheKey]preStartCacheEntry
	now        func() time.Time
	ttl        time.Duration
	capacity   int
	access     uint64
	generation uint64
	cacheKey   preStartCacheHMACKey
	cacheReady bool
}

type preStartCacheKey struct {
	ProviderName      string
	WorkspaceID       string
	HomeIdentity      string
	SandboxID         string
	SandboxBackend    string
	SandboxProfile    string
	SandboxInstanceID string
	AuthMode          compozyconfig.ProviderAuthMode
	EnvPolicy         compozyconfig.ProviderEnvPolicy
	HomePolicy        compozyconfig.ProviderHomePolicy
	Harness           compozyconfig.ProviderHarness
	RuntimeProvider   string
	Fingerprint       preStartCacheFingerprint
}

type preStartCacheEntry struct {
	report    PreStartReport
	expiresAt time.Time
	usedAt    uint64
}

type preStartCacheCandidate struct {
	key   preStartCacheKey
	entry preStartCacheEntry
}

type preStarterOption func(*PreStarter)

// NewPreStarter constructs an isolated pre-start probe cache for one daemon.
func NewPreStarter() *PreStarter {
	return newPreStarter()
}

// newPreStarter applies internal cache policy overrides for package tests.
func newPreStarter(options ...preStarterOption) *PreStarter {
	starter := &PreStarter{
		entries:  make(map[preStartCacheKey]preStartCacheEntry),
		now:      time.Now,
		ttl:      defaultPreStartCacheTTL,
		capacity: defaultPreStartCacheCapacity,
	}
	starter.cacheKey, starter.cacheReady = newPreStartCacheHMACKey()
	for _, option := range options {
		if option != nil {
			option(starter)
		}
	}
	if starter.now == nil {
		starter.now = time.Now
	}
	if starter.ttl <= 0 {
		starter.ttl = defaultPreStartCacheTTL
	}
	if starter.capacity <= 0 {
		starter.capacity = defaultPreStartCacheCapacity
	}
	return starter
}

// PreStart classifies provider-auth readiness before a provider subprocess is
// spawned. It caches only final, explicitly scoped probe results.
func (s *PreStarter) PreStart(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) PreStartReport {
	normalized := env.Normalize()
	if s == nil || !preStartContextCacheable(ctx) {
		return runPreStart(ctx, provider, &normalized)
	}
	key, cacheable := s.newPreStartCacheKey(provider, &normalized)
	if !cacheable {
		return runPreStart(ctx, provider, &normalized)
	}

	now := s.now()
	s.mu.Lock()
	s.deleteExpiredLocked(now)
	if entry, ok := s.entries[key]; ok {
		entry.usedAt = s.nextAccessLocked()
		s.entries[key] = entry
		report, cloneable := clonePreStartReport(entry.report)
		s.mu.Unlock()
		if cloneable {
			return report
		}
		return runPreStart(ctx, provider, &normalized)
	}
	generation := s.generation
	s.mu.Unlock()

	report := runPreStart(ctx, provider, &normalized)
	if !preStartContextCacheable(ctx) || !preStartReportSafeForCache(report, provider, &normalized) {
		return report
	}
	cachedReport, cloneable := clonePreStartReport(report)
	if !cloneable {
		return report
	}

	now = s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteExpiredLocked(now)
	if generation != s.generation || !preStartContextCacheable(ctx) {
		return cachedReport
	}
	s.entries[key] = preStartCacheEntry{
		report:    cachedReport,
		expiresAt: now.Add(s.ttl),
		usedAt:    s.nextAccessLocked(),
	}
	s.evictOverflowLocked()
	returned, returnedCloneable := clonePreStartReport(cachedReport)
	if !returnedCloneable {
		return report
	}
	return returned
}

// Clear drops every cached report owned by this starter. The generation prevents
// a probe that began before Clear from repopulating the new configuration cache.
func (s *PreStarter) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.entries = make(map[preStartCacheKey]preStartCacheEntry)
	s.generation++
	s.mu.Unlock()
}

func withPreStartCacheTTL(ttl time.Duration) preStarterOption {
	return func(starter *PreStarter) {
		starter.ttl = ttl
	}
}

func withPreStartCacheCapacity(capacity int) preStarterOption {
	return func(starter *PreStarter) {
		starter.capacity = capacity
	}
}

func withPreStartClock(now func() time.Time) preStarterOption {
	return func(starter *PreStarter) {
		starter.now = now
	}
}

func (s *PreStarter) newPreStartCacheKey(
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) (preStartCacheKey, bool) {
	if s == nil || !s.cacheReady {
		return preStartCacheKey{}, false
	}
	scope := normalizePreStartScope(env.PreStartScope)
	providerName := strings.TrimSpace(env.ProviderName)
	if providerName == "" || !scope.cacheable() {
		return preStartCacheKey{}, false
	}
	fingerprint, fingerprinted := preStartSemanticFingerprint(s.cacheKey, provider, env)
	if !fingerprinted {
		return preStartCacheKey{}, false
	}
	return preStartCacheKey{
		ProviderName:      providerName,
		WorkspaceID:       scope.WorkspaceID,
		HomeIdentity:      scope.HomeIdentity,
		SandboxID:         scope.SandboxID,
		SandboxBackend:    scope.SandboxBackend,
		SandboxProfile:    scope.SandboxProfile,
		SandboxInstanceID: scope.SandboxInstanceID,
		AuthMode:          provider.EffectiveAuthMode(),
		EnvPolicy:         provider.EffectiveEnvPolicy(),
		HomePolicy:        provider.EffectiveHomePolicy(),
		Harness:           provider.EffectiveHarness(),
		RuntimeProvider:   strings.TrimSpace(provider.RuntimeProviderName(providerName)),
		Fingerprint:       fingerprint,
	}, true
}

func preStartContextCacheable(ctx context.Context) bool {
	return ctx != nil && ctx.Err() == nil && context.Cause(ctx) == nil
}

func normalizePreStartScope(scope PreStartScope) PreStartScope {
	return PreStartScope{
		WorkspaceID:       strings.TrimSpace(scope.WorkspaceID),
		HomeIdentity:      strings.TrimSpace(scope.HomeIdentity),
		SandboxID:         strings.TrimSpace(scope.SandboxID),
		SandboxBackend:    strings.TrimSpace(scope.SandboxBackend),
		SandboxProfile:    strings.TrimSpace(scope.SandboxProfile),
		SandboxInstanceID: strings.TrimSpace(scope.SandboxInstanceID),
	}
}

func (s PreStartScope) cacheable() bool {
	return s.WorkspaceID != "" &&
		s.HomeIdentity != "" &&
		s.SandboxID != "" &&
		s.SandboxBackend != "" &&
		s.SandboxProfile != ""
}

func (s *PreStarter) deleteExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if !now.Before(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}

func (s *PreStarter) nextAccessLocked() uint64 {
	if s.access == ^uint64(0) {
		s.rebaseAccessOrderLocked()
	}
	s.access++
	return s.access
}

func (s *PreStarter) rebaseAccessOrderLocked() {
	entries := make([]preStartCacheCandidate, 0, len(s.entries))
	for key, entry := range s.entries {
		entries = append(entries, preStartCacheCandidate{key: key, entry: entry})
	}
	for index := range entries {
		for next := index + 1; next < len(entries); next++ {
			if preStartEntryLess(entries[next], entries[index]) {
				entries[index], entries[next] = entries[next], entries[index]
			}
		}
	}
	for index, item := range entries {
		item.entry.usedAt = uint64(index + 1)
		s.entries[item.key] = item.entry
	}
	s.access = uint64(len(entries))
}

func (s *PreStarter) evictOverflowLocked() {
	for len(s.entries) > s.capacity {
		var victim preStartCacheCandidate
		found := false
		for key, entry := range s.entries {
			candidate := preStartCacheCandidate{key: key, entry: entry}
			if !found || preStartEntryLess(candidate, victim) {
				victim = candidate
				found = true
			}
		}
		delete(s.entries, victim.key)
	}
}

func preStartEntryLess(left preStartCacheCandidate, right preStartCacheCandidate) bool {
	if left.entry.usedAt != right.entry.usedAt {
		return left.entry.usedAt < right.entry.usedAt
	}
	return preStartCacheKeyLess(left.key, right.key)
}

func preStartCacheKeyLess(left preStartCacheKey, right preStartCacheKey) bool {
	for _, field := range [][2]string{
		{left.ProviderName, right.ProviderName},
		{left.WorkspaceID, right.WorkspaceID},
		{left.HomeIdentity, right.HomeIdentity},
		{left.SandboxID, right.SandboxID},
		{left.SandboxBackend, right.SandboxBackend},
		{left.SandboxProfile, right.SandboxProfile},
		{left.SandboxInstanceID, right.SandboxInstanceID},
		{string(left.AuthMode), string(right.AuthMode)},
		{string(left.EnvPolicy), string(right.EnvPolicy)},
		{string(left.HomePolicy), string(right.HomePolicy)},
		{string(left.Harness), string(right.Harness)},
		{left.RuntimeProvider, right.RuntimeProvider},
	} {
		if field[0] != field[1] {
			return field[0] < field[1]
		}
	}
	return bytes.Compare(left.Fingerprint[:], right.Fingerprint[:]) < 0
}

func clonePreStartReport(report PreStartReport) (PreStartReport, bool) {
	if report.Cause != nil {
		return PreStartReport{}, false
	}
	if report.Item == nil {
		return PreStartReport{}, true
	}
	item := *report.Item
	evidence, cloneable := clonePreStartEvidence(item.Evidence)
	if !cloneable {
		return PreStartReport{}, false
	}
	item.Evidence = evidence
	return PreStartReport{Item: &item}, true
}

func preStartReportSafeForCache(
	report PreStartReport,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) bool {
	if report.Cause != nil {
		return false
	}
	if report.Item == nil {
		return true
	}
	for _, value := range preStartSensitiveInputs(provider, env) {
		if value != "" && preStartReportContains(report.Item, value) {
			return false
		}
	}
	return true
}

func preStartSensitiveInputs(provider compozyconfig.ProviderConfig, env *ProbeEnv) []string {
	launchResolution := env.launchResolution()
	values := []string{
		strings.TrimSpace(provider.Command),
		strings.TrimSpace(provider.AuthStatusCmd),
		strings.TrimSpace(provider.AuthLoginCmd),
		launchResolution.ResolvedExecutable(),
		authStatusCommandExecutable(env),
	}
	for _, slot := range provider.EffectiveCredentialSlots() {
		values = append(values, strings.TrimSpace(slot.SecretRef))
	}
	for _, entry := range env.CommandEnv {
		_, value, ok := strings.Cut(entry, "=")
		if ok && strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	return values
}

func preStartReportContains(item *diagcontract.DiagnosticItem, value string) bool {
	if item == nil {
		return false
	}
	for _, field := range []string{
		item.ID,
		item.Code,
		item.Severity,
		item.Category,
		item.Title,
		item.Message,
		item.SuggestedCommand,
		item.DocURL,
		item.DataFreshness,
	} {
		if strings.Contains(field, value) {
			return true
		}
	}
	return preStartEvidenceContains(item.Evidence, value)
}

func preStartEvidenceContains(evidence map[string]any, value string) bool {
	for key, nested := range evidence {
		if strings.Contains(key, value) || preStartEvidenceValueContains(nested, value) {
			return true
		}
	}
	return false
}

func preStartEvidenceValueContains(nested any, value string) bool {
	switch typed := nested.(type) {
	case string:
		return strings.Contains(typed, value)
	case []byte:
		return strings.Contains(string(typed), value)
	case json.RawMessage:
		return strings.Contains(string(typed), value)
	case []string:
		for _, item := range typed {
			if strings.Contains(item, value) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if preStartEvidenceValueContains(item, value) {
				return true
			}
		}
	case map[string]any:
		return preStartEvidenceContains(typed, value)
	case map[string]string:
		for key, item := range typed {
			if strings.Contains(key, value) || strings.Contains(item, value) {
				return true
			}
		}
	}
	return false
}

func clonePreStartEvidence(evidence map[string]any) (map[string]any, bool) {
	if len(evidence) == 0 {
		return nil, true
	}
	cloned := make(map[string]any, len(evidence))
	for key, value := range evidence {
		copyValue, cloneable := clonePreStartEvidenceValue(value)
		if !cloneable {
			return nil, false
		}
		cloned[key] = copyValue
	}
	return cloned, true
}

func clonePreStartEvidenceValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number, time.Time:
		return typed, true
	case []byte:
		return append([]byte(nil), typed...), true
	case json.RawMessage:
		return append(json.RawMessage(nil), typed...), true
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		cloned := make([]any, len(typed))
		for index, nested := range typed {
			copyValue, cloneable := clonePreStartEvidenceValue(nested)
			if !cloneable {
				return nil, false
			}
			cloned[index] = copyValue
		}
		return cloned, true
	case map[string]any:
		return clonePreStartEvidence(typed)
	case map[string]string:
		cloned := make(map[string]string, len(typed))
		maps.Copy(cloned, typed)
		return cloned, true
	default:
		return nil, false
	}
}
