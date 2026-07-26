package situation

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compozy/agh/internal/session"
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

func (c *promptSectionCache) compact(info *session.Info, sections []renderedSection) []renderedSection {
	if c == nil || info == nil || len(sections) == 0 {
		return sections
	}
	sessionID := strings.TrimSpace(info.ID)
	acpSessionID := strings.TrimSpace(info.ACPSessionID)
	if sessionID == "" || acpSessionID == "" {
		return sections
	}
	workspaceKey := firstTrimmed(info.WorkspaceID, info.Workspace)
	signatures := promptSectionSignatures(sections)
	sequence := c.sequence.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()

	previous, ok := c.states[sessionID]
	canReuse := ok &&
		previous.acpSessionID == acpSessionID &&
		previous.workspaceKey == workspaceKey
	compacted := sections
	if canReuse {
		compacted = compactUnchangedPromptSections(sections, previous.signatures, signatures)
	}
	c.states[sessionID] = promptSectionCacheState{
		acpSessionID: acpSessionID,
		workspaceKey: workspaceKey,
		signatures:   signatures,
		lastUsed:     sequence,
	}
	c.evictOldestLocked()
	return compacted
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
