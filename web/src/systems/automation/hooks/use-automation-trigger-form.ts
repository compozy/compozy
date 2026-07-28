import type { FormEvent } from "react";

import {
  loopTargetAvailabilityMessage,
  useLoopTargetCatalog,
  type LoopAutomationStartKind,
  type LoopTargetDraft,
} from "@/systems/loops";
import {
  isNetworkParticipationDraftValid,
  networkParticipationDraftFromPayload,
} from "@/systems/network";

import {
  automationTargetMode,
  bindLoopTargetWorkspace,
  retryDraftForStrategy,
  setTriggerTargetMode,
  loopTargetWorkspaceId,
  type AutomationTargetMode,
} from "../lib/automation-drafts";
import {
  availableDataFields,
  ENVELOPE_KEYS,
  filterKeyOptions,
  getEventDef,
} from "../lib/trigger-catalog";
import { composeEventId, formatEventKind, parseEventSelection } from "../lib/trigger-event-id";
import {
  buildTriggerPreview,
  retainValidFilters,
  type WorkspaceOption,
} from "../lib/trigger-preview";
import { buildVariableChips } from "../lib/trigger-template";
import type {
  AutomationFireLimit,
  AutomationRetry,
  AutomationScope,
  AutomationTriggerFilter,
  CreateAutomationTriggerRequest,
} from "../types";
import type { SubConfigValues } from "../lib/trigger-sub-config";

