/**
 * The grouped session menu as one pure tree (draft of `session-menu-actions.ts`'s successor).
 * A row's right-click menu, its `…` menu, a board card's menus and both selection menus render
 * this tree, so the surfaces cannot drift: each surface only changes the facts it passes in.
 * Labels, icons and eligibility mirror the shipped definitions (`session-menu-verbs.ts`).
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

import { getSessionDisplayTitle } from "@/systems/session/lib/session-display-title";
import { sessionCountLabel } from "@/systems/session/lib/session-folders";
import { isSubagentSession, sessionTreeParentId } from "@/systems/session/lib/session-hierarchy";
import { sessionIsInLater } from "@/systems/session/lib/session-later";
import { sessionAdoptRefusal, sessionCanTakeOver } from "@/systems/session/lib/session-reparent";
import type { MenuTreeNode } from "@compozy/ui";

import { isUserControllableSession } from "@/systems/session/lib/session-running";
import type { SessionLinkedWorkItem } from "@/systems/session/lib/session-linked-work";
import type { SessionPayload } from "@/systems/session/types";

import { linkedPullRequestCount, linkedWorkNode } from "./session-menu-linked-work";
import { batchVerb, rowVerb, type SessionMenuVerb } from "./session-menu-verbs";

export type SessionMenuNode = MenuTreeNode;

export interface SessionMenuFolder {
  id: string;
  name: string;
}

/** What the host knows beyond the session: each surface passes only what it can act on. */
export interface SessionMenuFacts {
  folders: readonly SessionMenuFolder[];
  /** The session's folder; null in the project root. */
  folderId: string | null;
  /** Other active profiles. */
  profiles: readonly SessionMenuFolder[];
  /** Candidate parents for Nest under. */
  nestParents: readonly SessionPayload[];
  /** Manual sort in the list only; the board orders by column. */
  order?: { canMoveUp: boolean; canMoveDown: boolean };
  /** Clear conversation exists only for the open session. */
  clearable?: boolean;
  /** The full linked work, loaded when its submenu opens; absent, the row's own PR stands in. */
  linkedWork?: readonly SessionLinkedWorkItem[];
}

const separator = (id: string): SessionMenuNode => ({ kind: "separator", id });

function item(
  id: SessionMenuVerb,
  session: SessionPayload,
  extra: Partial<Extract<SessionMenuNode, { kind: "item" }>> = {}
): SessionMenuNode {
  const action = rowVerb(id, session);
  return { kind: "item", id, label: action.label, icon: action.icon, ...extra };
}

/** Drops leading, trailing and doubled separators left by absent verbs. */
function tidy(nodes: SessionMenuNode[]): SessionMenuNode[] {
  const out: SessionMenuNode[] = [];
  for (const node of nodes) {
    if (node.kind === "separator" && (out.length === 0 || out.at(-1)?.kind === "separator")) {
      continue;
    }
    out.push(node);
  }
  while (out.at(-1)?.kind === "separator") out.pop();
  return out;
}

/** Folder targets, shared by the row and the selection: the current one is checked, not hidden. */
function folderNodes(facts: SessionMenuFacts, removable: boolean): SessionMenuNode[] {
  if (facts.folders.length === 0) return [];
  const children: SessionMenuNode[] = [
    ...facts.folders.map<SessionMenuNode>(folder => ({
      kind: "item",
      id: `folder-${folder.id}`,
      label: folder.name,
      icon: Folder,
      checked: folder.id === facts.folderId,
      disabled: folder.id === facts.folderId,
    })),
    ...(removable
      ? [{ kind: "item", id: "unfolder", label: "Remove from folder", icon: FolderOutput } as const]
      : []),
  ];
  return [{ kind: "group", id: "folder", label: "Folder", children }];
}

function parentNodes(parents: readonly SessionPayload[]): SessionMenuNode[] {
  return parents.map(parent => ({
    kind: "item",
    id: `nest-${parent.id}`,
    label: getSessionDisplayTitle(parent),
  }));
}

function profileNodes(facts: Pick<SessionMenuFacts, "profiles">): SessionMenuNode[] {
  return facts.profiles.map(profile => ({
    kind: "item",
    id: `profile-${profile.id}`,
    label: profile.name,
  }));
}

