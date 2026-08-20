import { Layers } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
} from "@compozy/ui";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import { WORKSPACE_SETUP_COPY } from "../lib/workspace-setup-copy";
import type { WorkspaceSetupDefaultsModel } from "../lib/workspace-setup-defaults";
import { WorkspaceSetupDefaultsPane } from "./workspace-setup-defaults-pane";
import { WorkspaceSetupLocationPane } from "./workspace-setup-location-pane";

interface WorkspaceSetupModel {
  defaults: WorkspaceSetupDefaultsModel;
  setup: WorkspaceSetupContent;
}

interface WorkspaceSetupDialogProps {
  model: WorkspaceSetupModel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function WorkspaceSetupDialog({ open, onOpenChange, model }: WorkspaceSetupDialogProps) {
  const { setup, defaults } = model;
  const isSubmitting = setup.submissionMode !== null;
  const location = <WorkspaceSetupLocationPane setup={setup} />;
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
          description={WORKSPACE_SETUP_COPY.dialog.description}
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

export { WorkspaceSetupDialog };
export type { WorkspaceSetupModel };
