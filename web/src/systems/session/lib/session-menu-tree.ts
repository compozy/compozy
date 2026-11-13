/**
 * One session's menu as one pure tree. A list row and a Kanban card render it on right-click and
 * behind `…`, so the four entry points cannot drift: a host changes only the facts and handlers it
 * passes. Frequent verbs stay at the top level, related verbs sit behind Copy ▸, New from this ▸,
 * the linked work submenu and Move ▸; lifecycle verbs follow, and Clear conversation and Delete
 * close the menu. A verb the session or host cannot take is absent, never disabled.
 */
import {
  ArrowDown,
  ArrowUp,
  Clipboard,
  CopyPlus,
  Folder,
  FolderInput,
  FolderOutput,
  GitBranch,
  Link2,
} from "@compozy/ui/icons";

import { tidyMenuTree, type MenuTreeNode } from "@compozy/ui";

import type { SessionFolderPayload, SessionPayload } from "../types";
import type { SessionCopyKind } from "./session-copy";
import { getSessionDisplayTitle } from "./session-display-title";
import { isSubagentSession, sessionTreeParentId } from "./session-hierarchy";
import { sessionIsInLater } from "./session-later";
import {
  sessionBatchAction,
  sessionRowAction,
  type SessionMenuActionId,
} from "./session-menu-actions";
import {
  linkedPullRequestUrls,
  sessionLinkedWorkNode,
  type SessionMenuLinkedWork,
} from "./session-menu-linked-work";
import { sessionCanTakeOver } from "./session-reparent";
import { isUserControllableSession } from "./session-running";

/** Folder and manual-order verbs a root session adds under Move ▸. */
export interface SessionRowLayoutMenu {
  folders: readonly SessionFolderPayload[];
  /** The folder holding the session, or null in the workspace root. */
  folderId: string | null;
  onMove: (folderId: string | null) => void;
  /** Present only under the list's Manual sort, where the row can move that way. */
  onMoveUp?: () => void;
  onMoveDown?: () => void;
}

type SessionVerb = (session: SessionPayload) => void;

/**
 * The session verbs the tree reads. The lifecycle hooks' handlers (`SessionLifecycleActionHandlers`)
 * satisfy it; an optional verb a host lacks is left out of the menu.
 */
export interface SessionMenuActions {
  /** Any action in flight disables every verb but Copy. */
  pendingAction: string | null;
  onRename: SessionVerb;
  onStop: SessionVerb;
  onArchive: SessionVerb;
  onUnarchive: SessionVerb;
  onDelete: SessionVerb;
  onMarkSeen?: SessionVerb;
  onMoveToLater?: SessionVerb;
  onBackFromLater?: SessionVerb;
  /** Make top-level and Take over both detach the session from its parent. */
  onMakeTopLevel?: SessionVerb;
}

export interface SessionMenuProfileTarget {
  id: string;
  name: string;
}

/** What the host knows beyond the session, and the handlers it can act with. */
export interface SessionMenuInput {
  session: SessionPayload;
  actions: SessionMenuActions;
  /** `session-row` behind `…`, `session-row-menu` on right-click; ids read `<prefix>-<verb>-<id>`. */
  testIdPrefix: string;
  onContinue?: (session: SessionPayload) => void;
  onFork?: (session: SessionPayload) => void;
  /** Other active profiles; Move to profile opens its dialog. */
  profileTargets?: readonly SessionMenuProfileTarget[];
  onMoveToProfile?: (session: SessionPayload, target: SessionMenuProfileTarget) => void;
  /** Opens the Move under… parent picker; absent where the host cannot move lineage. */
  onMoveUnder?: (session: SessionPayload) => void;
  /** Clear conversation, offered by the window that has this session open. */
  clear?: { canClear: boolean; request: () => void } | null;
  layout?: SessionRowLayoutMenu;
  linkedWork?: SessionMenuLinkedWork | null;
  onCopy: (text: string, kind: SessionCopyKind) => void;
}

type ItemNode = Extract<MenuTreeNode, { kind: "item" }>;

const separator = (id: string): MenuTreeNode => ({ kind: "separator", id });

function verbNode(
  input: SessionMenuInput,
  id: SessionMenuActionId,
  onSelect: () => void,
  extra: Partial<ItemNode> = {}
): ItemNode {
  const action = sessionRowAction(id, input.session);
  return {
    kind: "item",
    id,
    label: action.label,
    icon: action.icon,
    testId: testIdOf(input, id),
    disabled: input.actions.pendingAction !== null,
    onSelect,
    ...extra,
  };
}

function testIdOf(input: SessionMenuInput, id: string): string {
  return `${input.testIdPrefix}-${id}-${input.session.id}`;
}