/** Move ▸ — where the session lives: folder, thread, profile and manual order. */
function sessionMoveNode(session: SessionPayload, facts: SessionMenuFacts): SessionMenuNode | null {
  const topLevel = !isSubagentSession(session) && sessionTreeParentId(session) === "";
  const parents = facts.nestParents.filter(parent => sessionAdoptRefusal(session, parent) === null);
  const live = session.archived_at === null && isUserControllableSession(session);
  const children = tidy([
    ...(topLevel ? folderNodes(facts, facts.folderId !== null) : []),
    separator("thread"),
    ...(parents.length > 0
      ? [{ kind: "sub", ...pick("nest", session), children: parentNodes(parents) } as const]
      : []),
    ...(batchVerb("make-top-level").eligible(session) ? [item("make-top-level", session)] : []),
    ...(sessionCanTakeOver(session) ? [item("take-over", session)] : []),
    separator("profile"),
    ...(live && topLevel && facts.profiles.length > 0
      ? [
          {
            kind: "sub",
            ...pick("move-to-profile", session),
            children: profileNodes(facts),
          } as const,
        ]
      : []),
    separator("order"),
    ...(facts.order
      ? [
          {
            kind: "item",
            id: "move-up",
            label: "Move up",
            icon: ArrowUp,
            disabled: !facts.order.canMoveUp,
          } as const,
          {
            kind: "item",
            id: "move-down",
            label: "Move down",
            icon: ArrowDown,
            disabled: !facts.order.canMoveDown,
          } as const,
        ]
      : []),
  ]);
  return children.length > 0
    ? { kind: "sub", id: "move", label: "Move", icon: FolderInput, children }
    : null;
}

function pick(id: SessionMenuVerb, session: SessionPayload) {
  const action = rowVerb(id, session);
  return { id, label: action.label, icon: action.icon };
}

/** Copy ▸ — every text the session can hand out. */
function sessionCopyNode(
  session: SessionPayload,
  links: readonly SessionLinkedWorkItem[]
): SessionMenuNode {
  const prs = linkedPullRequestCount(links);
  return {
    kind: "sub",
    id: "copy",
    label: "Copy",
    icon: Clipboard,
    children: tidy([
      item("copy-id", session),
      item("copy-title", session),
      separator("work"),
      ...(session.branch
        ? [{ kind: "item", id: "copy-branch", label: "Copy branch name", icon: GitBranch } as const]
        : []),
      ...(prs === 1
        ? [{ kind: "item", id: "copy-pr", label: "Copy pull request link", icon: Link2 } as const]
        : []),
      ...(prs > 1
        ? [
            {
              kind: "item",
              id: "copy-pr",
              label: "Copy pull request links",
              icon: Link2,
              hint: String(prs),
            } as const,
          ]
        : []),
    ]),
  };
}

/** The row's own pull request as a linked item, before the full list loads. */
function sessionOwnLinks(session: SessionPayload): SessionLinkedWorkItem[] {
  const pr = session.pull_request;
  if (!pr || !session.branch) return [];
  return [
    {
      repository: "",
      branch: session.branch,
      provenance: "created",
      runs_here: true,
      worktree_removed: false,
      pr_number: pr.number,
      pull_request: pr,
      pull_request_lookup: session.pull_request_lookup ?? null,
    },
  ];
}

/** New from this ▸ — Continue and Fork, on live user sessions. */
function sessionDeriveNode(session: SessionPayload): SessionMenuNode | null {
  if (!isUserControllableSession(session) || session.archived_at !== null) return null;
  return {
    kind: "sub",
    id: "derive",
    label: "New from this",
    icon: CopyPlus,
    children: [item("continue", session), item("fork", session)],
  };
}

/** One session's menu: the same tree on a list row and a board card, `…` or right-click. */
export function sessionMenuTree(
  session: SessionPayload,
  facts: SessionMenuFacts
): SessionMenuNode[] {
  const user = isUserControllableSession(session);
  const subagent = isSubagentSession(session);
  const archived = session.archived_at !== null;
  const live = session.state === "active" || session.state === "starting";
  const later = sessionIsInLater(session);
  const links = facts.linkedWork ?? sessionOwnLinks(session);
  return tidy([
    ...(user ? [item("rename", session)] : []),
    sessionCopyNode(session, links),
    ...[sessionDeriveNode(session), linkedWorkNode(session, links)].filter(
      (node): node is SessionMenuNode => node !== null
    ),
    separator("organize"),
    ...[sessionMoveNode(session, facts)].filter((node): node is SessionMenuNode => node !== null),
    ...(batchVerb("mark-seen").eligible(session) ? [item("mark-seen", session)] : []),
    separator("lifecycle"),
    ...(!archived && live ? [item("stop", session)] : []),
    ...(!subagent && !archived ? [item(later ? "back-from-later" : "later", session)] : []),
    ...(!subagent ? [item(archived ? "unarchive" : "archive", session)] : []),
    separator("danger"),
    ...(user && facts.clearable ? [item("clear", session)] : []),
    item("delete", session, { destructive: true }),
  ]);
}

