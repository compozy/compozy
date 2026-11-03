package situation

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/internal/session"
)

const promptSectionCacheMaxSessions = 2048

type promptSectionCache struct {
	sequence atomic.Uint64
	mu       sync.Mutex
	states   map[string]promptSectionCacheState
}

type promptSectionCacheState struct {
	acpSessionID string
	workspaceKey string
	signatures   map[string][sha256.Size]byte
	lastUsed     uint64
	boundary     int64
	used         int64
	usageKnown   bool
	reminded     bool
}

type promptSectionUnchangedPayload struct {
	Unchanged bool   `json:"unchanged"`
	Section   string `json:"section"`
	Reuse     string `json:"reuse"`
}

func newPromptSectionCache() *promptSectionCache {
	return &promptSectionCache{
		states: make(map[string]promptSectionCacheState),
	}
}

func (c *promptSectionCache) compact(
	info *session.Info,
	sections []renderedSection,
	boundary int64, observation ContextObservation,
) ([]renderedSection, bool, bool) {
	if c == nil || info == nil || len(sections) == 0 {
		return sections, false, false
	}
	sessionID := strings.TrimSpace(info.ID)
	acpSessionID := strings.TrimSpace(info.ACPSessionID)
	if sessionID == "" || acpSessionID == "" {
		return sections, false, false
	}
	workspaceKey := firstTrimmed(info.WorkspaceID, info.Workspace)
	signatures := promptSectionSignatures(sections)
	sequence := c.sequence.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()

	previous, ok := c.states[sessionID]

	sameContext := ok && previous.acpSessionID == acpSessionID && previous.workspaceKey == workspaceKey
	next := previous
	compactedSinceLastPrompt, usageDropped, reminder := next.observeMaintenance(boundary, observation, sameContext)
	canReuse := sameContext && !compactedSinceLastPrompt && !usageDropped
	compacted := sections
	if canReuse {
		compacted = compactUnchangedPromptSections(sections, previous.signatures, signatures)
	}

	next.acpSessionID, next.workspaceKey = acpSessionID, workspaceKey
	next.signatures, next.lastUsed = signatures, sequence
	c.states[sessionID] = next
	c.evictOldestLocked()
	return compacted, compactedSinceLastPrompt, reminder
}

func promptSectionSignatures(sections []renderedSection) map[string][sha256.Size]byte {
	signatures := make(map[string][sha256.Size]byte, len(sections))
	for _, section := range sections {
		signatures[section.name] = sha256.Sum256(section.raw)
	}
	return signatures
}

func compactUnchangedPromptSections(
	sections []renderedSection,
	previous map[string][sha256.Size]byte,
	current map[string][sha256.Size]byte,
) []renderedSection {
	compacted := make([]renderedSection, len(sections))
	for idx, section := range sections {
		compacted[idx] = section
		previousSignature, ok := previous[section.name]
		if !ok || previousSignature != current[section.name] {
			continue
		}
		raw, err := json.Marshal(promptSectionUnchangedPayload{
			Unchanged: true,
			Section:   section.name,
			Reuse:     "previous_full_section_same_acp_session",
		})
		if err != nil {
			// The marker has constant shape; keep the full section if encoding ever fails.
			continue
		}
		compacted[idx].raw = raw
	}
	return compacted
}

func (c *promptSectionCache) evictOldestLocked() {
	if len(c.states) <= promptSectionCacheMaxSessions {
		return
	}

	var oldestKey string
	oldestSequence := ^uint64(0)
	for key, state := range c.states {
		if state.lastUsed < oldestSequence {
			oldestKey = key
			oldestSequence = state.lastUsed
		}
	}
	if oldestKey != "" {
		delete(c.states, oldestKey)
	}
}

func (state *promptSectionCacheState) observeMaintenance(
	boundary int64,
	observation ContextObservation,
	sameContext bool,
) (bool, bool, bool) {
	compacted := boundary > state.boundary
	dropped := observation.Known && state.usageKnown && observation.Used < state.used
	if compacted || dropped || !sameContext {
		state.reminded = false
	}
	reminder := observation.Main && observation.Known && observation.Size > 0 && observation.Threshold > 0 &&
		float64(observation.Used)/float64(observation.Size) >= observation.Threshold && !state.reminded
	if observation.Known {
		state.used, state.usageKnown = observation.Used, true
	}
	state.reminded = state.reminded || reminder
	state.boundary = boundary
	return compacted, dropped, reminder
}
