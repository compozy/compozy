package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/spf13/cobra"
)

func addMemorySelectorFlags(cmd *cobra.Command, flags *memorySelectorFlags) {
	cmd.Flags().StringVar(&flags.Scope, automationScopeKey, "", "Memory scope: global, workspace, or agent")
	cmd.Flags().StringVar(&flags.Workspace, "workspace", "", "Workspace ID or path for workspace-bound memory")
	cmd.Flags().StringVar(&flags.Agent, "agent", "", "Agent name for agent-scoped memory")
	cmd.Flags().StringVar(&flags.AgentTier, "agent-tier", "", "Agent memory tier: workspace or global")
}

func resolveMemorySelectorFlags(
	deps commandDeps,
	flags memorySelectorFlags,
	opts memorySelectorOptions,
) (MemorySelectorQuery, error) {
	scope, err := parseOptionalCLIMemoryScope(flags.Scope)
	if err != nil {
		return MemorySelectorQuery{}, err
	}
	agent := strings.TrimSpace(flags.Agent)
	tier, err := parseOptionalCLIAgentTier(flags.AgentTier)
	if err != nil {
		return MemorySelectorQuery{}, err
	}
	if scope == "" && (agent != "" || tier != "") {
		scope = memcontract.ScopeAgent
	}
	if scope == "" {
		scope = opts.DefaultScope
	}
	if scope != memcontract.ScopeAgent && (agent != "" || tier != "") {
		return MemorySelectorQuery{}, errors.New(
			"memory.scope.agent_flags_invalid: --agent and --agent-tier require --scope agent",
		)
	}
	if scope == memcontract.ScopeAgent {
		if agent == "" {
			return MemorySelectorQuery{}, errors.New(
				"memory.scope.agent_required: --agent is required when --scope agent",
			)
		}
		if tier == "" {
			return MemorySelectorQuery{}, errors.New(
				"memory.scope.agent_tier_required: --agent-tier is required when --scope agent",
			)
		}
	}

	workspace := strings.TrimSpace(flags.Workspace)
	needsWorkspace := scope == memcontract.ScopeWorkspace || (scope == "" && opts.DefaultWorkspace) ||
		(scope == memcontract.ScopeAgent && tier == memcontract.AgentTierWorkspace)
	if workspace == "" && needsWorkspace {
		var err error
		workspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return MemorySelectorQuery{}, err
		}
	}
	return MemorySelectorQuery{
		Scope:       scope,
		WorkspaceID: workspace,
		AgentName:   agent,
		AgentTier:   tier,
	}, nil
}

func parseOptionalCLIMemoryScope(raw string) (memcontract.Scope, error) {
	scope := memcontract.Scope(strings.TrimSpace(raw)).Normalize()
	switch scope {
	case "":
		return "", nil
	case memcontract.ScopeGlobal, memcontract.ScopeWorkspace, memcontract.ScopeAgent:
		return scope, nil
	default:
		return "", errors.New("memory.scope.invalid: scope must be one of global, workspace, or agent")
	}
}

func parseOptionalCLIAgentTier(raw string) (memcontract.AgentTier, error) {
	tier := memcontract.AgentTier(strings.TrimSpace(raw)).Normalize()
	switch tier {
	case "":
		return "", nil
	case memcontract.AgentTierWorkspace, memcontract.AgentTierGlobal:
		return tier, nil
	default:
		return "", errors.New("memory.scope.agent_tier_invalid: agent-tier must be one of workspace or global")
	}
}

func parseRequiredMemoryType(raw string) (memcontract.Type, error) {
	typ, err := parseOptionalMemoryType(raw)
	if err != nil {
		return "", err
	}
	if typ == "" {
		return "", errors.New("memory.type_required: --type is required")
	}
	return typ, nil
}

