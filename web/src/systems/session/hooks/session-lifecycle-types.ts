import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import type { SessionBatchResult } from "../lib/session-batch";
import type { SessionChildrenChoice } from "../lib/session-tree";
import type { SessionPayload } from "../types";
import type { SessionLaterConfirmation } from "./use-session-later-actions";

export type SessionLifecycleAction =
  | "rename"
  | "stop"
  | "archive"
  | "unarchive"
  | "delete"
  | "later"
  | "back-from-later";

export interface SessionLifecycleActionHandlers {
  pendingAction: SessionLifecycleAction | null;
  pendingSessionId: string | null;
  onStop: (session: SessionPayload) => void;
  onRename: (session: SessionPayload) => void;
  onArchive: (session: SessionPayload) => void;
  onUnarchive: (session: SessionPayload) => void;
  onDelete: (session: SessionPayload) => void;
  onStopMany?: (sessions: readonly SessionPayload[]) => void;
  onArchiveMany?: (sessions: readonly SessionPayload[]) => void;
  onUnarchiveMany?: (sessions: readonly SessionPayload[]) => void;
  /** Opens Move to Later; absent where the host renders no Later dialog. */
  onMoveToLater?: (session: SessionPayload) => void;
  onMoveToLaterMany?: (sessions: readonly SessionPayload[]) => void;
  onBackFromLater?: (session: SessionPayload) => void;
  onBackFromLaterMany?: (sessions: readonly SessionPayload[]) => void;
  onDeleteMany?: (
    sessions: readonly SessionPayload[],
    onSelectionChange?: (remainingIds: readonly string[]) => void
  ) => void;
  /** Nests a top-level session under `parent` (dropped onto its row); absent where unsupported. */
  /** Loaded rows offered by the owning list as candidate parents. */
  nestParents?: readonly SessionPayload[];
  onAdopt?: (session: SessionPayload, parent: SessionPayload) => void;
  /** Returns a nested child to the top level; the row menu's Make top-level. */
  onMakeTopLevel?: (session: SessionPayload) => void;
}

export interface SessionDeleteConfirmation {
  sessions?: readonly SessionPayload[];
  results?: readonly SessionBatchResult[];
  onRetry?: () => void;
  open: boolean;
  session: SessionPayload | null;
  isDeleting: boolean;
  /** Present when the targets have child sessions to include. */
  childSessions?: SessionChildrenChoice;
  onOpenChange: (open: boolean) => void;
  /** `processes` answers what happens to running processes; absent when none run. */
  onConfirm: (processes?: SessionProcessPolicy) => void;
}

/**
 * The stop/archive question a list asks while its targets have processes
 * running: stop them too, keep them, or cancel. Hosts render it with
 * `SessionProcessChoiceDialog`.
 */
export interface SessionProcessChoiceConfirmation {
  open: boolean;
  action: "stop" | "archive";
  sessionCount: number;
  processCount: number;
  ports: readonly number[];
  /** Present when the targets have child sessions to include. */
  childSessions?: SessionChildrenChoice;
  onOpenChange: (open: boolean) => void;
  onChoose: (processes?: SessionProcessPolicy) => void;
}

export interface SessionRenameConfirmation {
  open: boolean;
  session: SessionPayload | null;
  isRenaming: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (name: string) => void;
}

export interface UseSessionLifecycleActionsResult {
  actions: SessionLifecycleActionHandlers;
  batchResults: readonly SessionBatchResult[];
  deleteDialog: SessionDeleteConfirmation;
  processDialog: SessionProcessChoiceConfirmation;
  renameDialog: SessionRenameConfirmation;
  laterDialog: SessionLaterConfirmation;
}
