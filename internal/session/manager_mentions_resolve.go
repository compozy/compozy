package session

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/mention"
	"github.com/compozy/compozy/internal/session/inputqueue"
	"github.com/compozy/compozy/internal/store"
)

// mentionResolution counts one admission's mention outcome for the debug log.
type mentionResolution struct {
	files    int
	folders  int
	sessions int
	// candidatesSkipped counts candidate tokens past mention.MaxCandidateTokens, left as text unprobed.
	candidatesSkipped int
	// unresolved counts probed candidates that did not resolve and stay plain text.
	unresolved int
	// dropped counts candidates left as text because a distinct-target cap was reached.
	dropped int
}

// prepareMentions resolves the authored message's @-tokens against the target session. It runs on every
// admission path for every caller, independent of command ingress: it neither reads nor grants
// AllowCommands. Tokens that do not resolve stay plain text and never reject the prompt.
func (m *Manager) prepareMentions(ctx context.Context, req *promptRequest) {
	if req == nil {
		return
	}
	req.mentions = nil
	tokens, skipped := mention.ParseCounted(req.authoredMessage)
	if len(tokens) == 0 {
		return
	}
	mentions, stats := m.resolveMentionTokens(ctx, req.target, tokens)
	stats.candidatesSkipped = skipped
	req.mentions = mentions
	m.sessionLogger(nil).DebugContext(ctx, "mention.resolved", "session_id", req.target,
		"files", stats.files,
		"folders", stats.folders,
		"sessions", stats.sessions,
		"candidates_skipped", stats.candidatesSkipped,
		"unresolved", stats.unresolved,
		"dropped", stats.dropped,
	)
}

// resolveMentionTokens verifies tokens against the target session's working directory, workspace, and
// profile, never the caller's. Distinct targets are capped in first-occurrence order; repeated tokens for
// one admitted target all stay mentions.
func (m *Manager) resolveMentionTokens(
	ctx context.Context,
	targetID string,
	tokens []mention.Token,
) ([]mention.Mention, mentionResolution) {
	stats := mentionResolution{}
	resolver := m.newMentionResolver(ctx, targetID)
	mentions := make([]mention.Mention, 0, len(tokens))
	for _, token := range tokens {
		resolved, ok, overCap := resolver.resolve(ctx, token)
		switch {
		case overCap:
			stats.dropped++
		case !ok:
			stats.unresolved++
		default:
			mentions = append(mentions, resolved)
		}
	}
	stats.files, stats.folders, stats.sessions = resolver.counts()
	if len(mentions) == 0 {
		return nil, stats
	}
	return mentions, stats
}

type mentionLookup struct {
	value mention.Mention
	ok    bool
}

type mentionResolver struct {
	manager    *Manager
	targetID   string
	targetMeta *store.SessionMeta
	workingDir string
	// paths and sessions memoize every probed target so a repeated token never probes twice.
	paths       map[string]mentionLookup
	sessions    map[string]mentionLookup
	pathCount   int
	folderCount int
	fileCount   int
	sessionHits int
	loaded      bool
}

func (m *Manager) newMentionResolver(_ context.Context, targetID string) *mentionResolver {
	return &mentionResolver{
		manager:  m,
		targetID: strings.TrimSpace(targetID),
		paths:    map[string]mentionLookup{},
		sessions: map[string]mentionLookup{},
	}
}

func (r *mentionResolver) counts() (files, folders, sessions int) {
	return r.fileCount, r.folderCount, r.sessionHits
}

// load reads the target session once, lazily, so prompts without tokens never touch metadata.
func (r *mentionResolver) load(ctx context.Context) {
	if r.loaded {
		return
	}
	r.loaded = true
	meta, err := r.manager.readSessionMetaReadOnly(ctx, r.targetID)
	if err != nil {
		return
	}
	r.targetMeta = &meta
	workingDir, err := r.manager.WorkingDir(ctx, r.targetID)
	if err != nil {
		return
	}
	r.workingDir = workingDir
}

func (r *mentionResolver) resolve(ctx context.Context, token mention.Token) (mention.Mention, bool, bool) {
	r.load(ctx)
	if strings.TrimSpace(token.SessionID) != "" {
		return r.resolveSession(ctx, token)
	}
	return r.resolvePath(token)
}

func (r *mentionResolver) resolvePath(token mention.Token) (mention.Mention, bool, bool) {
	relative, ok := mentionRelativePath(token.Path)
	if !ok || r.workingDir == "" {
		return mention.Mention{}, false, false
	}
	lookup, seen := r.paths[relative]
	if !seen {
		if r.pathCount >= mention.MaxPathMentions {
			return mention.Mention{}, false, true
		}
		lookup = r.probePath(relative)
		r.paths[relative] = lookup
		if lookup.ok {
			r.pathCount++
			if lookup.value.Kind == mention.KindFolder {
				r.folderCount++
			} else {
				r.fileCount++
			}
		}
	}
	if !lookup.ok {
		return mention.Mention{}, false, false
	}
	return withMentionToken(lookup.value, token), true, false
}

