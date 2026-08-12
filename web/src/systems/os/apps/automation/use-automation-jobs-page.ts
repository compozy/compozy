import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import {
  automationListError,
  automationUnavailableMessage,
  useAutomationCreateSeed,
  useAutomationPageBase,
  type AutomationCreateSeed,
  type AutomationRouteSearch,
} from "./use-automation-page-base";
import { automationJobRunLogic } from "./automation-job-run-store";
import {
  useAutomationJobEditor,
  useAutomationJobs,
  useTriggerAutomationJob,
} from "@/systems/automation";

export function useAutomationJobsPage(
  seed: AutomationCreateSeed = {},
  search: AutomationRouteSearch = {}
) {
  const page = useAutomationPageBase("jobs", search);
  const navigate = useNavigate();
  const runStore = useStore(automationJobRunLogic);
  const runPendingIds = useSelector(runStore, snapshot => snapshot.context.pendingIds);

  const jobsQuery = useAutomationJobs(page.listFilters);
  const jobs = jobsQuery.jobs;
  const runtimeUnavailableMessage = automationUnavailableMessage(
    "jobs",
    page.automationRuntime,
    jobsQuery.error
  );
  const runDisabled = runtimeUnavailableMessage !== null;

  const editor = useAutomationJobEditor({
    activeWorkspaceId: page.activeWorkspaceId,
    userHomeDir: page.userHomeDir,
    workspaces: page.workspaces,
    onSaved: job => void navigate({ to: "/jobs/$jobId", params: { jobId: job.id } }),
  });

  useAutomationCreateSeed("jobs", seed, page.activeWorkspaceId, editor.openLoopCreate);

  const triggerJobMutation = useTriggerAutomationJob();
  const onRunJob = (id: string) => {
    runStore.trigger.runRequested({
      execute: jobId => triggerJobMutation.mutateAsync({ id: jobId }),
      id,
      onFailure: error =>
        toast.error(error instanceof Error ? error.message : "Failed to trigger automation job"),
      onSuccess: run => toast.success(`Queued run ${run.id}.`),
      permitted: !runDisabled,
    });
  };

  return {
    clearFilters: page.clearFilters,
    editorDialogProps: editor.editorDialogProps,
    enabledFilter: page.enabledFilter,
    error: automationListError(runtimeUnavailableMessage, jobsQuery.error, jobs.length),
    errorMessage: runtimeUnavailableMessage ?? jobsQuery.error?.message ?? null,
    handleCreate: editor.openCreate,
    hasActiveFilters: page.hasActiveFilters,
    hasNextPage: jobsQuery.hasNextPage,
    isFetchingNextPage: jobsQuery.isFetchingNextPage,
    isLoading: jobsQuery.isLoading && jobs.length === 0,
    jobs,
    loadMore: () => void jobsQuery.fetchNextPage(),
    onRunJob,
    runDisabled,
    runPendingIds,
    runtimeUnavailableMessage,
    scopeFilter: page.scopeFilter,
    searchQuery: page.searchQuery,
    setEnabledFilter: page.setEnabledFilter,
    setScopeFilter: page.setScopeFilter,
    setSearchQuery: page.setSearchQuery,
    setSourceFilter: page.setSourceFilter,
    setView: page.setView,
    sourceFilter: page.sourceFilter,
    total: jobsQuery.total,
    view: page.view,
  };
}
