import { Archive, Square, Trash2 } from "@compozy/ui/icons";

import { ConfirmDialog } from "@compozy/ui";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionChildrenChoice } from "../lib/session-tree";
import { sessionArchiveChildrenCopy } from "../lib/session-tree";
import { SessionChildrenCheckbox } from "./session-children-checkbox";
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
  /** Present when the targets have child sessions the verb can include. */
  childSessions?: SessionChildrenChoice;
  onOpenChange: (open: boolean) => void;
  /** `processes` is absent when none run and the dialog only asked about children. */
  onChoose: (processes?: SessionProcessPolicy) => void;
}

const ACTION_ICONS = { stop: Square, archive: Archive, delete: Trash2 } as const;

/**
 * Asks what happens to the processes a session left running before a stop,
 * archive, or delete: stop them too, keep them running, or cancel. Shown only
 * while the count is positive; at zero the verb runs as it always did, unless
 * the targets have child sessions to include, which it then asks about alone.
 */
export function SessionProcessChoiceDialog({
  open,
  action,
  sessionCount,
  processCount,
  ports = [],
  detail,
  pending = false,
  childSessions,
  onOpenChange,
  onChoose,
}: SessionProcessChoiceDialogProps) {
  const childrenBody = childSessions ? (
    <SessionChildrenCheckbox action="archive" choice={childSessions} disabled={pending} />
  ) : undefined;
  if (processCount === 0 && childSessions) {
    const childrenCopy = sessionArchiveChildrenCopy(sessionCount, childSessions.count);
    return (
      <ConfirmDialog
        open={open}
        onOpenChange={next => {
          if (!pending || next) onOpenChange(next);
        }}
        title={childrenCopy.title}
        description={childrenCopy.description}
        confirmLabel={childrenCopy.confirmLabel}
        cancelLabel="Cancel"
        tone="warning"
        icon={Archive}
        isPending={pending}
        className="sm:max-w-lg"
        contentProps={{ "data-testid": "session-process-choice-dialog" }}
        confirmButtonProps={{ "data-testid": "session-children-confirm" }}
        cancelButtonProps={{ "data-testid": "session-process-choice-cancel" }}
        body={childrenBody}
        onConfirm={() => onChoose()}
      />
    );
  }
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
      tone={action === "delete" ? "destructive" : "warning"}
      icon={ACTION_ICONS[action]}
      body={childrenBody}
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