function countOf(sessions: readonly SessionPayload[], id: SessionMenuVerb) {
  const eligible = batchVerb(id).eligible;
  return sessions.filter(session => eligible(session)).length;
}

function batchItem(
  id: SessionMenuVerb,
  sessions: readonly SessionPayload[],
  hint?: (count: number) => string
): SessionMenuNode | null {
  const count = countOf(sessions, id);
  // A verb no selected session can take is absent, not a disabled "0 sessions" row.
  if (count === 0) return null;
  const live = sessions.some(
    session => session.state !== "stopped" && batchVerb("archive").eligible(session)
  );
  const action = batchVerb(id, live);
  return {
    kind: "item",
    id,
    label: action.label,
    icon: action.icon,
    hint: hint ? hint(count) : sessionCountLabel(count),
  };
}

/** The selection's menu: the row tree minus the verbs that only mean something for one session. */
export function selectionMenuTree(
  sessions: readonly SessionPayload[],
  facts: Omit<SessionMenuFacts, "folderId" | "order" | "clearable">
): SessionMenuNode[] {
  const present = (node: SessionMenuNode | null): node is SessionMenuNode => node !== null;
  const nestable = sessions.filter(session =>
    facts.nestParents.some(parent => sessionAdoptRefusal(session, parent) === null)
  ).length;
  const profileMovable = sessions.filter(
    session =>
      isUserControllableSession(session) &&
      session.archived_at === null &&
      !isSubagentSession(session) &&
      sessionTreeParentId(session) === ""
  ).length;
  const inLater = countOf(sessions, "back-from-later");
  const archived = countOf(sessions, "unarchive");
  const branches = sessions.filter(session => session.branch).length;
  const move: SessionMenuNode = {
    kind: "sub",
    id: "move",
    label: "Move",
    icon: FolderInput,
    children: tidy([
      ...folderNodes({ ...facts, folderId: null }, true),
      separator("thread"),
      {
        kind: "sub",
        id: "nest",
        label: "Nest under…",
        icon: rowVerb("nest", sessions[0]!).icon,
        hint: sessionCountLabel(nestable),
        children: parentNodes(facts.nestParents),
      },
      ...[batchItem("make-top-level", sessions)].filter(present),
      separator("profile"),
      {
        kind: "sub",
        id: "move-to-profile",
        label: "Move to profile",
        icon: rowVerb("move-to-profile", sessions[0]!).icon,
        hint: sessionCountLabel(profileMovable),
        children: profileNodes(facts),
      },
    ]),
  };
  const children = tidy([
    {
      kind: "sub",
      id: "copy",
      label: "Copy",
      icon: Clipboard,
      children: tidy([
        batchItem("copy-id", sessions, () => "")!,
        batchItem("copy-title", sessions, () => "")!,
        ...(branches > 0
          ? [
              {
                kind: "item",
                id: "copy-branch",
                label: "Copy branch names",
                icon: GitBranch,
                hint: sessionCountLabel(branches),
              } as const,
            ]
          : []),
      ]),
    },
    separator("organize"),
    move,
    ...[batchItem("mark-seen", sessions)].filter(present),
    separator("lifecycle"),
    ...[batchItem("stop", sessions, count => `${count} active`)].filter(present),
    ...[batchItem("later", sessions)].filter(present),
    ...(inLater > 0
      ? [batchItem("back-from-later", sessions, count => `${count} in Later`)]
      : []
    ).filter(present),
    ...[batchItem("archive", sessions)].filter(present),
    ...(archived > 0
      ? [batchItem("unarchive", sessions, count => `${count} archived`)]
      : []
    ).filter(present),
    separator("danger"),
    {
      kind: "item",
      id: "delete",
      label: `Delete ${sessionCountLabel(sessions.length)}`,
      icon: batchVerb("delete").icon,
      destructive: true,
      hint: "⌘⌫",
    },
  ]);
  return [{ kind: "group", id: "count", label: sessionCountLabel(sessions.length), children }];
}
