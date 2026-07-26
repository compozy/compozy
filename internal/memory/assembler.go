package memory

import (
	"context"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	memoryPromptIntro = `# Persistent Memory

Only prompt-safe MEMORY.md indexes are injected here. Show full memory entries on demand when relevant.`
	memoryTaxonomySection = `## Memory Taxonomy

- ` + "`user`" + `: stable preferences or working style that apply across projects.
- ` + "`feedback`" + `: recurring quality signals, review feedback, or mistakes to avoid next time.
- ` + "`project`" + `: current codebase decisions, ongoing work, and project-specific constraints.
- ` + "`reference`" + `: external facts, docs, or system references worth re-reading on demand.`
	memoryCommandsSection = `## Memory Commands

- ` + "`agh memory list`" + ` shows discoverable memory files in the current scope.
- ` + "`agh memory search <query>`" + ` searches durable memory before opening individual files.
- ` + "`agh memory show <filename>`" + ` shows the full content of one memory entry.
- ` + "`agh memory reindex`" + ` rebuilds the derived search catalog from Markdown memory files.
- ` + "`agh memory write --name <name> --type <type> --description <desc> --content <content>`" + ` proposes a durable memory write through the controller.`
	memoryStalenessSection = `## Staleness Policy

- Memories older than 1 day should be verified against the current repository
  or system state before asserting them as fact.`
)

// Assembler loads prompt-safe memory indexes and prepends them to the agent prompt.
type Assembler struct {
	store     *Store
	snapshots *SnapshotService
}

var (
	_ session.PromptProvider        = (*Assembler)(nil)
	_ session.ResumeContextProvider = (*Assembler)(nil)
)

// NewAssembler constructs a prompt assembler for the provided store.
func NewAssembler(store *Store, opts ...AssemblerOption) *Assembler {
	assembler := &Assembler{store: store}
	for _, opt := range opts {
		if opt != nil {
			opt(assembler)
		}
	}
	if assembler.snapshots == nil {
		assembler.snapshots = NewSnapshotService(store)
	}
	return assembler
}

// AssemblerOption customizes memory startup prompt assembly.
type AssemblerOption func(*Assembler)

// WithSnapshotService installs the frozen-snapshot service used by the assembler.
func WithSnapshotService(service *SnapshotService) AssemblerOption {
	return func(assembler *Assembler) {
		if assembler != nil && service != nil {
			assembler.snapshots = service
		}
	}
}

// WithSnapshotProvider lets the assembler source prompt blocks from an active provider.
func WithSnapshotProvider(provider SnapshotProvider) AssemblerOption {
	return func(assembler *Assembler) {
		if assembler != nil {
			assembler.snapshots = NewSnapshotService(assembler.store, WithProviderSnapshotSource(provider))
		}
	}
}

// PromptSection renders the frozen memory context block without the base agent
// prompt so it can participate in composed prompt assembly.
func (a *Assembler) PromptSection(ctx context.Context, workspace *workspacepkg.ResolvedWorkspace) (string, error) {
	if a == nil || a.snapshots == nil {
		return "", nil
	}
	snapshot, err := a.snapshots.Capture(ctx, PromptSnapshotRequest{
		WorkspaceID:   workspaceIDFromResolved(workspace),
		WorkspaceRoot: workspaceRootFromResolved(workspace),
		SessionType:   session.SessionTypeUser,
	})
	if err != nil {
		return "", err
	}
	checkpoint, err := a.checkpointSummarySection(ctx, workspaceRootFromResolved(workspace))
	if err != nil {
		return "", err
	}
	return joinAssemblerSections(snapshot.Section, checkpoint), nil
}

// PromptStartupSection renders memory using durable startup metadata.
func (a *Assembler) PromptStartupSection(
	ctx context.Context,
	startup session.StartupPromptContext,
	_ aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if a == nil || a.snapshots == nil {
		return "", nil
	}
	snapshot, err := a.snapshots.Capture(ctx, PromptSnapshotRequest{
		SessionID:     startup.SessionID,
		WorkspaceID:   firstAssemblerValue(startup.WorkspaceID, workspaceIDFromResolved(workspace)),
		WorkspaceRoot: firstAssemblerValue(startup.Workspace, workspaceRootFromResolved(workspace)),
		AgentName:     startup.AgentName,
		SessionType:   startup.SessionType,
	})
	if err != nil {
		return "", err
	}
	checkpoint, err := a.checkpointSummarySection(
		ctx,
		firstAssemblerValue(startup.Workspace, workspaceRootFromResolved(workspace)),
	)
	if err != nil {
		return "", err
	}
	return joinAssemblerSections(snapshot.Section, checkpoint), nil
}

// ResumeContextSection returns only the durable checkpoint needed beside a
// degraded transcript replay.
func (a *Assembler) ResumeContextSection(
	ctx context.Context,
	startup session.StartupPromptContext,
) (string, error) {
	if a == nil {
		return "", nil
	}
	return a.checkpointSummaryResumeSection(ctx, startup.Workspace, startup.SessionID)
}

// Assemble renders the dual-scope memory context ahead of the agent system prompt.
func (a *Assembler) Assemble(
	ctx context.Context,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	basePrompt := strings.TrimSpace(agent.Prompt)

	contextBlock, err := a.PromptStartupSection(ctx, session.StartupPromptContext{
		AgentName:   agent.Name,
		WorkspaceID: workspaceIDFromResolved(workspace),
		Workspace:   workspaceRootFromResolved(workspace),
		SessionType: session.SessionTypeUser,
	}, agent, workspace)
	if err != nil {
		return "", err
	}
	if basePrompt == "" {
		return contextBlock, nil
	}
	if contextBlock == "" {
		return basePrompt, nil
	}

	return contextBlock + "\n\n" + basePrompt, nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func workspaceIDFromResolved(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return ""
	}
	return strings.TrimSpace(workspace.ID)
}

func workspaceRootFromResolved(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return ""
	}
	return strings.TrimSpace(workspace.RootDir)
}

func firstAssemblerValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (a *Assembler) checkpointSummarySection(ctx context.Context, workspaceRoot string) (string, error) {
	if err := contextErr(ctx); err != nil {
		return "", err
	}
	root := strings.TrimSpace(workspaceRoot)
	if a == nil || a.store == nil || root == "" {
		return "", nil
	}
	body, _, err := loadCheckpointSummary(a.store.ForWorkspace(root))
	if err != nil {
		return "", err
	}
	return renderCheckpointSummarySection(body), nil
}

func (a *Assembler) checkpointSummaryResumeSection(
	ctx context.Context,
	workspaceRoot string,
	sessionID string,
) (string, error) {
	if err := contextErr(ctx); err != nil {
		return "", err
	}
	root := strings.TrimSpace(workspaceRoot)
	if a == nil || a.store == nil || root == "" {
		return "", nil
	}
	state, err := loadCheckpointSummaryState(a.store.ForWorkspace(root))
	if err != nil {
		return "", err
	}
	return renderCheckpointSummaryResumeSection(state, sessionID), nil
}

func joinAssemblerSections(sections ...string) string {
	nonEmpty := make([]string, 0, len(sections))
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}
