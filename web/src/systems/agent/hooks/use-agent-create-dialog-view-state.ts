import { useState } from "react";

import type { EntityMode } from "@agh/ui";

import type { RuntimeProviderOption } from "@/systems/runtime";

import { validateAgentCreateDraft, type AgentCreateDialogDraft } from "../lib/agent-create-draft";

interface AgentCreateDialogViewStateArgs {
  draft: AgentCreateDialogDraft;
  hasActiveWorkspace: boolean;
  initialMode: EntityMode;
  onOpenChange: (open: boolean) => void;
  open: boolean;
  providerOptions: RuntimeProviderOption[];
  providersError: string | null;
  providersLoading: boolean;
}

function useAgentCreateDialogViewState({
  draft,
  hasActiveWorkspace,
  initialMode,
  onOpenChange,
  providerOptions,
  providersError,
  providersLoading,
}: AgentCreateDialogViewStateArgs) {
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const validation = validateAgentCreateDraft(draft, {
    hasActiveWorkspace,
    providerOptions,
    providersError,
    providersLoading,
  });
  const visibleErrors = visibleAgentCreateErrors(draft, validation.fields, {
    providerOptions,
    providersError,
    providersLoading,
  });

  const activeProvider = providerOptions.find(option => option.id === draft.provider);

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setMode(initialMode);
    }
    onOpenChange(next);
  };

  /**
   * Advanced holds no unsupported selection to snap back — every advanced field
   * is a real contract field — so leaving Advanced preserves the draft. The one
   * thing that must not happen is a hidden invalid field silently blocking
   * submit, so the dialog reveals Advanced instead of discarding the value.
   */
  const revealAdvancedWhenBlocked = () => {
    if (!validation.advancedValid) {
      setMode("advanced");
      return true;
    }
    return false;
  };

  return {
    activeProvider,
    handleOpenChange,
    mode,
    revealAdvancedWhenBlocked,
    setMode,
    validation,
    visibleErrors,
  };
}

function visibleAgentCreateErrors(
  draft: AgentCreateDialogDraft,
  errors: Record<string, string | undefined>,
  context: {
    providerOptions: readonly RuntimeProviderOption[];
    providersError: string | null;
    providersLoading: boolean;
  }
): Record<string, string | undefined> {
  return {
    scope: errors.scope,
    name: draft.name.trim().length > 0 ? errors.name : undefined,
    categoryPath: draft.categoryPath.trim().length > 0 ? errors.categoryPath : undefined,
    provider:
      context.providersError ||
      context.providersLoading ||
      context.providerOptions.length === 0 ||
      draft.provider.trim().length > 0
        ? errors.provider
        : undefined,
    reasoningEffort: draft.reasoningEffort !== "" ? errors.reasoningEffort : undefined,
    prompt: draft.prompt.trim().length > 0 ? errors.prompt : undefined,
    tools: draft.tools.length > 0 ? errors.tools : undefined,
    toolsets: draft.toolsets.length > 0 ? errors.toolsets : undefined,
    denyTools: draft.denyTools.length > 0 ? errors.denyTools : undefined,
  };
}

export { useAgentCreateDialogViewState };