function isTopLevel(session: SessionPayload): boolean {
  return !isSubagentSession(session) && sessionTreeParentId(session) === "";
}

/** Copy ▸ — every text the session can hand out. Copying never waits on a pending action. */
function copyNode(input: SessionMenuInput): MenuTreeNode {
  const { session, onCopy } = input;
  const branch = session.branch;
  const urls = linkedPullRequestUrls(input.linkedWork?.items ?? []);
  const copy = (id: SessionMenuActionId, text: string, kind: SessionCopyKind) =>
    verbNode(input, id, () => onCopy(text, kind), { disabled: false });
  return {
    kind: "sub",
    id: "copy",
    label: "Copy",
    icon: Clipboard,
    testId: testIdOf(input, "copy"),
    children: [
      copy("copy-id", session.id, "Session ID"),
      copy("copy-title", getSessionDisplayTitle(session), "Title"),
      separator("copy-work"),
      ...(branch
        ? [
            {
              kind: "item",
              id: "copy-branch",
              label: "Copy branch name",
              icon: GitBranch,
              testId: testIdOf(input, "copy-branch"),
              onSelect: () => onCopy(branch, "Branch name"),
            } as const,
          ]
        : []),
      ...(urls.length > 0
        ? [
            {
              kind: "item",
              id: "copy-pr",
              label: urls.length === 1 ? "Copy pull request link" : "Copy pull request links",
              icon: Link2,
              ...(urls.length > 1 ? { hint: String(urls.length) } : {}),
              testId: testIdOf(input, "copy-pr"),
              onSelect: () =>
                onCopy(
                  urls.join("\n"),
                  urls.length === 1 ? "Pull request link" : "Pull request links"
                ),
            } as const,
          ]
        : []),
    ],
  };
}

/** New from this ▸ — Continue and Fork, on live user sessions whose host can derive them. */
function deriveNode(input: SessionMenuInput): MenuTreeNode {
  const { session, onContinue, onFork } = input;
  const offered = isUserControllableSession(session) && session.archived_at === null;
  return {
    kind: "sub",
    id: "derive",
    label: "New from this",
    icon: CopyPlus,
    disabled: input.actions.pendingAction !== null,
    testId: testIdOf(input, "derive"),
    children: offered
      ? [
          ...(onContinue ? [verbNode(input, "continue", () => onContinue(session))] : []),
          ...(onFork ? [verbNode(input, "fork", () => onFork(session))] : []),
        ]
      : [],
  };
}

/** The Folder group: the current folder is checked, not hidden. */
function folderNodes(input: SessionMenuInput, layout: SessionRowLayoutMenu): MenuTreeNode[] {
  const { id } = input.session;
  const prefix = input.testIdPrefix;
  return [
    {
      kind: "group",
      id: "folder",
      label: "Folder",
      children: [
        ...layout.folders.map<MenuTreeNode>(folder => {
          const current = folder.id === layout.folderId;
          return {
            kind: "item",
            id: `folder-${folder.id}`,
            label: folder.name,
            icon: Folder,
            checked: current,
            disabled: current,
            testId: `${prefix}-move-${id}-${folder.id}`,
            onSelect: () => layout.onMove(folder.id),
          };
        }),
        ...(layout.folderId !== null
          ? [
              {
                kind: "item",
                id: "unfolder",
                label: "Remove from folder",
                icon: FolderOutput,
                testId: `${prefix}-unfolder-${id}`,
                onSelect: () => layout.onMove(null),
              } as const,
            ]
          : []),
      ],
    },
  ];
}

function orderNodes(input: SessionMenuInput, layout: SessionRowLayoutMenu): MenuTreeNode[] {
  const { onMoveUp, onMoveDown } = layout;
  if (onMoveUp === undefined && onMoveDown === undefined) return [];
  const { id } = input.session;
  const prefix = input.testIdPrefix;
  return [
    {
      kind: "item",
      id: "move-up",
      label: "Move up",
      icon: ArrowUp,
      disabled: onMoveUp === undefined,
      testId: `${prefix}-up-${id}`,
      ...(onMoveUp ? { onSelect: onMoveUp } : {}),
    },
    {
      kind: "item",
      id: "move-down",
      label: "Move down",
      icon: ArrowDown,
      disabled: onMoveDown === undefined,
      testId: `${prefix}-down-${id}`,
      ...(onMoveDown ? { onSelect: onMoveDown } : {}),
    },
  ];
}

