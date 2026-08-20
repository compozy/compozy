import { Folder, X } from "lucide-react";

import { Alert, AlertDescription, Button, Eyebrow } from "@compozy/ui";

import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";
import type { OnboardingWorkspacesApi } from "../hooks/use-onboarding-workspaces";
import { DirectoryBrowser } from "./directory-browser";

interface StepWorkspacesProps {
  workspaces: OnboardingWorkspacesApi;
  onSkip: () => void;
  skipDisabled?: boolean;
}

/**
 * Browse on the left, confirm on the right: the two columns close flush at a
 * fixed split height so the panel keeps one shape while folders come and go.
 */
export function StepWorkspaces({ workspaces, onSkip, skipDisabled = false }: StepWorkspacesProps) {
  const selected = workspaces.workspaces;

  return (
    <div className="mt-5" data-testid="onboarding-step-workspaces">
      <div className="grid h-setup-split grid-cols-[1.12fr_1fr] gap-4.5 max-md:h-auto max-md:grid-cols-1 max-md:gap-5">
        <section className="flex min-h-0 flex-col">
          <Eyebrow className="mb-2.5 block text-subtle">Browse for a folder</Eyebrow>
          {/* Onboarding registers on pick — the wizard has no deferred commit. */}
          <DirectoryBrowser
            browseError={workspaces.browseError}
            currentPath={workspaces.currentPath}
            entries={workspaces.entries}
            homePath={workspaces.home}
            isBrowsing={workspaces.isBrowsing}
            isPicked={workspaces.isAdded}
            onGoHome={workspaces.goHome}
            onGoParent={workspaces.goToParent}
            onNavigate={workspaces.navigateTo}
            onPick={path => void workspaces.addWorkspace(path)}
            parentPath={workspaces.parent}
            pickPending={workspaces.isResolving}
            pickRowLabel={name => `Add ${name} as a workspace`}
            roots={workspaces.roots}
            testIdPrefix="onboarding-directory-browser"
          />
          {workspaces.resolveError ? (
            <p
              className="mt-2 text-form-hint text-danger"
              role="alert"
              data-testid="onboarding-resolve-error"
            >
              {workspaces.resolveError}
            </p>
          ) : null}
        </section>

        <section className="flex min-h-0 flex-col">
          <div className="flex items-baseline justify-between gap-2.5">
            <Eyebrow className="text-subtle">Selected workspaces</Eyebrow>
            <span className="text-micro text-faint tabular-nums">
              {selected.length} folder{selected.length === 1 ? "" : "s"}
            </span>
          </div>
          {workspaces.catalogError ? (
            <Alert role="alert" variant="warning" className="mt-2.5">
              <AlertDescription className="flex items-center justify-between gap-3">
                <span>{workspaces.catalogError}</span>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  disabled={workspaces.isCatalogLoading}
                  onClick={() => void workspaces.reloadCatalog()}
                >
                  Retry
                </Button>
              </AlertDescription>
            </Alert>
          ) : null}
          {selected.length === 0 ? (
            <p className="mt-2.5 grid flex-1 place-items-center rounded-md border border-dashed border-line px-4 py-4 text-center text-small-body leading-5 text-faint max-md:min-h-24">
              None yet
            </p>
          ) : (
            <ul className="mt-2.5 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto max-md:max-h-50">
              {selected.map(workspace => (
                <li
                  key={workspace.path}
                  className="flex flex-none items-center gap-2.5 rounded-md bg-canvas-soft px-2.5 py-2 ring-1 ring-inset ring-line"
                  data-testid="onboarding-selected-workspace"
                >
                  <span className="grid size-7 flex-none place-items-center rounded-sm bg-elevated text-warning">
                    <Folder className="size-3.5" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-small-body font-medium text-fg-strong">
                      {workspace.name}
                    </span>
                    <span className="block truncate font-mono text-micro text-subtle">
                      {workspace.path}
                    </span>
                  </span>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={
                      workspaces.isRemoving ||
                      workspaces.isCatalogLoading ||
                      workspaces.catalogError !== null
                    }
                    onClick={() => void workspaces.removeWorkspace(workspace.path)}
                    aria-label={`Remove ${workspace.name}`}
                  >
                    <X className="size-3.5" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <div className="mt-3" data-testid="onboarding-skip-global">
            <Button
              disabled={skipDisabled}
              onClick={onSkip}
              size="sm"
              type="button"
              variant="outline"
            >
              {GLOBAL_SCOPE_COPY.skipOnboarding}
            </Button>
          </div>
        </section>
      </div>
    </div>
  );
}
