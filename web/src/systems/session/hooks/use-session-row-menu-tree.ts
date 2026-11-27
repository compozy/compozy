import type { MenuTreeNode } from "@compozy/ui";
import { use } from "react";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";

import { SessionClearContext } from "../contexts/session-clear-context-value";
import { SessionDeriveContext } from "../contexts/session-derive-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import { SessionMenuOpenContext } from "../contexts/session-menu-open-context-value";
import type { SessionMenuSourceData } from "../contexts/session-menu-source-context-value";
import { SessionProfileMoveContext } from "../contexts/session-profile-move-context-value";
import { copySessionText } from "../lib/session-copy";
import type { SessionMenuLinkedWork } from "../lib/session-menu-linked-work";
import { sessionMenuTree, type SessionRowLayoutMenu } from "../lib/session-menu-tree";
import { sessionComputerOf } from "../lib/session-row-ref";
import type { SessionListTarget, SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./session-lifecycle-types";
import { requestSessionLinkDialog } from "./use-session-link-dialog";
import { requestSessionParentPicker } from "./use-session-parent-picker";

export interface SessionRowMenuOptions {
  /** `session-row` behind `…`, `session-row-menu` on right-click. */
  testIdPrefix: string;
  /** Folder moves and manual order; a list root row or a card's read layout. */
  layout?: SessionRowLayoutMenu;
  /** The full linked work once read; absent, the catalog summary's lead stands in. */
  linkedWork?: SessionMenuSourceData["linkedWork"];
  /** Opens this session: Link another… and Show all need its window. */
  onOpen?: () => void;
}

/** The catalog row's own linked work: its summary's lead, before the full list is read. */
function summaryLinkedWork(session: SessionPayload): SessionMenuSourceData["linkedWork"] {
  const summary = session.linked_work;
  return summary?.lead ? { items: [summary.lead], total: summary.total, refreshing: false } : null;
}

function linkedWorkMenu(
  session: SessionPayload,
  options: SessionRowMenuOptions,
  openSession: ((target: SessionListTarget) => void) | null,
  computerId: ComputerId
): SessionMenuLinkedWork | null {
  const work = options.linkedWork ?? summaryLinkedWork(session);
  if (!work) return null;
  const { onOpen } = options;
  const remote = isRemoteComputerId(computerId);
  return {
    ...work,
    ...(onOpen
      ? {
          // The link dialog lives in the session's window: open it, then ask for the dialog.
          onLink: () => {
            onOpen();
            requestSessionLinkDialog(session.id);
          },
          onShowAll: onOpen,
        }
      : {}),
    // A rolled-up item's child shares the session's project; its agent resolves on open.
    ...(openSession
      ? {
          onOpenSession: (id: string) =>
            openSession({
              id,
              agent_name: "",
              workspace_id: session.workspace_id,
              ...(remote ? { computerId } : {}),
            }),
        }
      : {}),
  };
}

/**
 * One session's menu tree for its row or card, `…` or right-click: the session's verbs, plus the
 * Continue, Fork, Move to profile and Clear conversation entries the surrounding host provides,
 * and Move under… where the host can move lineage.
 */
export function useSessionRowMenuTree(
  session: SessionPayload,
  actions: SessionLifecycleActionHandlers,
  options: SessionRowMenuOptions
): MenuTreeNode[] {
  const requestContinue = use(SessionDeriveContext);
  const requestFork = use(SessionForkContext);
  const clear = use(SessionClearContext);
  const profileMove = use(SessionProfileMoveContext);
  const openSession = use(SessionMenuOpenContext);
  const computerId = sessionComputerOf(session);
  const remote = isRemoteComputerId(computerId);
  return sessionMenuTree({
    session,
    actions,
    testIdPrefix: options.testIdPrefix,
    ...(!remote && requestContinue
      ? { onContinue: (source: SessionPayload) => requestContinue(source) }
      : {}),
    ...(!remote && requestFork ? { onFork: (source: SessionPayload) => requestFork(source) } : {}),
    ...(!remote && profileMove
      ? { profileTargets: profileMove.targets, onMoveToProfile: profileMove.request }
      : {}),
    clear: !remote && clear?.sessionId === session.id ? clear : null,
    // One picker, hosted by the shell, serves local rows, cards and the palette.
    ...(!remote && actions.onAdopt
      ? { onMoveUnder: (source: SessionPayload) => requestSessionParentPicker(source.id) }
      : {}),
    ...(options.layout ? { layout: options.layout } : {}),
    linkedWork: linkedWorkMenu(session, options, openSession, computerId),
    onCopy: (text, kind) => void copySessionText(text, kind),
  });
}
