import { useNavigate } from "@tanstack/react-router";

import { useAutomationTriggerEditor, useAutomationTriggers } from "@/systems/automation";

import {
  automationListError,
  automationUnavailableMessage,
  useAutomationCreateSeed,
  useAutomationPageBase,
  type AutomationCreateSeed,
  type AutomationRouteSearch,
} from "./use-automation-page-base";

export function useAutomationTriggersPage(
  seed: AutomationCreateSeed = {},
  search: AutomationRouteSearch = {}
) {
  const page = useAutomationPageBase("triggers", search);
  const navigate = useNavigate();

  const triggersQuery = useAutomationTriggers({ ...page.listFilters, event: search.event });
  const triggers = triggersQuery.triggers;
  const runtimeUnavailableMessage = automationUnavailableMessage(
    "triggers",
    page.automationRuntime,
    triggersQuery.error
  );

  const editor = useAutomationTriggerEditor({
    activeWorkspaceId: page.activeWorkspaceId,
    workspaces: page.workspaces,
    onSaved: trigger =>
      void navigate({ to: "/triggers/$triggerId", params: { triggerId: trigger.id } }),
  });

  useAutomationCreateSeed("triggers", seed, page.activeWorkspaceId, editor.openLoopCreate);

  return {
    clearFilters: page.clearFilters,
    editorDialogProps: editor.editorDialogProps,
    enabledFilter: page.enabledFilter,
    error: automationListError(runtimeUnavailableMessage, triggersQuery.error, triggers.length),
    errorMessage: runtimeUnavailableMessage ?? triggersQuery.error?.message ?? null,
    eventFilter: page.eventFilter,
    handleCreate: editor.openCreate,
    hasActiveFilters: page.hasActiveFilters,
    hasNextPage: triggersQuery.hasNextPage,
    isFetchingNextPage: triggersQuery.isFetchingNextPage,
    isLoading: triggersQuery.isLoading && triggers.length === 0,
    loadMore: () => void triggersQuery.fetchNextPage(),
    runtimeUnavailableMessage,
    scopeFilter: page.scopeFilter,
    searchQuery: page.searchQuery,
    setEnabledFilter: page.setEnabledFilter,
    setEventFilter: page.setEventFilter,
    setScopeFilter: page.setScopeFilter,
    setSearchQuery: page.setSearchQuery,
    setSourceFilter: page.setSourceFilter,
    setView: page.setView,
    sourceFilter: page.sourceFilter,
    total: triggersQuery.total,
    triggers,
    view: page.view,
  };
}
