import { Folder, X } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  DirectoryPickerInput,
  Empty,
  Eyebrow,
  KindIcon,
  Spinner,
} from "@compozy/ui";

import { DESTINATION_COPY } from "@/systems/workspace";
import type { OnboardingWorkspacesApi } from "../hooks/use-onboarding-workspaces";

interface StepWorkspacesProps {
  workspaces: OnboardingWorkspacesApi;
  onSkip: () => void;
  skipDisabled?: boolean;
}

/**
 * Type, complete, or browse to a folder and add it; added projects list
 * below. The column keeps a fixed height so the panel holds one shape while
 * folders come and go.
 */
export function StepWorkspaces({ workspaces, onSkip, skipDisabled = false }: StepWorkspacesProps) {
  const selected = workspaces.workspaces;
  const field = workspaces.pathField;
  const message = field.message ?? workspaces.resolveError;

  return (
    <div className="mt-5" data-testid="onboarding-step-workspaces">
      <div className="flex h-setup-split flex-col gap-5 max-md:h-auto">
        <section className="flex flex-none flex-col">
          <label className="mb-2.5 block" htmlFor="onboarding-project-folder">
            <Eyebrow className="text-subtle">Project folder</Eyebrow>
          </label>
          {/* Onboarding registers on add — the wizard has no deferred commit. */}
          <form
            className="flex items-center gap-2"
            onSubmit={event => {
              event.preventDefault();
              void workspaces.addFromField();
            }}
          >
            <DirectoryPickerInput
              aria-describedby={message ? "onboarding-project-folder-message" : undefined}
              data-testid="onboarding-folder-input"
              id="onboarding-project-folder"
              invalid={field.status === "invalid"}
              onBrowse={field.browse}
              onValueChange={field.onValueChange}
              placeholder="~/code/my-project"
              rootClassName="min-w-0 flex-1"
              suggestions={field.suggestions}
              browsePending={field.browsePending}
              value={field.value}
            />
            <Button
              data-testid="onboarding-add-folder"
              disabled={field.status !== "valid" || workspaces.isResolving}
              type="submit"
              variant="secondary"
            >
              {workspaces.isResolving ? <Spinner /> : null}
              Add
            </Button>
          </form>
          {message ? (
            <p
              className="mt-2 text-form-hint text-destructive"
              data-testid="onboarding-resolve-error"
              id="onboarding-project-folder-message"
              role="alert"
            >
              {message}
            </p>
          ) : null}
        </section>

        <section className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="flex items-baseline justify-between gap-2.5">
            <Eyebrow className="text-subtle">Selected projects</Eyebrow>
            <span className="text-micro text-muted-foreground tabular-nums">
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
            <Empty
              className="mt-2.5 flex-1 max-md:min-h-24"
              description="Folders you add appear here."
              fill={false}
              title="None yet"
            />
          ) : (
            <ul className="mt-2.5 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto max-md:max-h-50">
              {selected.map(workspace => (
                <li
                  key={workspace.path}
                  className="flex flex-none items-center gap-2.5 rounded-lg bg-card px-2.5 py-2 shadow-card"
                  data-testid="onboarding-selected-workspace"
                >
                  <KindIcon icon={Folder} tone="well" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-body font-medium text-fg">
                      {workspace.name}
                    </span>
                    <span className="block truncate font-mono text-eyebrow text-subtle">
                      {workspace.path}
                    </span>
                  </span>
                  <Button
                    variant="quiet"
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
              variant="secondary"
            >
              {DESTINATION_COPY.skipOnboarding}
            </Button>
          </div>
        </section>
      </div>
    </div>
  );
}
