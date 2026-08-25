package skills

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	catalogDescriptionLimit = 200
	catalogEllipsis         = "..."
	currentCatalogOpen      = "<current-available-skills>"
	currentCatalogClose     = "</current-available-skills>"
	// ManagedSkillLoadingInstructions keeps managed skill loading on canonical native tools.
	ManagedSkillLoadingInstructions = "Do not invoke `compozy skill view` or read skill files directly from a managed session. " +
		"If current tool policy denies canonical `compozy__skill_view`, report that the skill cannot be loaded; `compozy skill view` is an operator-shell command only."
	catalogUsageInstructions = "Resolve canonical `compozy__skill_view` through the active harness, then call the returned tool reference to load full instructions for any skill.\n" +
		"Use the returned tool reference for canonical `compozy__skill_view` to read a specific skill resource file when the skill references one.\n" +
		ManagedSkillLoadingInstructions
	currentCatalogInstructions = "" +
		"The <current-available-skills> block above is the authoritative current skill state for this turn.\n" +
		"If it differs from any earlier <available-skills> startup snapshot, trust the current block."
	currentCatalogUnchangedInstructions = "" +
		"Previous catalog remains current; resolve canonical `compozy__skill_view` for full skill/resource instructions."
)

var (
	catalogTextReplacer = strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	catalogAttrReplacer = strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
)

// CatalogProvider builds the workspace-scoped skill catalog section expected by
// the composed prompt assembly pipeline.
type CatalogProvider struct {
	registry *Registry
}

// NewCatalogProvider constructs a CatalogProvider backed by the provided registry.
func NewCatalogProvider(registry *Registry) *CatalogProvider {
	return &CatalogProvider{registry: registry}
}

// PromptSection loads the workspace-scoped skills and returns their XML-like
// catalog representation.
func (cp *CatalogProvider) PromptSection(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if cp == nil || cp.registry == nil {
		return "", nil
	}

	skills, err := cp.registry.ForWorkspace(ctx, workspace)
	if err != nil {
		return "", fmt.Errorf("skills: build catalog for workspace %q: %w", catalogWorkspaceLabel(workspace), err)
	}

	return BuildCatalog(skills), nil
}

// PromptStartupSection resolves the effective catalog for the concrete agent
// being started when the prompt assembler provides that identity.
func (cp *CatalogProvider) PromptAgentSection(
	ctx context.Context,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if cp == nil || cp.registry == nil {
		return "", nil
	}

	skills, err := cp.registry.ForAgentDef(ctx, workspace, agent)
	if err != nil {
		return "", fmt.Errorf("skills: build catalog for agent %q: %w", agent.Name, err)
	}
	return BuildCatalog(skills), nil
}

// PromptAgentSessionSection resolves activation gates against the concrete startup session.
func (cp *CatalogProvider) PromptAgentSessionSection(
	ctx context.Context,
	sessionID string,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if cp == nil || cp.registry == nil {
		return "", nil
	}

	skills, err := cp.registry.ForAgentDefSession(ctx, workspace, agent, sessionID)
	if err != nil {
		return "", fmt.Errorf("skills: build catalog for agent %q session %q: %w", agent.Name, sessionID, err)
	}
	return BuildCatalog(skills), nil
}

// PromptAgentSessionFilteredSection applies one session-resolved injection-only filter.
func (cp *CatalogProvider) PromptAgentSessionFilteredSection(
	ctx context.Context,
	sessionID string,
	agent compozyconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
	filter func(*Skill) bool,
) (string, error) {
	if cp == nil || cp.registry == nil {
		return "", nil
	}
	skills, err := cp.registry.ForAgentDefSession(ctx, workspace, agent, sessionID)
	if err != nil {
		return "", fmt.Errorf("skills: build filtered catalog for agent %q session %q: %w", agent.Name, sessionID, err)
	}
	return BuildCatalog(filterInjectedSkills(skills, filter)), nil
}

func filterInjectedSkills(skills []*Skill, filter func(*Skill) bool) []*Skill {
	if filter == nil {
		return skills
	}
	filtered := make([]*Skill, 0, len(skills))
	for _, skill := range skills {
		if skill != nil && filter(skill) {
			filtered = append(filtered, skill)
		}
	}
	return filtered
}

// BuildCatalog renders the XML-like available-skills block injected into agent
// system prompts.
func BuildCatalog(skills []*Skill) string {
	return buildCatalog(skills, "<available-skills>", "</available-skills>", catalogUsageInstructions)
}

// BuildCurrentCatalog renders the authoritative per-turn skills block injected
// ahead of live prompts.
func BuildCurrentCatalog(skills []*Skill) string {
	return buildCatalog(
		skills,
		currentCatalogOpen,
		currentCatalogClose,
		currentCatalogInstructions+"\n"+catalogUsageInstructions,
	)
}

// BuildCurrentCatalogUnchanged renders the compact per-turn marker used when
// the catalog did not change.
func BuildCurrentCatalogUnchanged() string {
	return strings.Join([]string{
		currentCatalogOpen,
		"  <catalog-state unchanged=\"true\">" + currentCatalogUnchangedInstructions + "</catalog-state>",
		currentCatalogClose,
		"",
		ManagedSkillLoadingInstructions,
	}, "\n")
}

func buildCatalog(skills []*Skill, openTag string, closeTag string, instructions string) string {
	type catalogEntry struct {
		name        string
		description string
	}

	entries := make([]catalogEntry, 0, len(skills))
	for _, skill := range skills {
		if skill == nil || !skill.Enabled || !skillIsActive(skill) {
			continue
		}

		name := strings.TrimSpace(skill.Meta.Name)
		if name == "" {
			continue
		}

		entries = append(entries, catalogEntry{
			name:        name,
			description: truncateCatalogDescription(skill.Meta.Description),
		})
	}

	if len(entries) == 0 {
		return ""
	}

	slices.SortFunc(entries, func(left, right catalogEntry) int {
		return strings.Compare(left.name, right.name)
	})

	var builder strings.Builder
	builder.Grow(len(entries) * 64)
	builder.WriteString(openTag)
	builder.WriteString("\n")
	for _, entry := range entries {
		builder.WriteString(`  <skill name="`)
		builder.WriteString(escapeCatalogAttr(entry.name))
		builder.WriteString(`">`)
		builder.WriteString(escapeCatalogText(entry.description))
		builder.WriteString("</skill>\n")
	}
	builder.WriteString(closeTag)
	builder.WriteString("\n\n")
	builder.WriteString(instructions)

	return builder.String()
}

func truncateCatalogDescription(description string) string {
	if utf8.RuneCountInString(description) <= catalogDescriptionLimit {
		return description
	}

	truncationLimit := catalogDescriptionLimit - len(catalogEllipsis)
	runeCount := 0
	for idx := range description {
		if runeCount == truncationLimit {
			return description[:idx] + catalogEllipsis
		}
		runeCount++
	}

	return description
}

func escapeCatalogText(value string) string {
	return catalogTextReplacer.Replace(value)
}

func escapeCatalogAttr(value string) string {
	return catalogAttrReplacer.Replace(value)
}

func catalogWorkspaceLabel(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return "<global>"
	}
	if name := strings.TrimSpace(workspace.Name); name != "" {
		return name
	}
	if root := strings.TrimSpace(workspace.RootDir); root != "" {
		return root
	}
	if id := strings.TrimSpace(workspace.ID); id != "" {
		return id
	}
	return "<global>"
}
