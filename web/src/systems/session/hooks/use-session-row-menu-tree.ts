import type { MenuTreeNode } from "@compozy/ui";
import { use } from "react";

import { SessionClearContext } from "../contexts/session-clear-context-value";
import { SessionDeriveContext } from "../contexts/session-derive-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import type { SessionMenuSourceData } from "../contexts/session-menu-source-context-value";
import { SessionProfileMoveContext } from "../contexts/session-profile-move-context-value";
import { copySessionText } from "../lib/session-copy";
import { sessionMenuTree, type SessionRowLayoutMenu } from "../lib/session-menu-tree";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./session-lifecycle-types";
import { requestSessionLinkDialog } from "./use-session-link-dialog";

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

/**
 * One session's menu tree for its row or card, `…` or right-click: the session's verbs, plus the
 * Continue, Fork, Move to profile and Clear conversation entries the surrounding host provides.
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
  const { onOpen } = options;
  const work = options.linkedWork ?? summaryLinkedWork(session);
  return sessionMenuTree({
    session,
    actions,
    testIdPrefix: options.testIdPrefix,
    ...(requestContinue ? { onContinue: (source: SessionPayload) => requestContinue(source) } : {}),
    ...(requestFork ? { onFork: (source: SessionPayload) => requestFork(source) } : {}),
    ...(profileMove
      ? { profileTargets: profileMove.targets, onMoveToProfile: profileMove.request }
      : {}),
    clear: clear?.sessionId === session.id ? clear : null,
    ...(options.layout ? { layout: options.layout } : {}),
    linkedWork: work
      ? {
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
        }
      : null,
    onCopy: (text, kind) => void copySessionText(text, kind),
  });
}
