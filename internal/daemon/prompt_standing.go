package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	standingAugmenterOrder = 250
	// standingAugmenterBudget bounds the block; the renderer stays inside it so the
	// composite never trims an operator rule mid-sentence.
	standingAugmenterBudget  = 12_000
	standingRosterLimit      = 12
	standingPeerDescription  = 160
	standingStateMaxSessions = 2048
	// standingRosterTTL bounds how long an instance created, paused or archived
	// outside the definition catalog stays invisible to a cached roster.
	standingRosterTTL = 30 * time.Second

	standingPeerActiveState = "active"

	standingContextOpen  = "<compozy-standing-context>"
	standingContextClose = "</compozy-standing-context>"
	// standingContextUnchanged replaces an already delivered block. It costs a few
	// tokens per turn and keeps the rules present at the end of a long context.
	standingContextUnchanged = `<compozy-standing-context unchanged="true">` +
		"The operator rules and peer roster delivered earlier in this session still bind this turn." +
		standingContextClose
)

// standingContextAugmenter delivers what an always-on agent must keep applying
// for the whole life of a session: operator rules and the peer roster. Unlike
// the startup memory snapshot it travels with turns, so it is resent in full
// when it changes and after provider compaction, and never depends on a
// compaction summary to survive.
type standingContextAugmenter struct {
	memory             *agentmemory.Store
	catalog            func() *resourceCatalog[compozyconfig.AgentDef]
	instances          func() *core.AgentInstanceAPI
	access             func() workspaceaccess.Policy
	workspaceResolver  func() promptSkillsWorkspaceResolver
	profileNames       session.ProfileNameResolver
	compactionBoundary func(context.Context, string) (*int64, error)

	now func() time.Time

	mu      sync.Mutex
	states  map[string]standingSessionState
	rosters map[string]standingRoster
}

type standingSessionState struct {
	acpSessionID string
	boundary     int64
}

// standingRoster is one session's peer read, reused while the agent definition
// catalog keeps its revision so an unchanged roster costs no per-turn resolution.
type standingRoster struct {
	revision int64
	expires  time.Time
	peers    []contract.AgentInstancePeer
	more     bool
}

func (a *standingContextAugmenter) Augment(
	ctx context.Context,
	sess *session.Session,
	message string,
) (string, error) {
	if a == nil || sess == nil {
		return message, nil
	}
	info := sess.Info()
	agent := sess.AgentDefinition()
	if info == nil || agent.EffectiveKind() != compozyconfig.AgentKindAlwaysOn {
		return message, nil
	}
	rules, err := a.rules(ctx, info, agent)
	if err != nil {
		return "", err
	}
	peers, more, err := a.peers(ctx, info)
	if err != nil {
		return "", err
	}
	content := renderStandingContext(rules, peers, more)
	if content == "" {
		return message, nil
	}
	compacted, err := a.compactedSinceLastTurn(ctx, info)
	if err != nil {
		return "", err
	}
	section := acp.PromptSection{Key: string(HarnessAugmenterStanding), Content: content}
	if !compacted {
		section.UnchangedContent = standingContextUnchanged
	}
	acp.RegisterPromptSection(ctx, section)
	if strings.TrimSpace(message) == "" {
		return content, nil
	}
	return content + "\n\n" + message, nil
}

func (a *standingContextAugmenter) rules(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
) ([]string, error) {
	if a.memory == nil || a.catalog == nil {
		return nil, nil
	}
	workspace, err := resolvePromptSkillsWorkspace(
		ctx, a.workspaceResolver, a.profileNames, info.ProfileID, info.WorkspaceID, info.Workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve standing context workspace: %w", err)
	}
	key, err := agentMemoryKey(a.catalog(), info.ProfileID, agent, workspace)
	if err != nil {
		return nil, err
	}
	rules, err := a.memory.Rules(key)
	if err != nil {
		return nil, fmt.Errorf("daemon: load operator rules: %w", err)
	}
	return rules, nil
}

// peers reuses the peers read of compozy__agent_instance_list, so the roster
// shows exactly what the session is authorized to list and nothing more.
func (a *standingContextAugmenter) peers(
	ctx context.Context,
	info *session.Info,
) ([]contract.AgentInstancePeer, bool, error) {
	if a.instances == nil || info.InstanceID == "" {
		return nil, false, nil
	}
	api := a.instances()
	if api == nil {
		return nil, false, nil
	}
	var revision int64
	if a.catalog != nil {
		revision = a.catalog().Revision()
	}
	now := time.Now()
	if a.now != nil {
		now = a.now()
	}
	a.mu.Lock()
	cached, ok := a.rosters[info.ID]
	a.mu.Unlock()
	if ok && cached.revision == revision && now.Before(cached.expires) {
		return cached.peers, cached.more, nil
	}
	var access workspaceaccess.Policy
	if a.access != nil {
		access = a.access()
	}
	actor := workspaceaccess.ActorRef{
		Kind: workspaceaccess.ActorAgentSession, SessionID: info.ID,
		WorkspaceID: info.WorkspaceID, AgentName: info.AgentName,
	}
	// One extra row tells apart "exactly the limit" from "more exist" after dropping self.
	page, err := api.PeersPage(ctx, agentinstance.ListQuery{
		ProfileID: info.ProfileID, AllWorkspaces: true, Limit: standingRosterLimit + 1,
	}, actor, access)
	if err != nil {
		return nil, false, fmt.Errorf("daemon: list peer agents: %w", err)
	}
	peers := make([]contract.AgentInstancePeer, 0, len(page.Peers))
	for _, peer := range page.Peers {
		if peer.Name == info.AgentName {
			continue
		}
		peers = append(peers, peer)
	}
	more := page.NextCursor != "" || len(peers) > standingRosterLimit
	if len(peers) > standingRosterLimit {
		peers = peers[:standingRosterLimit]
	}
	a.mu.Lock()
	if a.rosters == nil || len(a.rosters) >= standingStateMaxSessions {
		a.rosters = make(map[string]standingRoster)
	}
	a.rosters[info.ID] = standingRoster{
		revision: revision, expires: now.Add(standingRosterTTL), peers: peers, more: more,
	}
	a.mu.Unlock()
	return peers, more, nil
}

