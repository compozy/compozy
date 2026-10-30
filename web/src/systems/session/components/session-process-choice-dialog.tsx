import { Archive, Square, Trash2 } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import {
  processPortsSentence,
  sessionProcessChoiceCopy,
  type SessionProcessChoiceAction,
} from "../lib/session-process-choice";

export interface SessionProcessChoiceDialogProps {
  open: boolean;
  action: SessionProcessChoiceAction;
  /** Sessions the verb applies to. */
  sessionCount: number;
  /** Processes those sessions left running. */
  processCount: number;
  ports?: readonly number[];
  /** Extra consequence copy the verb already carries (a paused Goal, a deleted transcript). */
  detail?: string;
  pending?: boolean;
  onOpenChange: (open: boolean) => void;
  onChoose: (processes: SessionProcessPolicy) => void;
}

const ACTION_ICONS = { stop: Square, archive: Archive, delete: Trash2 } as const;

/**
 * Asks what happens to the processes a session left running before a stop,
 * archive, or delete: stop them too, keep them running, or cancel. Shown only
 * while the count is positive; at zero the verb runs as it always did.
 */
export function SessionProcessChoiceDialog({
  open,
  action,
  sessionCount,
  processCount,
  ports = [],
  detail,
  pending = false,
  onOpenChange,
  onChoose,
}: SessionProcessChoiceDialogProps) {
  const copy = sessionProcessChoiceCopy(action, sessionCount, processCount);
  const portsSentence = processPortsSentence(ports);
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={next => {
        if (!pending || next) onOpenChange(next);
      }}
      title={copy.title}
      description={[copy.description, portsSentence, detail].filter(Boolean).join(" ")}
      note={copy.note}
      noteTone="neutral"
      confirmLabel={copy.confirmLabel}
      cancelLabel="Cancel"
      tone={action === "delete" ? "danger" : "warning"}
      icon={ACTION_ICONS[action]}
      isPending={pending}
      className="sm:max-w-lg"
      contentProps={{ "data-testid": "session-process-choice-dialog" }}
      confirmButtonProps={{ "data-testid": "session-process-choice-terminate" }}
      cancelButtonProps={{ "data-testid": "session-process-choice-cancel" }}
      alternateAction={{
        label: copy.keepLabel,
        onSelect: () => onChoose("keep"),
        buttonProps: { "data-testid": "session-process-choice-keep" },
      }}
      onConfirm={() => onChoose("terminate")}
    />
  );
}
