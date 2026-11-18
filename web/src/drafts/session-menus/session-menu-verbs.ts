/**
 * The shipped verbs of `session-menu-actions.ts`, mirrored for the draft. The shipped module reaches
 * `@tanstack/react-query` through `session-batch-feedback.ts` → `session-batch.ts` →
 * `query-options.ts`, which the prototype network guard rejects. Implementation keeps one owner:
 * move `batchSkipReason` into a pure module and import the shipped definitions directly.
 */
import {
  Archive,
  ArrowRightLeft,
  Bookmark,
  CheckCheck,
  Copy,
  CornerDownRight,
  CornerLeftUp,
  Eraser,
  GitFork,
  Hash,
  Pencil,
  RotateCcw,
  Square,
  Trash2,
  Undo2,
  UserRound,
  type LucideIcon,
} from "@compozy/ui/icons";

import { isSubagentSession } from "@/systems/session/lib/session-hierarchy";
import { sessionIsInLater } from "@/systems/session/lib/session-later";
import { sessionCanMakeTopLevel } from "@/systems/session/lib/session-reparent";
import type { SessionPayload } from "@/systems/session/types";

interface Verb {
  label: string;
  batchLabel?: string;
  icon: LucideIcon;
  eligible?: (session: SessionPayload) => boolean;
}

const live = (session: SessionPayload) =>
  session.state === "active" || session.state === "starting";
const archived = (session: SessionPayload) => session.archived_at !== null;

export const SESSION_MENU_VERBS = {
  rename: { label: "Rename session", icon: Pencil },
  "copy-id": { label: "Copy session ID", batchLabel: "Copy session IDs", icon: Hash },
  "copy-title": { label: "Copy title", batchLabel: "Copy titles", icon: Copy },
  continue: { label: "Continue with another agent…", icon: ArrowRightLeft },
  fork: { label: "Fork session…", icon: GitFork },
  "move-to-profile": { label: "Move to profile", icon: UserRound },
  nest: { label: "Nest under…", icon: CornerDownRight },
  "make-top-level": {
    label: "Make top-level",
    icon: CornerLeftUp,
    eligible: sessionCanMakeTopLevel,
  },
  "take-over": { label: "Take over session", icon: CornerLeftUp },
  "mark-seen": {
    label: "Mark as seen",
    icon: CheckCheck,
    eligible: session => session.attention_dismissible === true,
  },
  stop: {
    label: "Stop session",
    batchLabel: "Stop",
    icon: Square,
    eligible: session => live(session) && !archived(session),
  },
  later: {
    label: "Move to Later…",
    icon: Bookmark,
    eligible: session =>
      !isSubagentSession(session) && !archived(session) && !sessionIsInLater(session),
  },
  "back-from-later": {
    label: "Back from Later",
    icon: Undo2,
    eligible: session => !isSubagentSession(session) && sessionIsInLater(session),
  },
  archive: {
    label: "Archive session",
    batchLabel: "Archive",
    icon: Archive,
    eligible: session => !isSubagentSession(session) && !archived(session),
  },
  unarchive: {
    label: "Unarchive session",
    batchLabel: "Unarchive",
    icon: RotateCcw,
    eligible: session => !isSubagentSession(session) && archived(session),
  },
  clear: { label: "Clear conversation", icon: Eraser },
  delete: { label: "Delete session", batchLabel: "Delete", icon: Trash2, eligible: () => true },
} satisfies Record<string, Verb>;

export type SessionMenuVerb = keyof typeof SESSION_MENU_VERBS;

/** Row wording: a live session's archive names its stop-first consequence. */
export function rowVerb(id: SessionMenuVerb, session: SessionPayload) {
  const verb: Verb = SESSION_MENU_VERBS[id];
  const label =
    id === "archive" && session.state !== "stopped" ? "Stop and archive session" : verb.label;
  return { id, label, icon: verb.icon };
}

/** Selection wording and eligibility. */
export function batchVerb(id: SessionMenuVerb, liveArchive = false) {
  const verb: Verb = SESSION_MENU_VERBS[id];
  const label =
    id === "archive" && liveArchive ? "Stop and archive" : (verb.batchLabel ?? verb.label);
  return { id, label, icon: verb.icon, eligible: verb.eligible ?? (() => true) };
}
