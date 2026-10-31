import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  FormSection,
  SkeletonRegion,
} from "@compozy/ui";

import { useWorktrees } from "@/systems/workspace";

import { useLoopConfigure } from "../../hooks/use-loop-configure";
import type { LoopConfig, LoopDetail, LoopEffectiveConfig } from "../../types";
import { LoopConfigureChecks } from "./loop-configure-checks";
import { LoopConfigureFooter, LoopConfigureHeader } from "./loop-configure-chrome";
import { LoopConfigureLimits } from "./loop-configure-limits";
import { LoopConfigurePending } from "./loop-configure-pending";
import { LoopConfigureStrategy } from "./loop-configure-strategy";
import { LoopConfigureSwitchRow } from "./loop-configure-switch-row";
import { LoopWorktreeSection, LoopWorktreeSectionSkeleton } from "./loop-worktree-section";

export interface LoopConfigureDialogProps {
  open: boolean;
  workspaceId: string;
  /** The Loop's name from the route: enough to name the dialog before the Loop has been read. */
  name?: string;
  /** The Loop and its settings. Until both have been read the dialog shows its placeholders. */
  loop?: LoopDetail;
  config?: LoopConfig | null;
  effectiveConfig?: LoopEffectiveConfig;
  onOpenChange: (open: boolean) => void;
  /** Opens the builder for structural edits. */
  onOpenEditor?: () => void;
}

/**
 * The configure dialog. Without a `loop` it shows the dialog's chrome over placeholders; with one
 * it shows the form. Both render through the same `Dialog`, so the settings arriving swaps the
 * dialog's content rather than closing and reopening it.
 */
export function LoopConfigureDialog({
  open,
  workspaceId,
  name,
  loop,
  config = null,
  effectiveConfig,
  onOpenChange,
  onOpenEditor,
}: LoopConfigureDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-testid="loop-configure-dialog"
        showCloseButton={false}
        unframed
      >
        {loop && effectiveConfig ? (
          <LoopConfigureForm
            config={config}
            effectiveConfig={effectiveConfig}
            loop={loop}
            onOpenChange={onOpenChange}
            onOpenEditor={onOpenEditor}
            open={open}
            workspaceId={workspaceId}
          />
        ) : (
          <LoopConfigurePending
            name={name ?? loop?.name ?? ""}
            onClose={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

interface LoopConfigureFormProps {
  open: boolean;
  workspaceId: string;
  loop: LoopDetail;
  config: LoopConfig | null;
  effectiveConfig: LoopEffectiveConfig;
  onOpenChange: (open: boolean) => void;
  onOpenEditor?: () => void;
}

function LoopConfigureForm({
  open,
  workspaceId,
  loop,
  config,
  effectiveConfig,
  onOpenChange,
  onOpenEditor,
}: LoopConfigureFormProps) {
  const worktrees = useWorktrees(workspaceId, { enabled: open });
  const model = useLoopConfigure({
    workspaceId,
    loop,
    config,
    effectiveConfig,
    onSaved: () => onOpenChange(false),
  });

  return (
    <>
      <LoopConfigureHeader
        name={loop.name}
        onClose={model.busy ? undefined : () => onOpenChange(false)}
      />

      <EntityDialogBody className="flex flex-col" data-testid="loop-configure-body">
        <Alert data-testid="loop-configure-structural-note" variant="neutral">
          <AlertDescription>Steps, inputs, and the goal stay fixed here.</AlertDescription>
          <AlertActions>
            <Button
              data-testid="loop-configure-edit-link"
              disabled={!onOpenEditor || model.busy}
              onClick={onOpenEditor}
              size="sm"
              type="button"
              variant="link"
            >
              {loop.source === "workspace" ? "Edit" : "Copy and edit"} in the builder
            </Button>
          </AlertActions>
        </Alert>

        <FormSection title="Checks">
          <LoopConfigureChecks
            descriptors={model.descriptors}
            disabled={model.busy}
            onCommandChange={model.setCheckCommand}
            onToggle={model.toggleCheck}
            states={model.draft.checks}
          />
        </FormSection>

        <FormSection title="Your approval">
          <div className="overflow-hidden rounded-lg bg-sunken">
            <LoopConfigureSwitchRow
              checked={model.draft.humanGateEnabled}
              disabled={model.busy}
              help="Pauses the run and asks you to approve before it finishes. Off by default."
              onCheckedChange={model.setHumanGate}
              testId="loop-configure-human-gate"
              title="Ask me before finishing"
            />
          </div>
        </FormSection>

        <FormSection title="If a round fails">
          <LoopConfigureStrategy
            disabled={model.busy}
            onChange={model.setStrategy}
            value={model.draft.reattemptStrategy}
          />
        </FormSection>

        {worktrees.isPending ? (
          <SkeletonRegion label="Loading environments">
            <LoopWorktreeSectionSkeleton />
          </SkeletonRegion>
        ) : worktrees.error ? (
          <FormSection title="Environment default">
            <Alert variant="destructive">
              <AlertDescription>
                {worktrees.error instanceof Error
                  ? worktrees.error.message
                  : "Environments could not be loaded."}
              </AlertDescription>
            </Alert>
            <Button
              className="mt-2"
              onClick={() => void worktrees.refetch()}
              size="sm"
              type="button"
              variant="secondary"
            >
              Retry
            </Button>
          </FormSection>
        ) : worktrees.data ? (
          <LoopWorktreeSection
            disabled={model.busy}
            gitBacked={worktrees.data.repo.git_backed}
            onChange={model.setEnvironment}
            value={model.draft.environment}
            worktrees={worktrees.data.worktrees}
          />
        ) : null}

        <FormSection title="Limits">
          <LoopConfigureLimits
            disabled={model.busy}
            draft={model.draft.limits}
            effectiveConfig={effectiveConfig}
            onChange={model.setLimits}
          />
        </FormSection>
      </EntityDialogBody>

      <LoopConfigureFooter
        actions={{
          busy: model.busy,
          onPrimary: model.handleSave,
          onReset: model.handleReset,
        }}
        onCancel={() => onOpenChange(false)}
      />
    </>
  );
}
