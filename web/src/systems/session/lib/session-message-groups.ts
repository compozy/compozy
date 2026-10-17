// Inbound session-message grouping (#760): a run of "From" cards one sender
// delivered back to back reads as one collapsible group, its latest card
// visible. Pure over the thread's message list; the transcript decides which
// messages render nothing (`isSilent`) so a silent status message between two
// cards never splits the run, while any visible row does.

import { sessionMessageOrigin, type SessionMessageOrigin } from "./session-message-payload";

export interface GroupableMessage {
  id: string;
  role?: string;
  metadata?: unknown;
}

export interface SessionInboundGroup {
  /** The first member's message id: stable while the run grows at its end. */
  id: string;
  /** Member message ids, oldest first; always two or more. */
  memberIds: readonly string[];
  /** The sender every member names. */
  sender: SessionMessageOrigin;
}

export interface SessionInboundGroupMember {
  group: SessionInboundGroup;
  /** The newest member: always visible, carries the group's toggle. */
  latest: boolean;
}

function senderKey(origin: SessionMessageOrigin): string {
  return `${origin.workspaceId}\u0000${origin.sessionId}`;
}

/**
 * Member lookup by message id for every run of two or more consecutive inbound
 * session messages from the same sender. Messages for which `isSilent` holds
 * (rendering no row) are skipped between members; any other message, the
 * operator's own prompt included, ends the run.
 */
export function sessionInboundGroups(
  messages: readonly GroupableMessage[],
  isSilent: (message: GroupableMessage) => boolean
): ReadonlyMap<string, SessionInboundGroupMember> {
  const runs: { key: string; sender: SessionMessageOrigin; ids: string[] }[] = [];
  let open = false;
  for (const message of messages) {
    const origin = message.role === "user" ? sessionMessageOrigin(message.metadata) : null;
    if (origin) {
      const key = senderKey(origin);
      const current = open ? runs.at(-1) : undefined;
      if (current?.key === key) current.ids.push(message.id);
      else runs.push({ key, sender: origin, ids: [message.id] });
      open = true;
    } else if (!isSilent(message)) {
      open = false;
    }
  }
  const members = new Map<string, SessionInboundGroupMember>();
  for (const run of runs) {
    if (run.ids.length < 2) continue;
    const group: SessionInboundGroup = { id: run.ids[0]!, memberIds: run.ids, sender: run.sender };
    run.ids.forEach((id, index) =>
      members.set(id, { group, latest: index === run.ids.length - 1 })
    );
  }
  return members;
}
