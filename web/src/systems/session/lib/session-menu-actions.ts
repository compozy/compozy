import {
  Archive,
  ArrowRightLeft,
  Bookmark,
  CheckCheck,
  Copy,
  CornerLeftUp,
  CornerDownRight,
  Eraser,
  GitFork,
  Hash,
  Pencil,
  RotateCcw,
  Square,
  Trash2,
  Undo2,
  UserRound,
} from "@compozy/ui/icons";
import type { SessionPayload } from "@/systems/session/types";
import { batchSkipReason } from "@/systems/session/lib/session-batch-feedback";
import { sessionCanMakeTopLevel } from "@/systems/session/lib/session-reparent";

/** Labels, icons and batch eligibility have one owner for row and selection menus. */
export const SESSION_MENU_ACTIONS = {
  rename: { label: "Rename session", icon: Pencil },
  "copy-id": {
    label: "Copy session ID",
    batchLabel: "Copy session IDs",
    icon: Hash,
    eligible: () => true,
  },
  "copy-title": {
    label: "Copy title",
    batchLabel: "Copy titles",
    icon: Copy,
    eligible: () => true,
  },
  continue: { label: "Continue with another agent…", icon: ArrowRightLeft },
  fork: { label: "Fork session…", icon: GitFork },
  "move-to-profile": { label: "Move to profile", icon: UserRound },
  "move-under": { label: "Move under…", icon: CornerDownRight },
  "make-top-level": {
    label: "Make top-level",
    icon: CornerLeftUp,
    eligible: sessionCanMakeTopLevel,
  },
  "take-over": { label: "Take over session", icon: CornerLeftUp },
  "mark-seen": {
    label: "Mark as seen",
    icon: CheckCheck,
    eligible: (session: SessionPayload) => session.attention_dismissible === true,
  },
  stop: {
    label: "Stop session",
    batchLabel: "Stop",
    icon: Square,
    eligible: (session: SessionPayload) => batchSkipReason("stop", session) === null,
  },
  later: {
    label: "Move to Later…",
    icon: Bookmark,
    eligible: (session: SessionPayload) => batchSkipReason("later", session) === null,
  },
  "back-from-later": {
    label: "Back from Later",
    icon: Undo2,
    eligible: (session: SessionPayload) => batchSkipReason("back-from-later", session) === null,
  },
  archive: {
    label: "Archive session",
    batchLabel: "Archive",
    icon: Archive,
    eligible: (session: SessionPayload) => batchSkipReason("archive", session) === null,
  },
  unarchive: {
    label: "Unarchive session",
    batchLabel: "Unarchive",
    icon: RotateCcw,
    eligible: (session: SessionPayload) => batchSkipReason("unarchive", session) === null,
  },
  clear: { label: "Clear conversation", icon: Eraser },
  delete: { label: "Delete session", batchLabel: "Delete", icon: Trash2, eligible: () => true },
} as const;

export type SessionMenuActionId = keyof typeof SESSION_MENU_ACTIONS;
export type SessionBatchMenuActionId = {
  [Id in SessionMenuActionId]: (typeof SESSION_MENU_ACTIONS)[Id] extends { eligible: unknown }
    ? Id
    : never;
}[SessionMenuActionId];

/** Row wording names the stop-first archive consequence for live sessions. */
export function sessionRowAction(id: SessionMenuActionId, session: SessionPayload) {
  const definition = SESSION_MENU_ACTIONS[id];
  return {
    id,
    label:
      id === "archive" && session.state !== "stopped"
        ? "Stop and archive session"
        : definition.label,
    icon: definition.icon,
  };
}

/** Selection wording and eligibility come from the same definitions as row actions. */
export function sessionBatchAction(id: SessionBatchMenuActionId, liveArchive = false) {
  const definition = SESSION_MENU_ACTIONS[id];
  return {
    id,
    label:
      id === "archive" && liveArchive
        ? "Stop and archive"
        : "batchLabel" in definition
          ? definition.batchLabel
          : definition.label,
    icon: definition.icon,
    eligible: definition.eligible,
  };
}
