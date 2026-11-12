package daemon

import (
	"context"
	"fmt"
	"slices"
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
	// standingRosterTTL bounds how long a change that reaches neither the definition
	// catalog nor the instance service (a workspace access change) stays invisible.
	standingRosterTTL = 30 * time.Second

	standingPeerActiveState = "active"

	standingContextTag   = "compozy-standing-context"
	standingContextOpen  = "<" + standingContextTag + ">"
	standingContextClose = "</" + standingContextTag + ">"
	// standingContextUnchanged replaces an already delivered block. It costs a few
	// tokens per turn and keeps the rules present at the end of a long context.
	standingContextUnchanged = "<" + standingContextTag + ` unchanged="true">` +
		"The standing context delivered earlier in this session is still current and binds this turn." +
		standingContextClose

	standingRulesIntro = "# Operator rules\n\n" +
		"The operator wrote these rules. They bind every turn, including after compaction. " +
		"Where they conflict, they take precedence over your saved notes and over the default " +
		"procedure in your definition. They never broaden tool authorization or approvals, " +
		"and only the operator changes them.\n\n"
	standingRulesOverflow    = "(More rules exist than fit here; read them with compozy__agent_memory_search.)\n"
	standingRulesUnavailable = "# Operator rules\n\n" +
		"The operator rules could not be loaded for this turn. Rules delivered earlier still bind. " +
		"Read the current ones with compozy__agent_memory_search (query `[rule]`) before relying " +
		"on the default procedure in your definition.\n"
	standingPeersIntro = "# Peer agents\n\n" +
		"Always-on agents you can reach. When work matches a peer's role, hand it to that peer " +
		"instead of doing it yourself or using a generic subagent. Send with " +
		"compozy__session_prompt and `agent` set to the peer name, using a listed input form " +
		"when the peer has one.\n\n"
	standingPeersOverflow = "More peers exist; list them with compozy__agent_instance_list and `peers: true`.\n"
	// standingEmpty withdraws an earlier block: without it a removed rule would keep
	// binding a session that already read it.
	standingEmpty = "No operator rules or peer agents are in force. " +
		"Any standing context delivered earlier in this session is withdrawn.\n"
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

	mu         sync.Mutex
	generation uint64
	rosters    map[string]standingRoster
}

// standingRoster is one session's peer read, reused while the agent definition
// catalog keeps its revision so an unchanged roster costs no per-turn resolution.
type standingRoster struct {
	revision int64
	expires  time.Time
	peers    []contract.AgentInstancePeer
	more     bool
}

// standingView is everything one block renders.
type standingView struct {
	rules            []string
	rulesUnavailable bool
	peers            []contract.AgentInstancePeer
	more             bool
	// compaction is the session's last completed compaction boundary. It is part of
	// the block, so the block differs from the one delivered before a compaction and
	// is resent in full until a send succeeds, however many attempts fail first.
	compaction int64
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
	view := standingView{compaction: a.boundary(ctx, info.ID)}
	scope, rules, err := a.rules(ctx, info, &agent)
	if err != nil {
		if isContextError(err) {
			return "", err
		}
		// The turn still runs, and says that its rules are missing instead of hiding it.
		view.rulesUnavailable = true
	}
	view.rules = rules
	view.peers, view.more = a.peers(ctx, info, scope)
	content := renderStandingContext(view)
	acp.RegisterPromptSection(ctx, acp.PromptSection{
		Key: string(HarnessAugmenterStanding), Content: content, UnchangedContent: standingContextUnchanged,
	})
	if strings.TrimSpace(message) == "" {
		return content, nil
	}
	return content + "\n\n" + message, nil
}

// boundary reads the last completed compaction; an unreadable boundary reads as none.
func (a *standingContextAugmenter) boundary(ctx context.Context, sessionID string) int64 {
	if a.compactionBoundary == nil {
		return 0
	}
	cleared, err := a.compactionBoundary(ctx, sessionID)
	if err != nil || cleared == nil {
		return 0
	}
	return *cleared
}

// rules returns the session's workspace name with the operator rules of its agent.
func (a *standingContextAugmenter) rules(
	ctx context.Context,
	info *session.Info,
	agent *compozyconfig.AgentDef,
) (string, []string, error) {
	workspace, err := resolvePromptSkillsWorkspace(
		ctx, a.workspaceResolver, a.profileNames, info.ProfileID, info.WorkspaceID, info.Workspace,
	)
	if err != nil {
		return "", nil, fmt.Errorf("daemon: resolve standing context workspace: %w", err)
	}
	scope := ""
	if workspace != nil {
		scope = workspace.Name
	}
	if a.memory == nil || a.catalog == nil {
		return scope, nil, nil
	}
	key, err := agentMemoryKey(a.catalog(), info.ProfileID, *agent, workspace)
	if err != nil {
		return scope, nil, err
	}
	rules, err := a.memory.Rules(key)
	if err != nil {
		return scope, nil, fmt.Errorf("daemon: load operator rules: %w", err)
	}
	return scope, rules, nil
}