export interface UseAutomationTriggerFormParams {
  activeWorkspaceId?: string | null;
  draft: CreateAutomationTriggerRequest;
  isPending: boolean;
  mode: "create" | "edit";
  onChange: (draft: CreateAutomationTriggerRequest) => void;
  onSubmit: () => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

function computeCanSubmit(
  draft: CreateAutomationTriggerRequest,
  selection: ReturnType<typeof parseEventSelection>,
  mode: "create" | "edit",
  loopTargetCompatible: boolean
): boolean {
  const loopWorkspaceId = draft.loop_target?.workspace_id ?? "";
  const loopWorkspaceValid =
    loopWorkspaceId !== "" &&
    (draft.scope === "global" || loopWorkspaceId === (draft.workspace_id ?? ""));
  const targetValid =
    automationTargetMode(draft) === "loop"
      ? Boolean(draft.loop_target?.loop_name.trim()) &&
        loopTargetCompatible &&
        loopWorkspaceValid &&
        isNetworkParticipationDraftValid(
          networkParticipationDraftFromPayload(draft.loop_target?.network_participation),
          ["named", "loop_run"]
        )
      : draft.agent_name.trim() !== "" && draft.prompt.trim() !== "";
  const baseValid =
    draft.name.trim() !== "" &&
    targetValid &&
    draft.event.trim() !== "" &&
    (draft.scope === "global" || Boolean(draft.workspace_id));
  if (!baseValid) return false;

  if (selection.family === "hook") return selection.hookName.trim() !== "";
  if (selection.family === "ext") {
    return selection.extExt.trim() !== "" && selection.extEvent.trim() !== "";
  }
  if (selection.family === "webhook") {
    const hasIds = Boolean(draft.endpoint_slug?.trim()) && Boolean(draft.webhook_id?.trim());
    return mode === "create" ? hasIds && Boolean(draft.webhook_secret_value?.trim()) : hasIds;
  }
  return true;
}

/** View-model for the trigger form: derives preview/options and owns every patch handler. */
export function useAutomationTriggerForm({
  activeWorkspaceId,
  draft,
  isPending,
  mode,
  onChange,
  onSubmit,
  workspaces,
}: UseAutomationTriggerFormParams) {
  const selection = parseEventSelection(draft.event);
  const def = getEventDef(selection.catalogId);
  const isWebhook = selection.family === "webhook";
  const disabledCatalogIds =
    mode === "edit" && draft.scope === "workspace" ? new Set(["webhook"]) : undefined;
  const retry = retryDraftForStrategy(draft.retry?.strategy ?? "none", draft.retry ?? undefined);
  const eventKind = formatEventKind(selection, draft.event);

  const resolvedWorkspaces: WorkspaceOption[] =
    workspaces && workspaces.length > 0
      ? [...workspaces]
      : activeWorkspaceId
        ? [{ id: activeWorkspaceId, name: activeWorkspaceId }]
        : [];

  const targetMode = automationTargetMode(draft);
  const loopTarget: LoopTargetDraft = draft.loop_target ?? {
    loop_name: "",
    inputs: {},
    input_mapping: {},
  };
  const loopWorkspaceId = loopTargetWorkspaceId(draft, activeWorkspaceId);
  const requiredStartKind: LoopAutomationStartKind = isWebhook ? "webhook" : "trigger";
  const loopCatalog = useLoopTargetCatalog(
    loopWorkspaceId,
    loopTarget.loop_name,
    requiredStartKind
  );
  const loopTargetIssue = loopTargetAvailabilityMessage(loopCatalog, mode);
  const preview = buildTriggerPreview(draft, {
    mode,
    targetIssue: loopTargetIssue,
    workspaces: resolvedWorkspaces,
  });

  const dataFields = def ? availableDataFields(def) : [];
  const subConfigValues: SubConfigValues = {
    hookName: selection.hookName,
    extExt: selection.extExt,
    extEvent: selection.extEvent,
    endpointSlug: draft.endpoint_slug ?? "",
    webhookId: draft.webhook_id ?? "",
    webhookSecret: draft.webhook_secret_value ?? "",
  };

  const patch = (next: Partial<CreateAutomationTriggerRequest>) => onChange({ ...draft, ...next });

  const handleLoopTargetChange = (next: LoopTargetDraft) => {
    patch({ loop_target: { ...next, workspace_id: loopWorkspaceId } });
  };

  const handleScopeChange = (scope: AutomationScope) => {
    if (scope === "global") {
      patch({ scope: "global", workspace_id: undefined });
      return;
    }
    const fallback = draft.workspace_id ?? activeWorkspaceId ?? resolvedWorkspaces[0]?.id;
    const workspaceId = fallback ?? "";
    onChange(
      bindLoopTargetWorkspace(
        { ...draft, scope: "workspace", workspace_id: workspaceId || undefined },
        workspaceId
      )
    );
  };

  const handleSelectEvent = (catalogId: string) => {
    if (disabledCatalogIds?.has(catalogId)) return;
    const nextDef = getEventDef(catalogId);
    if (!nextDef) return;
    const event = composeEventId({
      family: nextDef.family,
      catalogId,
      hookName: selection.hookName,
      extExt: selection.extExt,
      extEvent: selection.extEvent,
    });
    const next: CreateAutomationTriggerRequest = {
      ...draft,
      event,
      filter: retainValidFilters(draft.filter ?? {}, nextDef),
    };
    if (nextDef.family === "webhook") {
      next.scope = "global";
      next.workspace_id = undefined;
    } else {
      next.endpoint_slug = undefined;
      next.webhook_id = undefined;
      next.webhook_secret_value = undefined;
    }
    onChange(next);
  };

  const handleSubConfigChange = (subPatch: Partial<SubConfigValues>) => {
    const next: CreateAutomationTriggerRequest = { ...draft };
    if (subPatch.hookName !== undefined) {
      next.event = composeEventId({ family: "hook", hookName: subPatch.hookName });
    }
    if (subPatch.extExt !== undefined || subPatch.extEvent !== undefined) {
      next.event = composeEventId({
        family: "ext",
        extExt: subPatch.extExt ?? selection.extExt,
        extEvent: subPatch.extEvent ?? selection.extEvent,
      });
    }
    if (subPatch.endpointSlug !== undefined) next.endpoint_slug = subPatch.endpointSlug;
    if (subPatch.webhookId !== undefined) next.webhook_id = subPatch.webhookId;
    if (subPatch.webhookSecret !== undefined) next.webhook_secret_value = subPatch.webhookSecret;
    onChange(next);
  };

  const canSubmit = computeCanSubmit(draft, selection, mode, loopCatalog.status === "compatible");

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit || isPending) return;
    onSubmit();
  };

  return {
    disabledCatalogIds,
    selection,
    isWebhook,
    retry,
    eventKind,
    resolvedWorkspaces,
    preview,
    variables: buildVariableChips(dataFields),
    keyOptions: def ? filterKeyOptions(def) : [...ENVELOPE_KEYS],
    openPayload: def?.openPayload ?? true,
    hasActiveFilters: Object.keys(draft.filter ?? {}).some(key => key.trim() !== ""),
    subConfigValues,
    canSubmit,
    reliabilityDefaultOpen:
      mode === "edit" || retry.strategy === "backoff" || draft.enabled === false,
    onName: (name: string) => patch({ name }),
    onScopeChange: handleScopeChange,
    onWorkspaceChange: (workspace_id: string) =>
      onChange(bindLoopTargetWorkspace({ ...draft, workspace_id }, workspace_id)),
    onSelectEvent: handleSelectEvent,
    onSubConfigChange: handleSubConfigChange,
    onFilterChange: (filter: AutomationTriggerFilter) => patch({ filter }),
    onAgentChange: (agent_name: string) => patch({ agent_name }),
    onPromptChange: (prompt: string) => patch({ prompt }),
    targetMode,
    loopCatalog,
    loopTarget,
    onTargetModeChange: (mode: AutomationTargetMode) =>
      onChange(setTriggerTargetMode(draft, mode, loopWorkspaceId)),
    onLoopTargetChange: handleLoopTargetChange,
    onRetryChange: (next: AutomationRetry) => patch({ retry: next }),
    onFireLimitChange: (next: AutomationFireLimit) => patch({ fire_limit: next }),
    onEnabledChange: (enabled: boolean) => patch({ enabled }),
    handleSubmit,
  };
}