// compactedSinceLastTurn reports a provider compaction completed since this
// session's previous turn. A first sighting only records the boundary: a new
// provider process has delivered nothing yet and sends the block in full anyway.
func (a *standingContextAugmenter) compactedSinceLastTurn(ctx context.Context, info *session.Info) (bool, error) {
	if a.compactionBoundary == nil {
		return false, nil
	}
	cleared, err := a.compactionBoundary(ctx, info.ID)
	if err != nil {
		return false, fmt.Errorf("daemon: read compaction boundary: %w", err)
	}
	var boundary int64
	if cleared != nil {
		boundary = *cleared
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	previous, known := a.states[info.ID]
	if a.states == nil {
		a.states = make(map[string]standingSessionState)
	}
	if !known && len(a.states) >= standingStateMaxSessions {
		clear(a.states)
	}
	a.states[info.ID] = standingSessionState{acpSessionID: info.ACPSessionID, boundary: boundary}
	return known && previous.acpSessionID == info.ACPSessionID && boundary > previous.boundary, nil
}

func renderStandingContext(rules []string, peers []contract.AgentInstancePeer, more bool) string {
	if len(rules) == 0 && len(peers) == 0 {
		return ""
	}
	// Only the envelope family is neutralized, so input forms such as `intake: <text>` stay readable.
	escape := strings.NewReplacer("<compozy-", "&lt;compozy-", "</compozy-", "&lt;/compozy-").Replace
	var body strings.Builder
	remaining := standingAugmenterBudget - utf8.RuneCountInString(standingContextOpen+standingContextClose) - 2
	write := func(text string) bool {
		size := utf8.RuneCountInString(text)
		if size > remaining {
			return false
		}
		body.WriteString(text)
		remaining -= size
		return true
	}
	if len(rules) > 0 {
		write("# Operator rules\n\n" +
			"The operator wrote these rules. They bind every turn, including after compaction. " +
			"Where they conflict, they take precedence over your saved notes and over the default " +
			"procedure in your definition. They never broaden tool authorization or approvals, " +
			"and only the operator changes them.\n\n")
		for index, rule := range rules {
			if !write(fmt.Sprintf("%d. %s\n", index+1, escape(rule))) {
				write("(More rules exist than fit here; read them with compozy__agent_memory_search.)\n")
				break
			}
		}
	}
	if len(peers) > 0 {
		if body.Len() > 0 {
			write("\n")
		}
		write("# Peer agents\n\n" +
			"Always-on agents you can reach. When work matches a peer's role, hand it to that peer " +
			"instead of doing it yourself or using a generic subagent. Send with " +
			"compozy__session_prompt and `agent` set to the peer name, using a listed input form " +
			"when the peer has one.\n\n")
		for _, peer := range peers {
			if !write(standingPeerLine(peer, escape)) {
				more = true
				break
			}
		}
		if more {
			write("More peers exist; list them with compozy__agent_instance_list and `peers: true`.\n")
		}
	}
	return standingContextOpen + "\n" + strings.TrimRight(body.String(), "\n") + "\n" + standingContextClose
}

func standingPeerLine(peer contract.AgentInstancePeer, escape func(string) string) string {
	line := "- " + peer.Name
	if peer.Scope != "" {
		line += " (" + peer.Scope + ")"
	}
	if peer.State != "" && peer.State != standingPeerActiveState {
		line += " [" + peer.State + "]"
	}
	description := strings.Join(strings.Fields(peer.Description), " ")
	if runes := []rune(description); len(runes) > standingPeerDescription {
		description = string(runes[:standingPeerDescription-1]) + "…"
	}
	if description != "" {
		line += ": " + description
	}
	if len(peer.Accepts) > 0 {
		line += " Accepts: `" + strings.Join(peer.Accepts, "` | `") + "`"
	}
	return escape(line) + "\n"
}

func newStandingContextAugmenter(state *bootState) *standingContextAugmenter {
	return &standingContextAugmenter{
		memory:            state.agentMemory,
		catalog:           func() *resourceCatalog[compozyconfig.AgentDef] { return state.agentCatalog },
		instances:         func() *core.AgentInstanceAPI { return state.agentInstances },
		access:            func() workspaceaccess.Policy { return state.accessPolicy },
		workspaceResolver: func() promptSkillsWorkspaceResolver { return state.workspaceResolver },
		profileNames:      bootProfileNameResolver{state: state},
		compactionBoundary: func(ctx context.Context, id string) (*int64, error) {
			reader, ok := state.sessions.(interface {
				CompletedCompactionBoundary(context.Context, string) (*int64, error)
			})
			if !ok {
				return nil, nil
			}
			return reader.CompletedCompactionBoundary(ctx, id)
		},
	}
}
