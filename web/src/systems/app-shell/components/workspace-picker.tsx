import type { ReactElement } from "react";

import { ChevronRight, RefreshCw } from "lucide-react";

import {
  Alert,
  AppShell,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellSidebar,
  Button,
  Logo,
  SectionHeading,
  StatusBadge,
} from "@compozy/ui";

import type { Workspace } from "../types";

export interface WorkspacePickerProps {
  workspaces: Workspace[];
  staleWorkspaceId?: string | null;
  syncError?: string | null;
  syncMessage?: string | null;
  isSyncing?: boolean;
  onSelect: (workspaceId: string) => void;
  onSync?: () => void;
}

export function WorkspacePicker({
  workspaces,
  staleWorkspaceId,
  syncError,
  syncMessage,
  isSyncing = false,
  onSelect,
  onSync,
}: WorkspacePickerProps): ReactElement {
  const missingWorkspaces = workspaces.filter(isMissingWorkspace);
  const availableWorkspaces = workspaces.filter(workspace => !isMissingWorkspace(workspace));
  const shouldShowSectionLabels = missingWorkspaces.length > 0;

  return (
    <AppShell>
      <AppShellSidebar>
        <Logo size="sm" variant="full" symbolSrc="/symbol.png" />
        <p className="text-sm leading-6 text-muted-foreground">
          The shell is single-workspace-per-tab. Pick one to attach and the rest of the navigation
          will unlock for this browser tab.
        </p>
      </AppShellSidebar>

      <AppShellMain>
        <AppShellHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <SectionHeading
              description="Select the workspace the daemon should act on for this browser tab."
              eyebrow="Select"
              title="Choose a workspace"
            />
            {onSync ? (
              <Button
                data-testid="workspace-picker-sync"
                icon={<RefreshCw className="size-4" />}
                loading={isSyncing}
                onClick={onSync}
                variant="secondary"
              >
                Refresh workspaces
              </Button>
            ) : null}
          </div>
        </AppShellHeader>

        <AppShellContent>
          {staleWorkspaceId ? (
            <Alert data-testid="workspace-picker-stale" variant="warning">
              Your previously selected workspace is no longer registered with the daemon. Pick a new
              one to continue.
            </Alert>
          ) : null}

          {syncMessage ? (
            <Alert data-testid="workspace-picker-sync-success" variant="success">
              {syncMessage}
            </Alert>
          ) : null}

          {syncError ? (
            <Alert data-testid="workspace-picker-sync-error" variant="error">
              {syncError}
            </Alert>
          ) : null}

          <div className="space-y-5" data-testid="workspace-picker-list">
            {availableWorkspaces.length > 0 ? (
              <WorkspaceSection
                label={shouldShowSectionLabels ? `Available · ${availableWorkspaces.length}` : null}
                onSelect={onSelect}
                testId="workspace-picker-available"
                workspaces={availableWorkspaces}
              />
            ) : null}

            {missingWorkspaces.length > 0 ? (
              <WorkspaceSection
                label={`Path missing · ${missingWorkspaces.length}`}
                onSelect={onSelect}
                testId="workspace-picker-missing-section"
                workspaces={missingWorkspaces}
              />
            ) : null}
          </div>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}

function isMissingWorkspace(workspace: Workspace): boolean {
  return workspace.filesystem_state === "missing";
}

function WorkspaceSection({
  label,
  onSelect,
  testId,
  workspaces,
}: {
  label: string | null;
  onSelect: (workspaceId: string) => void;
  testId: string;
  workspaces: Workspace[];
}): ReactElement {
  return (
    <section className="space-y-3" data-testid={testId}>
      {label ? <p className="eyebrow text-muted-foreground">{label}</p> : null}
      <ul className="overflow-hidden rounded-[var(--radius-xl)] border border-border-subtle bg-card shadow-[var(--shadow-sm)]">
        {workspaces.map(workspace => (
          <WorkspaceRow key={workspace.id} onSelect={onSelect} workspace={workspace} />
        ))}
      </ul>
    </section>
  );
}

function WorkspaceRow({
  onSelect,
  workspace,
}: {
  onSelect: (workspaceId: string) => void;
  workspace: Workspace;
}): ReactElement {
  return (
    <li className="border-b border-border-subtle last:border-b-0">
      <button
        className="group grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-5 py-4 text-left transition-[background-color,color] duration-200 hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60"
        data-testid={`workspace-picker-select-${workspace.id}`}
        onClick={() => onSelect(workspace.id)}
        type="button"
      >
        <span className="min-w-0">
          <span className="eyebrow text-muted-foreground">workspace</span>
          <span className="mt-1 block truncate text-sm font-semibold text-foreground">
            {workspace.name}
          </span>
          {workspace.filesystem_state === "missing" || workspace.read_only ? (
            <span className="mt-2 flex flex-wrap gap-2">
              {workspace.filesystem_state === "missing" ? (
                <StatusBadge
                  data-testid={`workspace-picker-missing-${workspace.id}`}
                  tone="warning"
                >
                  path missing
                </StatusBadge>
              ) : null}
              {workspace.read_only ? (
                <StatusBadge
                  data-testid={`workspace-picker-readonly-${workspace.id}`}
                  tone="neutral"
                >
                  read-only
                </StatusBadge>
              ) : null}
            </span>
          ) : null}
          <span
            className="mt-1 block truncate font-mono text-xs text-muted-foreground"
            title={workspace.root_dir}
          >
            {workspace.root_dir}
          </span>
        </span>
        <span className="flex items-center gap-3">
          <StatusBadge tone="info">select</StatusBadge>
          <ChevronRight
            aria-hidden
            className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary"
          />
        </span>
      </button>
    </li>
  );
}
