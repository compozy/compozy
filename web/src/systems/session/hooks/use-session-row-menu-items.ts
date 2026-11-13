import {
  sessionRowAction,
  sessionBatchAction,
  type SessionMenuActionId,
} from "../lib/session-menu-actions";
import type { LucideIcon } from "@compozy/ui/icons";
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
import {
  SessionProfileMoveContext,
  type SessionProfileMoveHandle,
} from "../contexts/session-profile-move-context-value";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { copySessionText } from "../lib/session-copy";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { isSubagentSession, sessionTreeParentId } from "../lib/session-hierarchy";
import { sessionIsInLater } from "../lib/session-later";
import { sessionCanTakeOver } from "../lib/session-reparent";
import { requestSessionParentPicker } from "./use-session-parent-picker";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

/** One lifecycle verb shared by the row's `…` menu and its context menu. */
export interface SessionRowMenuItem {
  /** Stable verb id; test ids read `session-row-<id>-<session id>`. */
  id: SessionMenuActionId;
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
      ...sessionRowAction("unarchive", session),
      onSelect: () => actions.onUnarchive(session),
    };
  }
  return {
    ...sessionRowAction("archive", session),
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
          ...sessionRowAction("back-from-later", session),
          onSelect: () => onBackFromLater(session),
        }
      : null;
  }
  return onMoveToLater
    ? {
        ...sessionRowAction("later", session),
        onSelect: () => onMoveToLater(session),
      }
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
      ...sessionRowAction("continue", session),
      onSelect: () => requestContinue(session),
    });
  }
  if (requestFork) {
    items.push({
      ...sessionRowAction("fork", session),
      onSelect: () => requestFork(session),
    });
  }
  return items;
}

/**
 * Move to profile, offered on top-level user sessions with another active profile
 * to go to. The dialog states why a running session or one with children cannot move.
 */
function sessionProfileMoveMenuItem(
  session: SessionPayload,
  profileMove: SessionProfileMoveHandle | null
): SessionRowMenuItem | null {
  if (!profileMove || !isUserControllableSession(session) || session.archived_at !== null) {
    return null;
  }
  if (isSubagentSession(session) || sessionTreeParentId(session) !== "") return null;
  const targets = profileMove.targets.filter(target => target.id !== session.profile_id);
  if (targets.length === 0) return null;
  return {
    ...sessionRowAction("move-to-profile", session),
    choices: targets.map(target => ({
      id: target.id,
      label: target.name,
      onSelect: () => profileMove.request(session, target),
    })),
  };
}

/** Mark as seen, Stop, Later, and Archive verbs gated on the session's state. */
function sessionStateMenuItems(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem[] {
  const items: SessionRowMenuItem[] = [];
  const { onMarkSeen } = actions;
  // Only a Needs you the daemon can clear: a pending answer, sign-in, or stop stays.
  if (onMarkSeen && sessionBatchAction("mark-seen").eligible(session)) {
    items.push({
      ...sessionRowAction("mark-seen", session),
      onSelect: () => onMarkSeen(session),
    });
  }
  if (session.archived_at === null && isStopEligible(session)) {
    items.push({
      ...sessionRowAction("stop", session),
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
  const profileMove = use(SessionProfileMoveContext);
  const userSession = isUserControllableSession(session);
  const items: SessionRowMenuItem[] = [];
  if (userSession) {
    items.push({
      ...sessionRowAction("rename", session),
      onSelect: () => actions.onRename(session),
    });
  }
  items.push(
    {
      ...sessionRowAction("copy-id", session),
      onSelect: () => void copySessionText(session.id, "Session ID"),
    },
    {
      ...sessionRowAction("copy-title", session),
      onSelect: () => void copySessionText(getSessionDisplayTitle(session), "Title"),
    }
  );
  items.push(...sessionDeriveMenuItems(session, requestContinue, requestFork));
  const profileMoveItem = sessionProfileMoveMenuItem(session, profileMove);
  if (profileMoveItem) items.push(profileMoveItem);
  const moveUnderItem = sessionMoveUnderMenuItem(session, actions);
  if (moveUnderItem) items.push(moveUnderItem);
  const { onMakeTopLevel } = actions;
  if (onMakeTopLevel && sessionBatchAction("make-top-level").eligible(session)) {
    items.push({
      ...sessionRowAction("make-top-level", session),
      onSelect: () => onMakeTopLevel(session),
    });
  }
  if (onMakeTopLevel && sessionCanTakeOver(session)) {
    items.push({
      ...sessionRowAction("take-over", session),
      onSelect: () => onMakeTopLevel(session),
    });
  }
  items.push(...sessionStateMenuItems(session, actions));
  if (userSession && clear?.sessionId === session.id) {
    items.push({
      ...sessionRowAction("clear", session),
      disabled: !clear.canClear,
      onSelect: clear.request,
    });
  }
  items.push({
    ...sessionRowAction("delete", session),
    destructive: true,
    onSelect: () => actions.onDelete(session),
  });
  return items;
}

/**
 * Move under…, offered on the host's own unarchived user sessions when it can
 * move lineage. The picker states why a continued, forked or nested session
 * cannot move, and offers Make top-level where that applies.
 */
function sessionMoveUnderMenuItem(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem | null {
  if (!actions.onAdopt || !isUserControllableSession(session)) return null;
  if (isSubagentSession(session) || session.archived_at !== null) return null;
  return {
    ...sessionRowAction("move-under", session),
    onSelect: () => requestSessionParentPicker(session.id),
  };
}
