import { RotateCcw, Save, SlidersHorizontal } from "lucide-react";

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
  FormSection,
} from "@agh/ui";

import { useLoopConfigure } from "../../hooks/use-loop-configure";
import type { LoopConfig, LoopDetail, LoopEffectiveConfig } from "../../types";
import { LoopConfigureChecks } from "./loop-configure-checks";
import { LoopConfigureLimits } from "./loop-configure-limits";
import { LoopConfigureStrategy } from "./loop-configure-strategy";
import { LoopConfigureSwitchRow } from "./loop-configure-switch-row";

interface LoopConfigureDialogProps {
  open: boolean;
  workspaceId: string;
  loop: LoopDetail;
  config: LoopConfig | null;
  effectiveConfig: LoopEffectiveConfig;
  onOpenChange: (open: boolean) => void;
  /** Opens the builder for structural edits. */
  onOpenEditor?: () => void;
}

/**
 * The no-fork Configure surface (design §4.7): writes the per-loop `loop_config`
 * store — verification-check toggles + command overrides, the human-approval-gate
 * flag, the re-attempt strategy, and the 6 clamped limit overrides.
 *
 * Structural fields (node order / kinds / inputs / contract / goal) are not
 * editable here; they belong in the builder (ADR-009). There is NO cost-cap
 * field — cost is display-only (ADR-017).
 *
 * A modal rather than a sheet: this is a bounded, single-entity edit that ends
 * in Save or Cancel, which is the entity-editor contract. Sheets are for
 * browse-heavy, long-lived inspection.
 */
export function LoopConfigureDialog({
  open,
  workspaceId,
  loop,
  config,
  effectiveConfig,
  onOpenChange,
  onOpenEditor,
}: LoopConfigureDialogProps) {
  const model = useLoopConfigure({
    workspaceId,
    loop,
    config,
    effectiveConfig,
    onSaved: () => onOpenChange(false),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-testid="loop-configure-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`Adjust how ${loop.name} runs without changing its structure.`}
          eyebrow="Loops · Configure"
          icon={SlidersHorizontal}
          onClose={model.busy ? undefined : () => onOpenChange(false)}
          title={`Configure ${loop.name}`}
        />

        <EntityDialogBody className="flex flex-col" data-testid="loop-configure-body">
          <Alert data-testid="loop-configure-structural-note" variant="neutral">
            <AlertDescription>
              Steps, inputs, node kinds and the goal stay fixed here. To change those,{" "}
              <button
                className="font-medium text-accent-strong underline-offset-2 hover:underline disabled:no-underline disabled:opacity-70"
                data-testid="loop-configure-edit-link"
                disabled={!onOpenEditor || model.busy}
                onClick={onOpenEditor}
                type="button"
              >
                {loop.source === "workspace" ? "Edit" : "Fork & edit"}
              </button>{" "}
              the definition in the builder.
            </AlertDescription>
          </Alert>

          <FormSection rightLabel="declared in the loop" title="Review gate">
            <LoopConfigureChecks
              descriptors={model.descriptors}
              disabled={model.busy}
              onCommandChange={model.setCheckCommand}
              onToggle={model.toggleCheck}
              states={model.draft.checks}
            />
          </FormSection>

          <FormSection title="Human approval gate">
            <div className="overflow-hidden rounded-lg border border-line-soft bg-canvas-tint">
              <LoopConfigureSwitchRow
                checked={model.draft.humanGateEnabled}
                description="Pauses the run as needs-approval for a human decision before it completes. Off by default."
                disabled={model.busy}
                onCheckedChange={model.setHumanGate}
                testId="loop-configure-human-gate"
                title="Pause for a human decision"
                typeLabel="human"
              />
            </div>
          </FormSection>

          <FormSection title="Re-attempt strategy">
            <LoopConfigureStrategy
              disabled={model.busy}
              onChange={model.setStrategy}
              value={model.draft.reattemptStrategy}
            />
          </FormSection>

          <FormSection rightLabel="per-loop defaults" title="Stop limits">
            <LoopConfigureLimits
              disabled={model.busy}
              draft={model.draft.limits}
              effectiveConfig={effectiveConfig}
              onChange={model.setLimits}
            />
          </FormSection>
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={model.busy}
          cancelTestId="loop-configure-cancel"
          isSaving={model.busy}
          leading={
            <Button
              data-testid="loop-configure-reset"
              disabled={model.busy}
              onClick={model.handleReset}
              size="sm"
              type="button"
              variant="ghost"
            >
              <RotateCcw aria-hidden="true" className="size-3.5" />
              Reset to defaults
            </Button>
          }
          onCancel={() => onOpenChange(false)}
          onPrimary={model.handleSave}
          primaryIcon={Save}
          primaryLabel={model.busy ? "Saving..." : "Save configuration"}
          primaryTestId="loop-configure-save"
        />
      </DialogContent>
    </Dialog>
  );
}