func parseOptionalMemoryType(raw string) (memcontract.Type, error) {
	typ := memcontract.Type(strings.TrimSpace(raw)).Normalize()
	if typ == "" {
		return "", nil
	}
	if err := typ.Validate(); err != nil {
		return "", err
	}
	return typ, nil
}

func resolveMemoryContent(cmd *cobra.Command, deps commandDeps, raw string) (string, error) {
	flag := cmd.Flags().Lookup(memoryContentKey)
	flagChanged := flag != nil && flag.Changed
	stdinContent, err := readOptionalCommandInput(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	if flagChanged && strings.TrimSpace(stdinContent) != "" && strings.TrimSpace(raw) != "-" {
		return "", errors.New("memory.content_conflict: provide memory content via --content or stdin, not both")
	}
	if flagChanged {
		if strings.TrimSpace(raw) == "-" {
			if strings.TrimSpace(stdinContent) == "" {
				return "", errors.New("memory.content_required: stdin content is required")
			}
			return stdinContent, nil
		}
		return resolveMemoryContentValue(deps, raw, cmd.InOrStdin())
	}
	if strings.TrimSpace(stdinContent) != "" {
		return stdinContent, nil
	}
	return "", errors.New("memory.content_required: content is required via --content or stdin")
}

func resolveMemoryContentValue(deps commandDeps, raw string, stdin io.Reader) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("memory.content_required: content is required")
	}
	if trimmed == "-" {
		content, err := readOptionalCommandInput(stdin)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(content) == "" {
			return "", errors.New("memory.content_required: stdin content is required")
		}
		return content, nil
	}
	if after, ok := strings.CutPrefix(trimmed, "@"); ok {
		path := strings.TrimSpace(after)
		if path == "" {
			return "", errors.New("memory.content_path_required: @ content path is required")
		}
		cleaned := filepath.Clean(path)
		if !filepath.IsAbs(cleaned) && deps.getwd != nil {
			wd, err := currentWorkingDirectory(deps)
			if err != nil {
				return "", err
			}
			cleaned = filepath.Join(wd, cleaned)
		}
		data, err := os.ReadFile(cleaned)
		if err != nil {
			return "", fmt.Errorf("memory.content_read_failed: read %s: %w", cleaned, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return "", errors.New("memory.content_required: file content is required")
		}
		return string(data), nil
	}
	return raw, nil
}

func readOptionalCommandInput(reader io.Reader) (string, error) {
	if reader == nil {
		return "", nil
	}
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", fmt.Errorf("cli: stat stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", nil
		}
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("cli: read stdin: %w", err)
	}
	return string(data), nil
}

func parseMemoryPromotionSelector(
	deps commandDeps,
	flags memorySelectorFlags,
	raw string,
) (contract.MemoryScopeSelectorPayload, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
		return contract.MemoryScopeSelectorPayload{}, errors.New(
			"memory.promote.selector_invalid: selector must be scope[:tier]",
		)
	}
	scope, err := parseOptionalCLIMemoryScope(parts[0])
	if err != nil {
		return contract.MemoryScopeSelectorPayload{}, err
	}
	if scope == "" {
		return contract.MemoryScopeSelectorPayload{}, errors.New(
			"memory.promote.selector_invalid: selector scope is required",
		)
	}
	promoteFlags := flags
	promoteFlags.Scope = string(scope)
	if len(parts) == 2 {
		promoteFlags.AgentTier = strings.TrimSpace(parts[1])
	}
	if scope != memcontract.ScopeAgent {
		promoteFlags.Agent = ""
		promoteFlags.AgentTier = ""
	}
	selector, err := resolveMemorySelectorFlags(deps, promoteFlags, memorySelectorOptions{DefaultWorkspace: true})
	if err != nil {
		return contract.MemoryScopeSelectorPayload{}, err
	}
	return contract.MemoryScopeSelectorPayload{
		Scope:       selector.Scope,
		WorkspaceID: selector.WorkspaceID,
		AgentName:   selector.AgentName,
		AgentTier:   selector.AgentTier,
	}, nil
}
