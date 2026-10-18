import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import {
  automationDetailStatus,
  automationDetailStatusMessage,
  useAutomationDetailActions,
  useAutomationDetailEntity,
  useAutomationDetailMutations,
  useAutomationDetailRuns,
  useAutomationEditIntent,
  useLoopTargetPresence,
  withQueuedRun,
} from "./use-automation-detail-parts";
import { automationUnavailableMessage } from "./use-automation-page-base";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  automationEditorWorkspaceId,
  automationLastRanAt,
  automationListingSearch,
  type AutomationDetailRouteSearch,
  type AutomationEditorSection,
  type AutomationEntityKind,
  type AutomationRun,
  toAutomationView,
  useAutomationEditor,
} from "@/systems/automation";
import { useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

/**
 * One detail view-model for both daemon entities. The route keeps the entity
 * (`/automations/jobs/:id` | `/automations/triggers/:id`) so every read and
 * mutation goes to one resource; the page itself renders one `AutomationView`.
 */
export function useAutomationDetailPage(
  kind: AutomationEntityKind,
  id: string,
  search: AutomationDetailRouteSearch = {}
) {
  const navigate = useNavigate();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const { activeWorkspaceId, isLoading: workspaceLoading, workspaces } = useActiveWorkspace();
  const [queuedRun, setQueuedRun] = useState<AutomationRun | null>(null);

  const { accessError, detailQuery, entity } = useAutomationDetailEntity(
    kind,
    id,
    liveDataEnabled,
    activeWorkspaceId,
    workspaceLoading
  );
  const runsQuery = useAutomationDetailRuns(kind, id, liveDataEnabled && Boolean(entity));

  const settingsQuery = useSettingsAutomation();
  const timeZone = settingsQuery.data?.config?.timezone?.trim() || undefined;
  const runtimeUnavailable = automationUnavailableMessage(
    settingsQuery.data?.runtime ?? null,
    detailQuery.error
  );

  const workspaceNameById = (workspaceId: string | undefined): string | null => {
    if (!workspaceId) return null;
    return workspaces.find(workspace => workspace.id === workspaceId)?.name ?? workspaceId;
  };
  const sentenceContext = {
    ...(timeZone ? { timeZone } : {}),
    workspaceName: workspaceNameById,
  };
  const view = entity ? toAutomationView(entity, sentenceContext) : undefined;
  const { loopTarget, loopMissing } = useLoopTargetPresence(entity, liveDataEnabled);

  const mutations = useAutomationDetailMutations(kind);
  const actions = useAutomationDetailActions({
    entity,
    kind,
    search,
    runtimeUnavailable: runtimeUnavailable !== null,
    mutations,
    onQueued: setQueuedRun,
  });

  const editor = useAutomationEditor({
    activeWorkspaceId: automationEditorWorkspaceId(entity, activeWorkspaceId),
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  });
  const openEdit = (section?: AutomationEditorSection) => {
    if (entity) editor.openEdit(entity, { section });
  };
  useAutomationEditIntent({ kind, id, search, ready: view?.canEdit === true, openEdit });

  const runs = withQueuedRun(
    entity ? (runsQuery.data ?? []) : [],
    kind === "job" ? queuedRun : null
  );
  const status = automationDetailStatus({
    accessError,
    entity,
    loading: detailQuery.isLoading || workspaceLoading,
    error: detailQuery.error,
  });

  return {
    editorDialogProps: editor.editorDialogProps,
    isJob: kind === "job",
    panel: {
      status,
      statusMessage: automationDetailStatusMessage(status, accessError, detailQuery.error),
      entity,
      view,
      sentenceContext,
      loopWorkspaceName: loopTarget ? workspaceNameById(loopTarget.workspaceId) : null,
      loopMissing,
      lastRanAt: automationLastRanAt(runs) ?? view?.lastRun?.startedAt ?? null,
      runs,
      runsError: entity ? runsQuery.error : null,
      runsLoading: entity ? runsQuery.isLoading : false,
      state: {
        isDeleting: mutations.remove.isPending,
        isTogglePending: mutations.update.isPending,
        isRunNowPending: mutations.runNow.isPending,
        isRunNowDisabled: runtimeUnavailable !== null,
      },
      onBack: () => void navigate({ to: "/automations", search: automationListingSearch(search) }),
      onDelete: actions.deleteEntity,
      onEdit: () => openEdit(),
      onRetryRuns: () => void runsQuery.refetch(),
      onRunNow: () => void actions.startNow(),
      // Opens in place: the `?edit=options` deep link is for arrivals, not a history entry.
      onSetUpRetries: () => openEdit("options"),
      onToggleEnabled: (enabled: boolean) => void actions.toggleEnabled(enabled),
    },
  };
}
