package session

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/mention"
)

// promptMentionInputs is what one dispatch hands internal/acp for the request's verified mentions.
type promptMentionInputs struct {
	links      []acp.PromptResourceLink
	references string
	delivery   acp.PromptMentionDelivery
}

func (inputs promptMentionInputs) apply(req *acp.PromptRequest) {
	req.MentionLinks = inputs.links
	req.SessionReferences = inputs.references
	req.MentionDelivery = inputs.delivery
}

// promptMentionInputs re-checks every admitted file/folder target against the session's persisted working
// directory, builds one resource link per distinct surviving target, and asks the injected renderer for
// the session-references block. Absolute paths exist only here, in the link URI; nothing persists them.
func (m *Manager) promptMentionInputs(
	ctx context.Context,
	session *Session,
	req *promptRequest,
) promptMentionInputs {
	inputs := promptMentionInputs{}
	if req == nil || req.delivery == PromptDeliveryMaintenance {
		return inputs
	}
	inputs.delivery.DroppedOverCap = mentionsDroppedOverCap(req.authoredMessage, req.mentions)
	paths, sessions := distinctMentionTargets(req.mentions)
	if len(paths) > 0 {
		inputs.links, inputs.delivery.DroppedAtDispatch = m.dispatchMentionLinks(ctx, session, paths)
	}
	if len(sessions) > 0 && m.mentionRenderer != nil {
		references, err := m.mentionRenderer(ctx, session, sessions, promptHumanIngress(req))
		if err != nil {
			m.sessionLogger(session).WarnContext(ctx, "session: render mention references failed", "error", err)
		} else if strings.TrimSpace(references.Text) != "" {
			inputs.references = references.Text
			inputs.delivery.Sessions = references.Sessions
			inputs.delivery.SessionIDs = slices.Clone(references.SessionIDs)
			inputs.delivery.ReferencesForm = string(references.Form)
		}
	}
	return inputs
}

func (m *Manager) dispatchMentionLinks(
	ctx context.Context,
	session *Session,
	paths []mention.Mention,
) ([]acp.PromptResourceLink, int) {
	workingDir, err := m.WorkingDir(ctx, session.ID)
	if err != nil {
		m.sessionLogger(session).WarnContext(ctx, "session: resolve mention working directory failed", "error", err)
		return nil, len(paths)
	}
	links := make([]acp.PromptResourceLink, 0, len(paths))
	dropped := 0
	for _, target := range paths {
		relative := strings.TrimSuffix(target.Target, "/")
		kind, ok := probeMentionPath(workingDir, relative)
		if !ok || kind != target.Kind {
			dropped++
			continue
		}
		absolute := filepath.Join(workingDir, filepath.FromSlash(relative))
		links = append(links, acp.NewPromptResourceLink(target.Target, absolute, kind == mention.KindFolder))
	}
	if len(links) == 0 {
		return nil, dropped
	}
	return links, dropped
}

// distinctMentionTargets splits mentions into one entry per distinct path target and per distinct session,
// in first-mention order: repeated tokens keep their chips but produce one delivery entry.
func distinctMentionTargets(mentions []mention.Mention) (paths []mention.Mention, sessions []mention.Mention) {
	seen := make(map[string]struct{}, len(mentions))
	for _, item := range mentions {
		key := string(item.Kind) + "\x00" + item.Target
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item.Kind == mention.KindSession {
			sessions = append(sessions, item)
		} else {
			paths = append(paths, item)
		}
	}
	return paths, sessions
}

// mentionsDroppedOverCap counts candidate tokens left as text because a per-prompt cap was reached: those
// past the candidate cap, and those whose distinct-target cap was already full when they appeared. It
// derives the count from the authored text and the admitted mentions alone, so queued, replayed, and
// restarted dispatches report the same number.
func mentionsDroppedOverCap(authored string, mentions []mention.Mention) int {
	pathTargets, sessionTargets := distinctMentionTargets(mentions)
	tokens, skipped := mention.ParseCounted(authored)
	pathFull := len(pathTargets) >= mention.MaxPathMentions
	sessionFull := len(sessionTargets) >= mention.MaxSessionMentions
	if !pathFull && !sessionFull {
		return skipped
	}
	admitted := make(map[string]struct{}, len(mentions))
	for _, item := range mentions {
		admitted[item.Token] = struct{}{}
	}
	pathCapAt := capReachedOffset(mentions, mention.MaxPathMentions, false)
	sessionCapAt := capReachedOffset(mentions, mention.MaxSessionMentions, true)
	dropped := skipped
	for _, token := range tokens {
		if _, ok := admitted[token.Raw]; ok {
			continue
		}
		isSession := strings.TrimSpace(token.SessionID) != ""
		if isSession && sessionFull && token.Start >= sessionCapAt {
			dropped++
		}
		if !isSession && pathFull && token.Start >= pathCapAt {
			dropped++
		}
	}
	return dropped
}

// capReachedOffset returns where the cap-th distinct target of one family was admitted.
func capReachedOffset(mentions []mention.Mention, capacity int, sessions bool) int {
	seen := map[string]struct{}{}
	for _, item := range mentions {
		if (item.Kind == mention.KindSession) != sessions {
			continue
		}
		seen[item.Target] = struct{}{}
		if len(seen) == capacity {
			return item.End
		}
	}
	return 0
}

// promptHumanIngress reports human operator ingress for one dispatch: a user turn without a
// session-message origin whose durable admission recorded the human operator as its author.
func promptHumanIngress(req *promptRequest) bool {
	return req.humanIngress && req.turnSource != TurnSourceSynthetic && req.meta.Origin == nil
}
