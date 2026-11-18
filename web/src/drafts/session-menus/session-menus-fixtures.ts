/** Fixture facts for the grouped session menu draft: the board's sessions, folders and profiles. */
import {
  SESSIONS_BOARD_ARCHIVED,
  SESSIONS_BOARD_SESSIONS,
} from "@/systems/session/mocks/sessions-board-fixtures";
import {
  SESSION_LINKS_DOZEN,
  SESSION_LINK_SETS,
} from "@/systems/session/mocks/session-links-fixtures";
import { linkKey, type SessionLinkedWorkItem } from "@/systems/session/lib/session-linked-work";
import type { SessionPayload } from "@/systems/session/types";

import type { SessionMenuFacts, SessionMenuFolder } from "./session-menu-model";

/** The board fixtures inherit a parent from the primary fixture; here every session is a root. */
const ALL = [...Object.values(SESSIONS_BOARD_SESSIONS).flat(), ...SESSIONS_BOARD_ARCHIVED].map(
  (session): SessionPayload =>
    session.lineage
      ? {
          ...session,
          lineage: { ...session.lineage, parent_session_id: "", root_session_id: session.id },
        }
      : session
);

export function boardSession(id: string): SessionPayload {
  const found = ALL.find(session => session.id === id);
  if (!found) throw new Error(`Missing board fixture "${id}"`);
  return found;
}

export const FOLDERS: SessionMenuFolder[] = [
  { id: "fix-patches", name: "fix-patches" },
  { id: "specs", name: "specs" },
  { id: "issue-batch", name: "Issue batch #1176–#1182" },
];

export const PROFILES: SessionMenuFolder[] = [
  { id: "work", name: "work" },
  { id: "personal", name: "personal" },
];

/** Root rows of the list, top to bottom, and the rows inside the open folder. */
export const LIST_ROOT_IDS = ["s4", "s2", "s1", "s9", "s19"];
export const LIST_FOLDER_IDS = ["s7", "s12"];

/** Coordinators a session can nest under. */
export const NEST_PARENTS = [boardSession("s1"), boardSession("s19")];

/** The coordinator's linked work: its own dozen plus a pull request rolled up from a child. */
const CHILD_PR = SESSION_LINK_SETS.rolledUp[0]!;
const COORDINATOR_LINKS: SessionLinkedWorkItem[] = [
  {
    ...CHILD_PR,
    branch: "fix/login-redirect-loop",
    pr_number: 962,
    pull_request: {
      ...CHILD_PR.pull_request!,
      number: 962,
      url: "https://github.com/compozy/compozy/pull/962",
    },
  },
  ...SESSION_LINKS_DOZEN,
];

const LINKED_WORK: Record<string, SessionLinkedWorkItem[]> = { s1: COORDINATOR_LINKS };

/** The submenu id of the coordinator's first item that needs you. */
export const COORDINATOR_FIRST_LINK = `link-${linkKey(COORDINATOR_LINKS[0]!)}`;

/** The facts a list row has: folders, manual order, and Clear on the open session. */
export function listFacts(id: string, folderId: string | null = null): SessionMenuFacts {
  const order = folderId === null ? LIST_ROOT_IDS : LIST_FOLDER_IDS;
  const index = order.indexOf(id);
  return {
    folders: FOLDERS,
    folderId,
    profiles: PROFILES,
    nestParents: NEST_PARENTS.filter(parent => parent.id !== id),
    order: { canMoveUp: index > 0, canMoveDown: index < order.length - 1 },
    clearable: id === "s4",
    linkedWork: LINKED_WORK[id],
  };
}

/** A board card has every list fact except manual order: columns order the board. */
export function boardFacts(id: string): SessionMenuFacts {
  return {
    folders: FOLDERS,
    folderId: id === "s7" || id === "s12" ? "fix-patches" : null,
    profiles: PROFILES,
    nestParents: NEST_PARENTS.filter(parent => parent.id !== id),
    clearable: id === "s4",
    linkedWork: LINKED_WORK[id],
  };
}

export const SELECTION_FACTS = { folders: FOLDERS, profiles: PROFILES, nestParents: NEST_PARENTS };