// probePath stats one relative target inside the working directory after symlink resolution.
func (r *mentionResolver) probePath(relative string) mentionLookup {
	probe := probeMentionPath
	if r.manager.mentionPathProbe != nil {
		probe = r.manager.mentionPathProbe
	}
	kind, ok := probe(r.workingDir, relative)
	if !ok {
		return mentionLookup{}
	}
	target := relative
	label := path.Base(relative)
	if kind == mention.KindFolder {
		target += "/"
		label += "/"
	}
	return mentionLookup{ok: true, value: mention.Mention{Kind: kind, Target: target, Label: label}}
}

func (r *mentionResolver) resolveSession(ctx context.Context, token mention.Token) (mention.Mention, bool, bool) {
	id := strings.TrimSpace(token.SessionID)
	if r.targetMeta == nil || id == r.targetID {
		return mention.Mention{}, false, false
	}
	lookup, seen := r.sessions[id]
	if !seen {
		if r.sessionHits >= mention.MaxSessionMentions {
			return mention.Mention{}, false, true
		}
		lookup = r.probeSession(ctx, id)
		r.sessions[id] = lookup
		if lookup.ok {
			r.sessionHits++
		}
	}
	if !lookup.ok {
		return mention.Mention{}, false, false
	}
	return withMentionToken(lookup.value, token), true, false
}

// probeSession admits another live or archived session of the target's workspace and profile. The
// lookup is scoped to that workspace and profile in the catalog query itself, so nothing about a session
// outside the scope (its title included) is ever read. A deleted session has no catalog row and stays text.
func (r *mentionResolver) probeSession(ctx context.Context, id string) mentionLookup {
	catalog := r.manager.sessionCatalog
	workspaceID := strings.TrimSpace(r.targetMeta.WorkspaceID)
	profileID := strings.TrimSpace(r.targetMeta.ProfileID)
	if catalog == nil || workspaceID == "" || profileID == "" {
		return mentionLookup{}
	}
	entries, err := catalog.ListSessions(ctx, store.SessionListQuery{
		ReadScope:   store.ReadScope{ProfileID: profileID},
		WorkspaceID: workspaceID,
		ID:          id,
		Archive:     store.SessionArchiveInclude,
		Limit:       1,
	})
	if err != nil || len(entries) == 0 || strings.TrimSpace(entries[0].ID) != id {
		return mentionLookup{}
	}
	label := strings.TrimSpace(entries[0].Name)
	if label == "" {
		label = id
	}
	return mentionLookup{ok: true, value: mention.Mention{Kind: mention.KindSession, Target: id, Label: label}}
}

func withMentionToken(value mention.Mention, token mention.Token) mention.Mention {
	value.Token = token.Raw
	value.Start = token.Start
	value.End = token.End
	return value
}

// mentionRelativePath normalizes a token path to a slash-separated path relative to the working
// directory. Absolute paths and paths that lexically leave the directory never resolve.
func mentionRelativePath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) {
		return "", false
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) ||
		filepath.VolumeName(trimmed) != "" {
		return "", false
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

// probeMentionPath reports the kind of relative under workingDir when its real path stays inside the
// real working directory: a directory is a folder, a regular file is a file, anything else is absent.
func probeMentionPath(workingDir string, relative string) (mention.Kind, bool) {
	candidate := filepath.Join(workingDir, filepath.FromSlash(relative))
	_, resolved, err := fileutil.ResolveExistingPathWithinRoot(workingDir, candidate)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false
	}
	switch {
	case info.IsDir():
		return mention.KindFolder, true
	case info.Mode().IsRegular():
		return mention.KindFile, true
	default:
		return "", false
	}
}

// humanPromptCaller reports human operator ingress, the same signal that enables slash commands.
func humanPromptCaller(caller PromptCaller) bool {
	return strings.EqualFold(strings.TrimSpace(caller.Kind), "human")
}

func cloneMentions(mentions []mention.Mention) []mention.Mention {
	if len(mentions) == 0 {
		return nil
	}
	return append([]mention.Mention(nil), mentions...)
}

// storedMentions decodes the mentions_json persisted on a queue row. The store validates the column as a
// JSON array on every write, so an undecodable value can only be a foreign shape; it carries no mentions.
func storedMentions(raw string) []mention.Mention {
	mentions, err := inputqueue.DecodeMentions(raw)
	if err != nil {
		return nil
	}
	return mentions
}

// hasStoredMentions reports whether a persisted mentions_json value carries at least one mention.
func hasStoredMentions(raw string) bool {
	return inputqueue.HasMentions(raw)
}
