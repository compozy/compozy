import { useEffect, useMemo, useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import {
  WorkflowMemoryView,
  useWorkflowMemoryFile,
  useWorkflowMemoryIndex,
} from "@/systems/memory";

export const Route = createFileRoute("/_app/memory_/$slug")({
  component: WorkflowMemoryRoute,
});

function WorkflowMemoryRoute(): ReactElement {
  const { slug } = useParams({ from: "/_app/memory_/$slug" });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const indexQuery = useWorkflowMemoryIndex(activeWorkspace.id, slug);
  const [selectedFileId, setSelectedFileId] = useState<string | null>(null);

  const entries = useMemo(() => indexQuery.data?.entries ?? [], [indexQuery.data?.entries]);

  useEffect(() => {
    if (entries.length === 0) {
      setSelectedFileId(null);
      return;
    }
    setSelectedFileId(current => {
      if (current && entries.some(entry => entry.file_id === current)) {
        return current;
      }
      const shared = entries.find(entry => entry.kind.trim().toLowerCase() === "shared");
      const fallback = shared ?? entries[0];
      return fallback?.file_id ?? null;
    });
  }, [entries]);

  const fileQuery = useWorkflowMemoryFile(activeWorkspace.id, slug, selectedFileId);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs font-medium text-primary transition-colors hover:text-foreground"
            data-testid="workflow-memory-header-back"
            onClick={() => void navigate({ to: "/memory" })}
            type="button"
          >
            ← Back to memory
          </button>
          <span className="eyebrow text-muted-foreground">workflow memory</span>
        </div>
      }
    >
      {indexQuery.isLoading && !indexQuery.data ? (
        <div className="space-y-3" data-testid="workflow-memory-loading">
          <p className="sr-only">Loading memory index…</p>
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {indexQuery.isError && !indexQuery.data ? (
        <Alert data-testid="workflow-memory-load-error" variant="error">
          {apiErrorMessage(indexQuery.error, `Failed to load memory index for ${slug}`)}
        </Alert>
      ) : null}
      {indexQuery.data ? (
        <WorkflowMemoryView
          documentError={
            fileQuery.isError && selectedFileId
              ? apiErrorMessage(
                  fileQuery.error,
                  `Failed to load memory file ${selectedFileId} for ${slug}`
                )
              : null
          }
          index={indexQuery.data}
          isDocumentLoading={fileQuery.isLoading}
          isDocumentRefreshing={fileQuery.isRefetching}
          onSelectFileId={setSelectedFileId}
          selectedDocument={fileQuery.data}
          selectedFileId={selectedFileId}
        />
      ) : null}
    </AppShellLayout>
  );
}
