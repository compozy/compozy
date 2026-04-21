import type { ReactElement } from "react";

import {
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellSidebar,
  Button,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
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
            <p
              className="rounded-[var(--radius-md)] border border-[color:var(--color-warning)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-warning)]"
              data-testid="workspace-picker-stale"
              role="alert"
            >
              Your previously selected workspace is no longer registered with the daemon. Pick a new
              one to continue.
            </p>
          ) : null}

          <ul className="grid gap-3 md:grid-cols-2" data-testid="workspace-picker-list">
            {workspaces.map(workspace => (
              <li key={workspace.id}>
                <SurfaceCard>
                  <SurfaceCardHeader>
                    <div>
                      <SurfaceCardEyebrow>workspace</SurfaceCardEyebrow>
                      <SurfaceCardTitle>{workspace.name}</SurfaceCardTitle>
                      <SurfaceCardDescription>{workspace.root_dir}</SurfaceCardDescription>
                    </div>
                    <StatusBadge tone="info">select</StatusBadge>
                  </SurfaceCardHeader>
                  <SurfaceCardBody>
                    <Button
                      data-testid={`workspace-picker-select-${workspace.id}`}
                      onClick={() => onSelect(workspace.id)}
                      size="sm"
                      type="button"
                    >
                      Use this workspace
                    </Button>
                  </SurfaceCardBody>
                </SurfaceCard>
              </li>
            ))}
          </ul>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}