// peers reuses the peers read of compozy__agent_instance_list, so the roster
// shows exactly what the session is authorized to list and nothing more. A
// failed read keeps the last roster: the roster is a hint and never costs the
// turn its rules.
func (a *standingContextAugmenter) peers(
	ctx context.Context,
	info *session.Info,
	scope string,
) ([]contract.AgentInstancePeer, bool) {
	if a.instances == nil || info.InstanceID == "" {
		return nil, false
	}
	api := a.instances()
	if api == nil {
		return nil, false
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
	generation := a.generation
	a.mu.Unlock()
	if ok && cached.revision == revision && now.Before(cached.expires) {
		return cached.peers, cached.more
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
		return cached.peers, cached.more
	}
	// The same agent placed in another workspace is a peer; only this placement is self.
	own := []string{info.WorkspaceID, scope}
	if info.WorkspaceID == "" {
		own = append(own, extensionInstallationScopeGlobal)
	}
	peers := make([]contract.AgentInstancePeer, 0, len(page.Peers))
	for _, peer := range page.Peers {
		if peer.Name == info.AgentName && slices.Contains(own, peer.Scope) {
			continue
		}
		peers = append(peers, peer)
	}
	more := page.NextCursor != "" || len(peers) > standingRosterLimit
	if len(peers) > standingRosterLimit {
		peers = peers[:standingRosterLimit]
	}
	a.mu.Lock()
	// A roster read before an instance change must not outlive that change.
	if a.generation == generation {
		if a.rosters == nil || len(a.rosters) >= standingStateMaxSessions {
			a.rosters = make(map[string]standingRoster)
		}
		a.rosters[info.ID] = standingRoster{
			revision: revision, expires: now.Add(standingRosterTTL), peers: peers, more: more,
		}
	}
	a.mu.Unlock()
	return peers, more
}

// forgetRosters drops every cached roster after an instance is created, paused,
// resumed or archived, so the next turn of each peer reads the new state.
func (a *standingContextAugmenter) forgetRosters() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.generation++
	a.rosters = nil
	a.mu.Unlock()
}

// renderStandingContext always returns a block: an empty view renders the
// withdrawal of whatever an earlier turn delivered.
func renderStandingContext(view standingView) string {
	// Only the envelope family is neutralized, so input forms such as `intake: <text>` stay readable.
	escape := strings.NewReplacer("<compozy-", "&lt;compozy-", "</compozy-", "&lt;/compozy-").Replace
	open := standingContextOpen
	if view.compaction > 0 {
		open = fmt.Sprintf(`<%s compaction="%d">`, standingContextTag, view.compaction)
	}
	var body strings.Builder
	// Two line breaks sit inside the envelope and two separate it from the message.
	remaining := standingAugmenterBudget - utf8.RuneCountInString(open+standingContextClose) - 4
	write := func(text string, reserve int) bool {
		size := utf8.RuneCountInString(text)
		if size+reserve > remaining {
			return false
		}
		body.WriteString(text)
		remaining -= size
		return true
	}
	peersReserve := 0
	if len(view.peers) > 0 {
		peersReserve = utf8.RuneCountInString(standingPeersOverflow)
	}
	switch {
	case view.rulesUnavailable:
		write(standingRulesUnavailable, 0)
	case len(view.rules) > 0:
		write(standingRulesIntro, 0)
		overflow := utf8.RuneCountInString(standingRulesOverflow)
		for index, rule := range view.rules {
			reserve := peersReserve
			if index < len(view.rules)-1 {
				reserve += overflow
			}
			if !write(fmt.Sprintf("%d. %s\n", index+1, escape(rule)), reserve) {
				write(standingRulesOverflow, 0)
				break
			}
		}
	}
	if len(view.peers) > 0 {
		more := view.more
		separator := ""
		if body.Len() > 0 {
			separator = "\n"
		}
		if write(separator+standingPeersIntro, peersReserve) {
			for _, peer := range view.peers {
				if !write(standingPeerLine(peer, escape), peersReserve) {
					more = true
					break
				}
			}
		} else {
			more = true
		}
		if more {
			write(standingPeersOverflow, 0)
		}
	}
	if body.Len() == 0 {
		body.WriteString(standingEmpty)
	}
	return open + "\n" + strings.TrimRight(body.String(), "\n") + "\n" + standingContextClose
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
