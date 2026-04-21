import { useState, type ReactElement, type FormEvent } from "react";

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

import { useResolveWorkspace } from "../hooks/use-workspaces";

interface WorkspaceOnboardingProps {
  onWorkspaceResolved?: (workspaceId: string) => void;
}

export function WorkspaceOnboarding({
  onWorkspaceResolved,
}: WorkspaceOnboardingProps): ReactElement {
  const [path, setPath] = useState("");
  const resolve = useResolveWorkspace();

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = path.trim();
    if (!trimmed) {
      return;
    }
    try {
      const workspace = await resolve.mutateAsync({ path: trimmed });
      onWorkspaceResolved?.(workspace.id);
    } catch {
      // The error is surfaced through `resolve.error` below; swallow to avoid
      // an unhandled rejection from the form submit handler.
    }
  }

  const errorMessage = resolve.error ? resolve.error.message : null;

  return (
    <AppShell>
      <AppShellSidebar>
        <AppShellBrand
          badge={<StatusBadge tone="accent">daemon</StatusBadge>}
          detail="localhost · operator runtime"
          title="Compozy"
        />
        <p className="text-sm leading-6 text-muted-foreground">
          The operator console needs a workspace before it can show any dashboard, workflow, or run
          data. Register one and the shell will unlock the rest of the navigation.
        </p>
      </AppShellSidebar>

      <AppShellMain>
        <AppShellHeader>
          <SectionHeading
            description="Point the daemon at a workspace root and the shell will attach to it for this browser tab."
            eyebrow="First run"
            title="Register a workspace"
          />
        </AppShellHeader>

        <AppShellContent>
          <SurfaceCard data-testid="workspace-onboarding">
            <SurfaceCardHeader>
              <div>
                <SurfaceCardEyebrow>workspace</SurfaceCardEyebrow>
                <SurfaceCardTitle>Resolve by path</SurfaceCardTitle>
                <SurfaceCardDescription>
                  The daemon resolves the path against its known workspaces and lazily registers it
                  if the root looks valid.
                </SurfaceCardDescription>
              </div>
              <StatusBadge tone="info">bootstrap</StatusBadge>
            </SurfaceCardHeader>

            <SurfaceCardBody>
              <form
                className="space-y-3"
                data-testid="workspace-onboarding-form"
                onSubmit={handleSubmit}
              >
                <label className="block space-y-1 text-sm">
                  <span className="font-eyebrow text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                    Absolute workspace path
                  </span>
                  <input
                    className="w-full rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/70 focus:outline-none focus:ring-1 focus:ring-accent"
                    data-testid="workspace-onboarding-input"
                    name="workspace-path"
                    onChange={event => setPath(event.target.value)}
                    placeholder="/Users/you/projects/example"
                    required
                    spellCheck={false}
                    value={path}
                  />
                </label>

                {errorMessage ? (
                  <p
                    className="text-sm text-[color:var(--color-danger)]"
                    data-testid="workspace-onboarding-error"
                    role="alert"
                  >
                    {errorMessage}
                  </p>
                ) : null}

                <div className="flex items-center gap-2">
                  <Button
                    data-testid="workspace-onboarding-submit"
                    disabled={resolve.isPending || path.trim().length === 0}
                    size="sm"
                    type="submit"
                  >
                    {resolve.isPending ? "Resolving…" : "Resolve workspace"}
                  </Button>
                </div>
              </form>
            </SurfaceCardBody>
          </SurfaceCard>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}
