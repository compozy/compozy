import { Layers } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  type EntityMode,
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
  // Every default is optional, so Simple is one pane: pick the folder, then Add.
  const [mode, setMode] = useState<EntityMode>("simple");
  const location = <WorkspaceSetupLocationPane setup={setup} />;
  const defaultsPane = <WorkspaceSetupDefaultsPane defaults={defaults} setup={setup} />;
  const main = (
    <>
      {location}
      {setup.createError ? (
        <Alert className="mt-4" data-testid="workspace-setup-error" variant="danger">
          <AlertDescription>{setup.createError}</AlertDescription>
        </Alert>
      ) : null}
    </>
  );

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] ${dialogShellClass(mode === "advanced" ? "xl" : "lg")}`}
        data-testid="workspace-setup-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={WORKSPACE_SETUP_COPY.dialog.description}
          eyebrow="Projects"
          icon={Layers}
          onClose={isSubmitting ? undefined : () => onOpenChange(false)}
          title={WORKSPACE_SETUP_COPY.dialog.title}
        />

        <EntityModeToolbar mode={mode} onModeChange={setMode} testIdPrefix="workspace-setup" />

        <form className="contents" onSubmit={event => void setup.handleCreateSubmit(event)}>
          {mode === "advanced" ? (
            <EntityDialogBody
              data-testid="workspace-setup-dialog-body"
              side={defaultsPane}
              variant="split"
            >
              {main}
            </EntityDialogBody>
          ) : (
            <EntityDialogBody data-testid="workspace-setup-dialog-body">{main}</EntityDialogBody>
          )}

          <EntityDialogFooter
            cancelDisabled={isSubmitting}
            cancelTestId="workspace-setup-cancel"
            isSaving={setup.submissionMode === "create"}
            onCancel={() => onOpenChange(false)}
            primaryDisabled={!setup.canSubmit}
            primaryLabel={WORKSPACE_SETUP_COPY.dialog.action}
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
