import { useId, useState, type ReactElement, type FormEvent } from "react";

import {
  Alert,
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
  const inputIds = useId();

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
  const inputDescriptionId = `${inputIds}-help`;
  const inputErrorId = `${inputIds}-error`;

  return (
    <AppShell>
      <AppShellSidebar>
        <AppShellBrand badge={<StatusBadge tone="accent">daemon</StatusBadge>} title="Compozy" />
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
                  <span className="eyebrow text-muted-foreground">Absolute workspace path</span>
                  <input
                    aria-describedby={
                      errorMessage ? `${inputDescriptionId} ${inputErrorId}` : inputDescriptionId
                    }
                    aria-invalid={Boolean(errorMessage)}
                    className="w-full rounded-[var(--radius-md)] border border-input bg-[color:var(--surface-inset)] px-3 py-2 font-mono text-sm text-foreground placeholder:text-muted-foreground/70 shadow-[var(--shadow-xs)] transition-[border-color,box-shadow] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
                    data-testid="workspace-onboarding-input"
                    name="workspace-path"
                    onChange={event => setPath(event.target.value)}
                    placeholder="/Users/you/projects/example"
                    required
                    spellCheck={false}
                    value={path}
                  />
                </label>
                <p
                  className="text-xs leading-5 text-muted-foreground"
                  data-testid="workspace-onboarding-input-help"
                  id={inputDescriptionId}
                >
                  Use an absolute path the daemon can read.
                </p>

                {errorMessage ? (
                  <Alert
                    data-testid="workspace-onboarding-error"
                    id={inputErrorId}
                    role="alert"
                    variant="error"
                  >
                    {errorMessage}
                  </Alert>
                ) : null}

                <div className="flex items-center gap-2">
                  <Button
                    data-testid="workspace-onboarding-submit"
                    disabled={resolve.isPending || path.trim().length === 0}
                    loading={resolve.isPending}
                    size="sm"
                    type="submit"
                  >
                    Resolve workspace
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
