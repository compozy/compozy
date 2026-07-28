"use client";

import { SaveIcon, Undo2Icon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import { Spinner } from "../spinner";

export interface PageActionsTopbarSlotProps extends React.ComponentProps<"div"> {
  /** Whether the underlying page has unsaved changes. */
  dirty: boolean;
  /** Save handler. */
  onSave: () => void;
  /** Discard handler. */
  onDiscard: () => void;
  /** Disables both buttons + swaps the save icon for a spinner. */
  saving?: boolean;
  /**
   * When dirty and not saving, marks Save as `aria-disabled` (still focusable)
   * instead of HTML `disabled` — used for client validation blocks.
   */
  saveBlocked?: boolean;
  /** Caption rendered beside the actions when `saveBlocked` is true. */
  saveBlockedCaption?: React.ReactNode;
  /** Optional override label for the save button. */
  saveLabel?: React.ReactNode;
  /** Optional override label for the discard button. */
  discardLabel?: React.ReactNode;
  /** Optional override label rendered while `saving` is true. */
  savingLabel?: React.ReactNode;
}

function PageActionsTopbarSlot({
  dirty,
  onSave,
  onDiscard,
  saving = false,
  saveBlocked = false,
  saveBlockedCaption,
  saveLabel = "Save changes",
  discardLabel = "Discard",
  savingLabel = "Saving…",
  className,
  ...props
}: PageActionsTopbarSlotProps) {
  const discardDisabled = !dirty || saving;
  const saveHtmlDisabled = !dirty || saving;
  const saveAriaDisabled = dirty && !saving && saveBlocked;

  return (
    <div
      data-slot="page-actions-topbar-slot"
      data-dirty={dirty ? "true" : "false"}
      data-saving={saving ? "true" : undefined}
      data-save-blocked={saveAriaDisabled ? "true" : undefined}
      className={cn("flex flex-wrap items-center justify-end gap-2", className)}
      {...props}
    >
      {saveAriaDisabled && saveBlockedCaption ? (
        <span
          className="text-xs text-subtle"
          data-slot="page-actions-topbar-slot-blocked-caption"
          data-testid="page-actions-topbar-slot-blocked-caption"
        >
          {saveBlockedCaption}
        </span>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        data-slot="page-actions-topbar-slot-discard"
        disabled={discardDisabled}
        onClick={onDiscard}
      >
        <Undo2Icon className="size-3" />
        {discardLabel}
      </Button>
      <Button
        type="button"
        variant="default"
        size="sm"
        data-slot="page-actions-topbar-slot-save"
        disabled={saveHtmlDisabled}
        aria-disabled={saveAriaDisabled || undefined}
        onClick={() => {
          if (saveAriaDisabled) return;
          onSave();
        }}
      >
        {saving ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : (
          <SaveIcon className="size-3" />
        )}
        {saving ? savingLabel : saveLabel}
      </Button>
    </div>
  );
}

export { PageActionsTopbarSlot };
