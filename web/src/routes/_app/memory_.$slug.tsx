import { useEffect, useMemo, useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

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
            className="text-xs text-accent hover:underline"
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
        <p className="text-sm text-muted-foreground" data-testid="workflow-memory-loading">
          Loading memory index…
        </p>
      ) : null}
      {indexQuery.isError && !indexQuery.data ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="workflow-memory-load-error"
          role="alert"
        >
          {apiErrorMessage(indexQuery.error, `Failed to load memory index for ${slug}`)}
        </p>
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