/** Move ▸ — where the session lives: folder, thread, profile and manual order. */
function moveNode(input: SessionMenuInput): MenuTreeNode {
  const { session, actions, layout } = input;
  const busy = actions.pendingAction !== null;
  const { onMakeTopLevel } = actions;
  const { onMoveUnder } = input;
  const live = isUserControllableSession(session) && session.archived_at === null;
  const targets = (input.profileTargets ?? []).filter(target => target.id !== session.profile_id);
  const { onMoveToProfile } = input;
  const profile = sessionRowAction("move-to-profile", session);
  return {
    kind: "sub",
    id: "move",
    label: "Move",
    icon: FolderInput,
    disabled: busy,
    testId: testIdOf(input, "move"),
    children: [
      // The host hands a layout only to a row that owns its placement (a list root, a card's
      // own read), so its presence is the gate: a row whose parent is off the page still files.
      ...(layout ? folderNodes(input, layout) : []),
      separator("move-thread"),
      // The picker says why a continued, forked or nested session cannot move, and offers Make
      // top-level where that applies, so the menu does not filter the session's parents.
      ...(onMoveUnder && live && !isSubagentSession(session)
        ? [verbNode(input, "move-under", () => onMoveUnder(session))]
        : []),
      ...(onMakeTopLevel && sessionBatchAction("make-top-level").eligible(session)
        ? [verbNode(input, "make-top-level", () => onMakeTopLevel(session))]
        : []),
      ...(onMakeTopLevel && sessionCanTakeOver(session)
        ? [verbNode(input, "take-over", () => onMakeTopLevel(session))]
        : []),
      separator("move-profile"),
      ...(onMoveToProfile && live && isTopLevel(session) && targets.length > 0
        ? [
            {
              kind: "sub",
              id: "move-to-profile",
              label: profile.label,
              icon: profile.icon,
              testId: testIdOf(input, "move-to-profile"),
              children: targets.map<MenuTreeNode>(target => ({
                kind: "item",
                id: `profile-${target.id}`,
                label: target.name,
                testId: `${testIdOf(input, "move-to-profile")}-${target.id}`,
                onSelect: () => onMoveToProfile(session, target),
              })),
            } as const,
          ]
        : []),
      separator("move-order"),
      ...(layout ? orderNodes(input, layout) : []),
    ],
  };
}

/** Stop, Move to Later or Back from Later, and Archive or Unarchive, gated on the session's state. */
function lifecycleNodes(input: SessionMenuInput): MenuTreeNode[] {
  const { session, actions } = input;
  const archived = session.archived_at !== null;
  const subagent = isSubagentSession(session);
  const live = session.state === "active" || session.state === "starting";
  const { onMoveToLater, onBackFromLater } = actions;
  const nodes: MenuTreeNode[] = [];
  if (!archived && live) nodes.push(verbNode(input, "stop", () => actions.onStop(session)));
  // A subagent follows its parent and an archived session has left the flow: no Later verb.
  if (!subagent && !archived) {
    if (sessionIsInLater(session)) {
      if (onBackFromLater) {
        nodes.push(verbNode(input, "back-from-later", () => onBackFromLater(session)));
      }
    } else if (onMoveToLater) {
      nodes.push(verbNode(input, "later", () => onMoveToLater(session)));
    }
  }
  // A subagent session archives with its parent (ADR-005): no standalone verb.
  if (!subagent) {
    nodes.push(
      archived
        ? verbNode(input, "unarchive", () => actions.onUnarchive(session))
        : verbNode(input, "archive", () => actions.onArchive(session))
    );
  }
  return nodes;
}

/** One session's menu: the same tree on a list row and a board card, `…` or right-click. */
export function sessionMenuTree(input: SessionMenuInput): MenuTreeNode[] {
  const { session, actions, clear, linkedWork } = input;
  const user = isUserControllableSession(session);
  const { onMarkSeen } = actions;
  const linked = linkedWork
    ? sessionLinkedWorkNode(linkedWork, {
        disabled: actions.pendingAction !== null,
        onCopy: input.onCopy,
        testId: id => testIdOf(input, id),
      })
    : null;
  return tidyMenuTree([
    ...(user ? [verbNode(input, "rename", () => actions.onRename(session))] : []),
    copyNode(input),
    deriveNode(input),
    ...(linked ? [linked] : []),
    separator("organize"),
    moveNode(input),
    ...(onMarkSeen && sessionBatchAction("mark-seen").eligible(session)
      ? [verbNode(input, "mark-seen", () => onMarkSeen(session))]
      : []),
    separator("lifecycle"),
    ...lifecycleNodes(input),
    separator("danger"),
    ...(user && clear
      ? [
          verbNode(input, "clear", clear.request, {
            disabled: actions.pendingAction !== null || !clear.canClear,
          }),
        ]
      : []),
    verbNode(input, "delete", () => actions.onDelete(session), { destructive: true }),
  ]);
}
