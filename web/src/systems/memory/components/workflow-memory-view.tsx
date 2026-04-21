import type { ReactElement } from "react";

import {
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  cn,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { MarkdownDocument, WorkflowMemoryEntry, WorkflowMemoryIndex } from "../types";

export interface WorkflowMemoryViewProps {
  index: WorkflowMemoryIndex;
  selectedFileId: string | null;
  selectedDocument: MarkdownDocument | undefined;
  onSelectFileId: (fileId: string) => void;
  isDocumentLoading: boolean;
  isDocumentRefreshing: boolean;
  documentError?: string | null;
}

export function WorkflowMemoryView(props: WorkflowMemoryViewProps): ReactElement {
  const {
    index,
    selectedFileId,
    selectedDocument,
    onSelectFileId,
    isDocumentLoading,
    isDocumentRefreshing,
    documentError,
  } = props;
  const { workflow, workspace, entries } = index;
  const safeEntries = entries ?? [];
  const shared = safeEntries.filter(entry => normalizeKind(entry.kind) === "shared");
  const notebooks = safeEntries.filter(entry => normalizeKind(entry.kind) !== "shared");

  return (
    <div className="space-y-6" data-testid="workflow-memory-view">
      <SectionHeading
        description={
          <span>
            <Link
              className="underline-offset-4 hover:underline"
              data-testid="workflow-memory-back"
              to="/memory"
            >
              Back to memory
            </Link>
            {" · "}
            {workspace.name} · {safeEntries.length} entr{safeEntries.length === 1 ? "y" : "ies"}
          </span>
        }
        eyebrow={`Memory · ${workflow.slug}`}
        title={workflow.slug}
      />

      {safeEntries.length === 0 ? (
        <SurfaceCard data-testid="workflow-memory-empty">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Empty</SurfaceCardEyebrow>
              <SurfaceCardTitle>No memory notebooks yet</SurfaceCardTitle>
              <SurfaceCardDescription>
                This workflow does not have any memory files on disk. Agents will write their first
                notebook after the next completed task.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
        </SurfaceCard>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside
            className="space-y-4 rounded-[var(--radius-md)] border border-border bg-black/10 p-3"
            data-testid="workflow-memory-sidebar"
          >
            {shared.length > 0 ? (
              <EntryGroup
                entries={shared}
                label="Shared"
                onSelectFileId={onSelectFileId}
                selectedFileId={selectedFileId}
                testId="workflow-memory-group-shared"
              />
            ) : null}
            {notebooks.length > 0 ? (
              <EntryGroup
                entries={notebooks}
                label="Per-task notebooks"
                onSelectFileId={onSelectFileId}
                selectedFileId={selectedFileId}
                testId="workflow-memory-group-task"
              />
            ) : null}
          </aside>

          <SurfaceCard data-testid="workflow-memory-document">
            {selectedFileId ? (
              <DocumentBody
                document={selectedDocument}
                isLoading={isDocumentLoading}
                isRefreshing={isDocumentRefreshing}
                error={documentError}
              />
            ) : (
              <SurfaceCardBody>
                <p
                  className="text-sm text-muted-foreground"
                  data-testid="workflow-memory-document-placeholder"
                >
                  Select a memory entry from the sidebar to view it.
                </p>
              </SurfaceCardBody>
            )}
          </SurfaceCard>
        </div>
      )}
    </div>
  );
}

function EntryGroup({
  entries,
  label,
  onSelectFileId,
  selectedFileId,
  testId,
}: {
  entries: WorkflowMemoryEntry[];
  label: string;
  onSelectFileId: (fileId: string) => void;
  selectedFileId: string | null;
  testId: string;
}): ReactElement {
  return (
    <div className="space-y-2" data-testid={testId}>
      <p className="eyebrow text-muted-foreground">{label}</p>
      <ul className="space-y-1">
        {entries.map(entry => (
          <li key={entry.file_id}>
            <button
              aria-pressed={selectedFileId === entry.file_id}
              className={cn(
                "flex w-full flex-col items-start gap-0.5 rounded-[var(--radius-sm)] px-2 py-1.5 text-left transition-colors",
                selectedFileId === entry.file_id
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "hover:bg-accent hover:text-foreground"
              )}
              data-testid={`workflow-memory-entry-${entry.file_id}`}
              onClick={() => onSelectFileId(entry.file_id)}
              title={entry.display_path}
              type="button"
            >
              <span className="truncate text-sm text-foreground">{entry.title}</span>
              <span className="truncate font-mono text-[10px] text-muted-foreground">
                {entry.display_path}
              </span>
              <span className="eyebrow text-muted-foreground">
                {formatBytes(entry.size_bytes)} · {formatTimestamp(entry.updated_at)}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function DocumentBody({
  document,
  isLoading,
  isRefreshing,
  error,
}: {
  document: MarkdownDocument | undefined;
  isLoading: boolean;
  isRefreshing: boolean;
  error?: string | null;
}): ReactElement {
  if (error) {
    return (
      <>
        <SurfaceCardHeader>
          <div>
            <SurfaceCardEyebrow>Error</SurfaceCardEyebrow>
            <SurfaceCardTitle>Memory document unavailable</SurfaceCardTitle>
            <SurfaceCardDescription>
              The daemon could not read this memory entry.
            </SurfaceCardDescription>
          </div>
          <StatusBadge tone="danger">error</StatusBadge>
        </SurfaceCardHeader>
        <SurfaceCardBody>
          <p
            className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-3 py-2 text-sm text-[color:var(--color-danger)]"
            data-testid="workflow-memory-document-error"
            role="alert"
          >
            {error}
          </p>
        </SurfaceCardBody>
      </>
    );
  }
  if (isLoading && !document) {
    return (
      <>
        <SurfaceCardHeader>
          <div>
            <SurfaceCardEyebrow>Loading</SurfaceCardEyebrow>
            <SurfaceCardTitle>Memory document…</SurfaceCardTitle>
            <SurfaceCardDescription>Fetching content from the daemon.</SurfaceCardDescription>
          </div>
        </SurfaceCardHeader>
        <SurfaceCardBody>
          <p
            className="text-sm text-muted-foreground"
            data-testid="workflow-memory-document-loading"
          >
            Loading…
          </p>
        </SurfaceCardBody>
      </>
    );
  }
  if (!document) {
    return (
      <SurfaceCardBody>
        <p
          className="text-sm text-muted-foreground"
          data-testid="workflow-memory-document-placeholder"
        >
          Select a memory entry from the sidebar to view it.
        </p>
      </SurfaceCardBody>
    );
  }
  const markdown = document.markdown?.trim() ?? "";
  return (
    <>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>{document.kind}</SurfaceCardEyebrow>
          <SurfaceCardTitle>{document.title}</SurfaceCardTitle>
          <SurfaceCardDescription>
            Updated {formatTimestamp(document.updated_at)}
          </SurfaceCardDescription>
        </div>
        {isRefreshing ? (
          <span
            className="text-xs text-muted-foreground"
            data-testid="workflow-memory-document-refreshing"
          >
            refreshing…
          </span>
        ) : null}
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {markdown.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="workflow-memory-document-empty">
            Document body is empty.
          </p>
        ) : (
          <pre
            className="max-h-[640px] overflow-auto whitespace-pre-wrap rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2 text-sm text-foreground"
            data-testid="workflow-memory-document-body"
          >
            {markdown}
          </pre>
        )}
      </SurfaceCardBody>
    </>
  );
}

function normalizeKind(kind: string): string {
  return kind.trim().toLowerCase();
}

function formatBytes(size: number): string {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
