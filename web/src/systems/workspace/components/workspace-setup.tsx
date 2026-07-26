import { Layers, Plus, Sparkles } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Eyebrow,
  Spinner,
} from "@agh/ui";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import { WORKSPACE_SETUP_COPY } from "../lib/workspace-setup-copy";
import type { WorkspaceSetupDefaultsModel } from "../lib/workspace-setup-defaults";
import { WorkspaceSetupDefaultsPane } from "./workspace-setup-defaults-pane";
import { WorkspaceSetupLocationPane } from "./workspace-setup-location-pane";

interface WorkspaceSetupModel {
  defaults: WorkspaceSetupDefaultsModel;
  setup: WorkspaceSetupContent;
}

interface WorkspaceSetupSharedProps {
  model: WorkspaceSetupModel;
}

interface WorkspaceSetupDialogProps extends WorkspaceSetupSharedProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type WorkspaceOnboardingProps = WorkspaceSetupSharedProps;

function WorkspaceSetupDialog({ open, onOpenChange, model }: WorkspaceSetupDialogProps) {
  const { setup, defaults } = model;
  const isSubmitting = setup.submissionMode !== null;
  const location = <WorkspaceSetupLocationPane setup={setup} size="compact" />;
  const defaultsPane = <WorkspaceSetupDefaultsPane defaults={defaults} setup={setup} />;

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("xl")}`}
        data-testid="workspace-setup-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Register a local directory as a workspace. The root becomes immutable — defaults stay editable afterwards."
          eyebrow="Workspace"
          icon={Layers}
          onClose={isSubmitting ? undefined : () => onOpenChange(false)}
          title={WORKSPACE_SETUP_COPY.dialog.title}
        />

        <form className="contents" onSubmit={event => void setup.handleCreateSubmit(event)}>
          <EntityDialogBody
            data-testid="workspace-setup-dialog-body"
            side={defaultsPane}
            variant="split"
          >
            {location}
            {setup.createError ? (
              <Alert className="mt-4" data-testid="workspace-setup-error" variant="danger">
                <AlertDescription>{setup.createError}</AlertDescription>
              </Alert>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            cancelDisabled={isSubmitting}
            cancelTestId="workspace-setup-cancel"
            hint="Registration is instant — sessions can launch here right away."
            isSaving={setup.submissionMode === "create"}
            onCancel={() => onOpenChange(false)}
            primaryDisabled={!setup.canSubmit}
            primaryLabel="Add workspace"
            primaryTestId="workspace-setup-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

function WorkspaceOnboarding({ model }: WorkspaceOnboardingProps) {
  const copy = WORKSPACE_SETUP_COPY.onboarding;
  const { setup, defaults } = model;
  const location = <WorkspaceSetupLocationPane setup={setup} size="comfortable" />;
  const defaultsPane = <WorkspaceSetupDefaultsPane defaults={defaults} setup={setup} />;

  return (
    <div
      className="flex min-h-0 flex-1 items-start justify-center overflow-y-auto bg-background p-6 lg:items-center lg:py-10"
      data-testid="workspace-onboarding"
    >
      <div className="w-full max-w-5xl rounded-xl border border-line bg-canvas p-6 sm:p-8 lg:p-10">
        <div className="grid gap-8 lg:grid-cols-[minmax(0,1.15fr)_minmax(22rem,24rem)] lg:gap-8 xl:gap-10">
          <div className="flex flex-col justify-between gap-6">
            <div className="space-y-4">
              <span
                aria-hidden="true"
                data-testid="workspace-onboarding-hero-icon"
                className="inline-flex size-14 items-center justify-center rounded-icon-well bg-surface-glaze text-accent"
              >
                <Sparkles className="size-6" />
              </span>
              <Eyebrow className="text-muted">{copy.eyebrow}</Eyebrow>
              <div className="space-y-3">
                <h1
                  data-testid="workspace-onboarding-hero-title"
                  className="max-w-xl text-detail-h1 font-medium tracking-detail-h1 text-fg"
                >
                  {copy.title}
                </h1>
                <p className="max-w-xl text-item-title leading-7 text-muted">{copy.description}</p>
              </div>
            </div>

            <div className="rounded-2xl border border-line bg-canvas-soft p-4">
              <Eyebrow className="text-muted">{copy.noteLabel}</Eyebrow>
              <p className="mt-2 text-sm leading-6 text-muted">{copy.noteBody}</p>
            </div>
          </div>

          <form
            className="flex w-full flex-col gap-4"
            data-testid="workspace-setup-options"
            onSubmit={event => void setup.handleCreateSubmit(event)}
          >
            {location}
            {defaultsPane}
            {setup.createError ? (
              <Alert data-testid="workspace-setup-error" variant="danger">
                <AlertDescription>{setup.createError}</AlertDescription>
              </Alert>
            ) : null}
            <Button
              className="w-full"
              data-testid="workspace-setup-onboarding-submit"
              disabled={!setup.canSubmit}
              type="submit"
            >
              {setup.submissionMode === "create" ? <Spinner /> : <Plus className="size-3.5" />}
              {WORKSPACE_SETUP_COPY.dialog.title}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}

export { WorkspaceOnboarding, WorkspaceSetupDialog };
export type { WorkspaceSetupModel };
