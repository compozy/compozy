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
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import { resolveStatusTone } from "./task-board-view";
import { resolveStatusTone as resolveRunStatusTone } from "@/systems/runs";

import type {
  MarkdownDocument,
  TaskDetailPayload,
  TaskRelatedRun,
  WorkflowMemoryEntry,
} from "../types";

export interface TaskDetailViewProps {
  payload: TaskDetailPayload;
  isRefreshing: boolean;
}

export function TaskDetailView(props: TaskDetailViewProps): ReactElement {
  const { payload, isRefreshing } = props;
  const { task, workflow, document, memory_entries, related_runs, live_tail_available } = payload;
  const tone = resolveStatusTone(task.status);
  const deps = task.depends_on ?? [];
  const memory = memory_entries ?? [];
  const runs = related_runs ?? [];

  return (
    <div className="space-y-6" data-testid="task-detail-view">
      <SectionHeading
        description={
          <span>
            <Link
              className="underline-offset-4 hover:underline"
              data-testid="task-detail-back-to-board"
              params={{ slug: workflow.slug }}
              to="/workflows/$slug/tasks"
            >
              Back to {workflow.slug} board
            </Link>
            {" · "}
            {task.type} · updated {formatTimestamp(task.updated_at)}
            {live_tail_available ? " · live tail available" : " · live tail unavailable"}
          </span>
        }
        eyebrow={`Task #${task.task_number} · ${task.task_id}`}
        title={
          <span className="flex items-center gap-3">
            <span>{task.title}</span>
            <StatusBadge data-testid="task-detail-status" tone={tone}>
              {task.status}
            </StatusBadge>
          </span>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)]">
        <DocumentCard document={document} />
        <aside className="space-y-4" data-testid="task-detail-sidebar">
          <DependenciesCard deps={deps} />
          <RelatedRunsCard runs={runs} />
          <MemoryCard entries={memory} slug={workflow.slug} />
        </aside>
      </div>

      {isRefreshing ? (
        <p className="text-xs text-muted-foreground" data-testid="task-detail-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function DocumentCard({ document }: { document: MarkdownDocument }): ReactElement {
  const markdown = document.markdown?.trim() ?? "";
  return (
    <SurfaceCard data-testid="task-detail-document">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>{document.kind}</SurfaceCardEyebrow>
          <SurfaceCardTitle>{document.title}</SurfaceCardTitle>
          <SurfaceCardDescription>
            Updated {formatTimestamp(document.updated_at)}
          </SurfaceCardDescription>
        </div>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {markdown.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="task-detail-document-empty">
            Document body is empty.
          </p>
        ) : (
          <pre
            className="max-h-[480px] overflow-auto whitespace-pre-wrap rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2 text-sm text-foreground"
            data-testid="task-detail-document-body"
          >
            {markdown}
          </pre>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function DependenciesCard({ deps }: { deps: string[] }): ReactElement {
  return (
    <SurfaceCard data-testid="task-detail-dependencies">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Dependencies</SurfaceCardEyebrow>
          <SurfaceCardTitle>depends_on</SurfaceCardTitle>
          <SurfaceCardDescription>
            Tasks that must complete before this one can progress.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{deps.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {deps.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="task-detail-dependencies-empty">
            No declared dependencies.
          </p>
        ) : (
          <ul className="flex flex-wrap gap-2" data-testid="task-detail-dependencies-list">
            {deps.map(dep => (
              <li
                className="rounded-[var(--radius-sm)] border border-border bg-black/10 px-2 py-1 text-xs text-muted-foreground"
                data-testid={`task-detail-dependency-${dep}`}
                key={dep}
              >
                {dep}
              </li>
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function RelatedRunsCard({ runs }: { runs: TaskRelatedRun[] }): ReactElement {
  return (
    <SurfaceCard data-testid="task-detail-related-runs">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Related runs</SurfaceCardEyebrow>
          <SurfaceCardTitle>Recent task runs</SurfaceCardTitle>
          <SurfaceCardDescription>
            Daemon runs associated with this task, most recent first.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{runs.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {runs.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="task-detail-related-runs-empty">
            No related runs yet.
          </p>
        ) : (
          <ul className="space-y-2" data-testid="task-detail-related-runs-list">
            {runs.map(run => (
              <RelatedRunRow key={run.run_id} run={run} />
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function RelatedRunRow({ run }: { run: TaskRelatedRun }): ReactElement {
  const tone = resolveRunStatusTone(run.status);
  return (
    <li
      className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-testid={`task-detail-run-row-${run.run_id}`}
    >
      <div className="min-w-0 space-y-1">
        <Link
          className="truncate text-sm font-medium text-foreground hover:underline"
          data-testid={`task-detail-run-link-${run.run_id}`}
          params={{ runId: run.run_id }}
          to="/runs/$runId"
        >
          {run.run_id}
        </Link>
        <p className="truncate text-xs text-muted-foreground">
          {run.mode} · started {formatTimestamp(run.started_at)}
          {run.ended_at ? ` · ended ${formatTimestamp(run.ended_at)}` : " · in flight"}
        </p>
      </div>
      <StatusBadge data-testid={`task-detail-run-status-${run.run_id}`} tone={tone}>
        {run.status}
      </StatusBadge>
    </li>
  );
}

function MemoryCard({
  entries,
  slug,
}: {
  entries: WorkflowMemoryEntry[];
  slug: string;
}): ReactElement {
  return (
    <SurfaceCard data-testid="task-detail-memory">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Memory</SurfaceCardEyebrow>
          <SurfaceCardTitle>Related memory files</SurfaceCardTitle>
          <SurfaceCardDescription>
            Workflow memory files associated with {slug}.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{entries.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {entries.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="task-detail-memory-empty">
            No related memory entries.
          </p>
        ) : (
          <ul className="space-y-2" data-testid="task-detail-memory-list">
            {entries.map(entry => (
              <li
                className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
                data-testid={`task-detail-memory-row-${entry.file_id}`}
                key={entry.file_id}
              >
                <p className="eyebrow text-muted-foreground">{entry.kind}</p>
                <p className="mt-1 truncate text-sm text-foreground" title={entry.display_path}>
                  {entry.title}
                </p>
                <p className="truncate text-xs text-muted-foreground">{entry.display_path}</p>
              </li>
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
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
