import { RotateCcw, Save, SlidersHorizontal } from "@compozy/ui/icons";

import { Button, EntityDialogFooter, EntityDialogHeader } from "@compozy/ui";

interface LoopConfigureHeaderProps {
  name: string;
  /** Omitted while a save is in flight, so the dialog cannot be dismissed mid-write. */
  onClose?: () => void;
}

/** The dialog's header; static for a Loop, so the pending and loaded forms share it. */
export function LoopConfigureHeader({ name, onClose }: LoopConfigureHeaderProps) {
  return (
    <EntityDialogHeader
      description={`Adjust how ${name} runs without changing its structure.`}
      eyebrow="Loops · Configure"
      icon={SlidersHorizontal}
      onClose={onClose}
      title={`Configure ${name}`}
    />
  );
}

interface LoopConfigureFooterProps {
  onCancel: () => void;
  /** Absent while the settings are still being read: every action but Cancel waits. */
  actions?: {
    busy: boolean;
    onPrimary: () => void;
    onReset: () => void;
  };
}

/** The dialog's footer: Reset to defaults, Cancel and the one primary action. */
export function LoopConfigureFooter({ onCancel, actions }: LoopConfigureFooterProps) {
  const busy = actions?.busy ?? false;
  return (
    <EntityDialogFooter
      cancelDisabled={busy}
      cancelTestId="loop-configure-cancel"
      isSaving={busy}
      leading={
        <Button
          data-testid="loop-configure-reset"
          disabled={busy || !actions}
          onClick={actions?.onReset}
          size="sm"
          type="button"
          variant="ghost"
        >
          <RotateCcw aria-hidden="true" className="size-3.5" />
          Reset to defaults
        </Button>
      }
      onCancel={onCancel}
      onPrimary={actions?.onPrimary}
      primaryDisabled={!actions}
      primaryIcon={Save}
      primaryLabel={busy ? "Saving..." : "Save configuration"}
      primaryTestId="loop-configure-save"
    />
  );
}
