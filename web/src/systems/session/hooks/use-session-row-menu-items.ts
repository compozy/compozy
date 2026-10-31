import {
  Archive,
  ArrowRightLeft,
  Clock,
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
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { use } from "react";

import { SessionClearContext } from "../contexts/session-clear-context-value";
import {
  SessionDeriveContext,
  type SessionContinueRequest,
} from "../contexts/session-derive-context-value";
import {
  SessionForkContext,
  type SessionForkRequest,
} from "../contexts/session-fork-context-value";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { copySessionText } from "../lib/session-copy";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { isSubagentSession } from "../lib/session-hierarchy";
import { sessionIsInLater } from "../lib/session-later";
import { sessionAdoptRefusal, sessionCanMakeTopLevel } from "../lib/session-reparent";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

/** One lifecycle verb shared by the row's `…` menu and its context menu. */
export interface SessionRowMenuItem {
  /** Stable verb id; test ids read `session-row-<id>-<session id>`. */
  id:
    | "rename"
    | "copy-id"
    | "copy-title"
    | "continue"
    | "fork"
    | "make-top-level"
    | "nest"
    | "stop"
    | "unarchive"
    | "later"
    | "back-from-later"
    | "archive"
    | "clear"
    | "delete";
  label: string;
  icon: LucideIcon;
  destructive?: boolean;
  /** Shown but unavailable right now, independent of a pending lifecycle action. */
  disabled?: boolean;
  onSelect?: () => void;
  choices?: Array<{ id: string; label: string; onSelect: () => void }>;
}

function isStopEligible(session: SessionPayload): boolean {
  return session.state === "active" || session.state === "starting";
}

/** Archive or restore; a live session stops first, which cancels any active turn. */
function sessionArchiveMenuItem(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem | null {
  // A subagent session archives with its parent (ADR-005): no standalone verb.
  if (isSubagentSession(session)) return null;
  if (session.archived_at !== null) {
    return {
      id: "unarchive",
      label: "Unarchive session",
      icon: RotateCcw,
      onSelect: () => actions.onUnarchive(session),
    };
  }
  return {
    id: "archive",
    label: session.state === "stopped" ? "Archive session" : "Stop and archive session",
    icon: Archive,
    onSelect: () => actions.onArchive(session),
  };
}

/**
 * Move to Later or Back from Later. A subagent session follows its parent, and
 * an archived session has left the flow already: neither gets a verb. Hosts
 * without a Later dialog leave the handlers out, and the verbs with them.
 */
function sessionLaterMenuItem(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem | null {
  if (isSubagentSession(session) || session.archived_at !== null) return null;
  const { onMoveToLater, onBackFromLater } = actions;
  if (sessionIsInLater(session)) {
    return onBackFromLater
      ? {
          id: "back-from-later",
          label: "Back from Later",
          icon: Undo2,
          onSelect: () => onBackFromLater(session),
        }
      : null;
  }
  return onMoveToLater
    ? { id: "later", label: "Move to Later…", icon: Clock, onSelect: () => onMoveToLater(session) }
    : null;
}

/** Continue and Fork, offered on live user sessions whose host can derive them. */
function sessionDeriveMenuItems(
  session: SessionPayload,
  requestContinue: SessionContinueRequest | null,
  requestFork: SessionForkRequest | null
): SessionRowMenuItem[] {
  if (!isUserControllableSession(session) || session.archived_at !== null) return [];
  const items: SessionRowMenuItem[] = [];
  if (requestContinue) {
    items.push({
      id: "continue",
      label: "Continue with another agent…",
      icon: ArrowRightLeft,
      onSelect: () => requestContinue(session),
    });
  }
  if (requestFork) {
    items.push({
      id: "fork",
      label: "Fork session…",
      icon: GitFork,
      onSelect: () => requestFork(session),
    });
  }
  return items;
}

/** Stop, Later, and Archive verbs gated on the session's lifecycle state. */
function sessionStateMenuItems(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem[] {
  const items: SessionRowMenuItem[] = [];
  if (session.archived_at === null && isStopEligible(session)) {
    items.push({
      id: "stop",
      label: "Stop session",
      icon: Square,
      onSelect: () => actions.onStop(session),
    });
  }
  const laterItem = sessionLaterMenuItem(session, actions);
  if (laterItem) items.push(laterItem);
  const archiveItem = sessionArchiveMenuItem(session, actions);
  if (archiveItem) items.push(archiveItem);
  return items;
}

/** State-gated lifecycle verbs for one session row, in menu order. */
export function useSessionRowMenuItems(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem[] {
  const requestContinue = use(SessionDeriveContext);
  const requestFork = use(SessionForkContext);
  const clear = use(SessionClearContext);
  const userSession = isUserControllableSession(session);
  const items: SessionRowMenuItem[] = [];
  if (userSession) {
    items.push({
      id: "rename",
      label: "Rename session",
      icon: Pencil,
      onSelect: () => actions.onRename(session),
    });
  }
  items.push(
    {
      id: "copy-id",
      label: "Copy session ID",
      icon: Hash,
      onSelect: () => void copySessionText(session.id, "Session ID"),
    },
    {
      id: "copy-title",
      label: "Copy title",
      icon: Copy,
      onSelect: () => void copySessionText(getSessionDisplayTitle(session), "Title"),
    }
  );
  items.push(...sessionDeriveMenuItems(session, requestContinue, requestFork));
  items.push(...sessionNestMenuItems(session, actions));
  const { onMakeTopLevel } = actions;
  if (onMakeTopLevel && sessionCanMakeTopLevel(session)) {
    items.push({
      id: "make-top-level",
      label: "Make top-level",
      icon: CornerLeftUp,
      onSelect: () => onMakeTopLevel(session),
    });
  }
  items.push(...sessionStateMenuItems(session, actions));
  if (userSession && clear?.sessionId === session.id) {
    items.push({
      id: "clear",
      label: "Clear conversation",
      icon: Eraser,
      disabled: !clear.canClear,
      onSelect: clear.request,
    });
  }
  items.push({
    id: "delete",
    label: "Delete session",
    icon: Trash2,
    destructive: true,
    onSelect: () => actions.onDelete(session),
  });
  return items;
}

function sessionNestMenuItems(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem[] {
  const items: SessionRowMenuItem[] = [];
  const { onAdopt } = actions;
  const parents = onAdopt
    ? (actions.nestParents ?? []).filter(parent => sessionAdoptRefusal(session, parent) === null)
    : [];
  if (onAdopt && parents.length > 0) {
    items.push({
      id: "nest",
      label: "Nest under…",
      icon: CornerDownRight,
      choices: parents.map(parent => ({
        id: parent.id,
        label: getSessionDisplayTitle(parent),
        onSelect: () => onAdopt(session, parent),
      })),
    });
  }
  return items;
}
