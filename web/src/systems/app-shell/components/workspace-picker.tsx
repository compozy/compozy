import type { ReactElement } from "react";

import { ChevronRight } from "lucide-react";

import {
  Alert,
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellSidebar,
  SectionHeading,
  StatusBadge,
} from "@compozy/ui";

import type { Workspace } from "../types";

export interface WorkspacePickerProps {
  workspaces: Workspace[];
  staleWorkspaceId?: string | null;
  onSelect: (workspaceId: string) => void;
}

export function WorkspacePicker({
  workspaces,
  staleWorkspaceId,
  onSelect,
}: WorkspacePickerProps): ReactElement {
  return (
    <AppShell>
      <AppShellSidebar>
        <AppShellBrand
          badge={<StatusBadge tone="accent">daemon</StatusBadge>}
          detail="localhost · operator runtime"
          title="Compozy"
        />
        <p className="text-sm leading-6 text-muted-foreground">
          The shell is single-workspace-per-tab. Pick one to attach and the rest of the navigation
          will unlock for this browser tab.
        </p>
      </AppShellSidebar>

      <AppShellMain>
        <AppShellHeader>
          <SectionHeading
            description="Select the workspace the daemon should act on for this browser tab."
            eyebrow="Select"
            title="Choose a workspace"
          />
        </AppShellHeader>

        <AppShellContent>
          {staleWorkspaceId ? (
            <Alert data-testid="workspace-picker-stale" variant="warning">
              Your previously selected workspace is no longer registered with the daemon. Pick a new
              one to continue.
            </Alert>
          ) : null}

          <ul
            className="overflow-hidden rounded-[var(--radius-xl)] border border-border-subtle bg-card shadow-[var(--shadow-sm)]"
            data-testid="workspace-picker-list"
          >
            {workspaces.map(workspace => (
              <li className="border-b border-border-subtle last:border-b-0" key={workspace.id}>
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
            ))}
          </ul>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}
