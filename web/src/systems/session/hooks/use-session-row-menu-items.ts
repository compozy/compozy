import { Archive, ArrowRightLeft, GitFork, Pencil, RotateCcw, Square, Trash2 } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { use } from "react";

import { SessionDeriveContext } from "../contexts/session-derive-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { isSubagentSession } from "../lib/session-hierarchy";
import { isUserControllableSession } from "../lib/session-running";
import type { SessionPayload } from "../types";

/** One lifecycle verb shared by the row's `…` menu and its context menu. */
export interface SessionRowMenuItem {
  /** Stable verb id; test ids read `session-row-<id>-<session id>`. */
  id: "rename" | "continue" | "fork" | "stop" | "unarchive" | "archive" | "delete";
  label: string;
  icon: LucideIcon;
  destructive?: boolean;
  onSelect: () => void;
}

function isStopEligible(session: SessionPayload): boolean {
  return session.state === "active" || session.state === "starting";
}

/** State-gated lifecycle verbs for one session row, in menu order. */
export function useSessionRowMenuItems(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers
): SessionRowMenuItem[] {
  const requestContinue = use(SessionDeriveContext);
  const requestFork = use(SessionForkContext);
  const isArchived = session.archived_at !== null;
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
  if (userSession && !isArchived && requestContinue) {
    items.push({
      id: "continue",
      label: "Continue with another agent…",
      icon: ArrowRightLeft,
      onSelect: () => requestContinue(session),
    });
  }
  if (userSession && !isArchived && requestFork) {
    items.push({
      id: "fork",
      label: "Fork session…",
      icon: GitFork,
      onSelect: () => requestFork(session),
    });
  }
  if (!isArchived && isStopEligible(session)) {
    items.push({
      id: "stop",
      label: "Stop session",
      icon: Square,
      onSelect: () => actions.onStop(session),
    });
  }
  // A subagent session archives with its parent (ADR-005): no standalone verb.
  if (!isSubagentSession(session)) {
    if (isArchived) {
      items.push({
        id: "unarchive",
        label: "Unarchive session",
        icon: RotateCcw,
        onSelect: () => actions.onUnarchive(session),
      });
    } else if (session.state === "stopped") {
      items.push({
        id: "archive",
        label: "Archive session",
        icon: Archive,
        onSelect: () => actions.onArchive(session),
      });
    }
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
