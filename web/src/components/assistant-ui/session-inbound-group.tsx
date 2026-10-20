import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { createContext, use, useState, type ReactNode } from "react";

import { TranscriptDisclosure } from "@compozy/ui";
import { useSessionLabel } from "@/systems/session/components/session-messages/use-session-label";
import {
  SessionMessageJumpContext,
  type SessionMessageJump,
} from "@/systems/session/contexts/session-message-jump-context-value";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import {
  sessionInboundGroups,
  type GroupableMessage,
  type SessionInboundGroup,
  type SessionInboundGroupMember,
} from "@/systems/session/lib/session-message-groups";
import {
  sessionMonitorWakeMeta,
  sessionReplyMeta,
} from "@/systems/session/lib/session-message-payload";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";

import {
  notifyDisclosureToggled,
  useOptionalThreadScrollStore,
} from "./hooks/thread-scroll-context";
import { useRevealHold } from "./hooks/session-navigation-target-context";
import { deriveSessionRows } from "./session-timeline.logic";

/** Which inbound groups the reader opened; every group starts collapsed. */
const inboundGroupLogic = createStoreLogic({
  context: { expanded: new Set<string>() },
  on: {
    toggled: (context, event: { id: string }) => {
      const expanded = new Set(context.expanded);
      if (expanded.has(event.id)) expanded.delete(event.id);
      else expanded.add(event.id);
      return { expanded };
    },
    opened: (context, event: { id: string }) =>
      context.expanded.has(event.id)
        ? context
        : { expanded: new Set(context.expanded).add(event.id) },
  },
});

type InboundGroupStore = ReturnType<typeof inboundGroupLogic.createStore>;

const InboundGroupMembersContext = createContext<ReadonlyMap<
  string,
  SessionInboundGroupMember
> | null>(null);
const InboundGroupStoreContext = createContext<InboundGroupStore | null>(null);

type ThreadMessageLike = GroupableMessage & { content?: unknown };

const silentByMessage = new WeakMap<object, boolean>();
const groupsByMessages = new WeakMap<object, ReadonlyMap<string, SessionInboundGroupMember>>();

// A message the transcript renders no row for: status-only assistant shells and
// synthetic wakes other than a reply or a monitor wake. Cached per message object.
function isSilentMessage(message: ThreadMessageLike): boolean {
  const cached = silentByMessage.get(message);
  if (cached !== undefined) return cached;
  const silent =
    message.role !== "user" &&
    sessionReplyMeta(message.metadata) === null &&
    sessionMonitorWakeMeta(message.metadata) === null &&
    (message.role !== "assistant" || deriveSessionRows(toTimelineParts(message)).length === 0);
  silentByMessage.set(message, silent);
  return silent;
}

function inboundGroupMembers(messages: readonly ThreadMessageLike[]) {
  const cached = groupsByMessages.get(messages);
  if (cached) return cached;
  const members = sessionInboundGroups(messages, isSilentMessage);
  groupsByMessages.set(messages, members);
  return members;
}

/**
 * Groups consecutive inbound session messages per sender (#760) and owns the
 * in-transcript jump: a jump to a member hidden in a collapsed group opens the
 * group first, then scrolls.
 */
export function SessionInboundGroupsProvider({
  messages,
  onJump,
  children,
}: {
  messages: readonly ThreadMessageLike[];
  onJump?: (threadMessageId: string) => void;
  children: ReactNode;
}) {
  const [store] = useState(() => inboundGroupLogic.createStore(undefined));
  const members = inboundGroupMembers(messages);
  const jump: SessionMessageJump | null = onJump
    ? threadMessageId => {
        const member = members.get(threadMessageId);
        if (member && !member.latest) store.trigger.opened({ id: member.group.id });
        onJump(threadMessageId);
      }
    : null;
  return (
    <InboundGroupStoreContext value={store}>
      <InboundGroupMembersContext value={members}>
        <SessionMessageJumpContext value={jump}>{children}</SessionMessageJumpContext>
      </InboundGroupMembersContext>
    </InboundGroupStoreContext>
  );
}

function InboundGroupToggle({
  group,
  expanded,
  onToggle,
}: {
  group: SessionInboundGroup;
  expanded: boolean;
  onToggle: () => void;
}) {
  const context = useSessionRuntimeRenderContext();
  const sender = useSessionLabel({
    sessionId: group.sender.sessionId,
    workspaceId: group.sender.workspaceId,
    currentWorkspaceId: context?.workspaceId ?? "",
    agentName: group.sender.agentName,
    titleAtSend: group.sender.titleAtSend,
  });
  const count = `${group.memberIds.length} messages`;
  const label = sender.pending ? count : `${sender.title ?? "A deleted session"} · ${count}`;
  return (
    <div className="flex min-w-0 pt-1">
      <TranscriptDisclosure
        variant="turn"
        icon={null}
        expanded={expanded}
        onToggle={onToggle}
        label={label}
        aria-label={expanded ? `${label}, showing all` : `${label}, showing the latest`}
        data-testid="session-inbound-group-toggle"
        data-count={group.memberIds.length}
        className="max-w-full"
      />
    </div>
  );
}

function InboundGroupMemberSlot({
  member,
  store,
  messageId,
  children,
}: {
  member: SessionInboundGroupMember;
  store: InboundGroupStore;
  messageId: string;
  children: ReactNode;
}) {
  const { group, latest } = member;
  const scrollStore = useOptionalThreadScrollStore();
  const byReader = useSelector(store, state => state.context.expanded.has(group.id));
  const hold = useRevealHold(`inbound-group:${group.id}`, reveal =>
    group.memberIds.includes(reveal.messageId)
  );
  const expanded = byReader || hold.held;
  if (!expanded && !latest) return null;
  // The toggle heads what is visible: the latest card while collapsed, the first one when open.
  const heads = expanded ? messageId === group.id : latest;
  const toggle = () => {
    notifyDisclosureToggled(scrollStore);
    if (hold.held) hold.release();
    if (byReader || !hold.held) store.trigger.toggled({ id: group.id });
  };
  return (
    <div data-inbound-group={group.id} data-group-latest={latest || undefined}>
      {heads ? <InboundGroupToggle group={group} expanded={expanded} onToggle={toggle} /> : null}
      {children}
    </div>
  );
}

/** One thread row: a grouped inbound message renders through its group, every other row as is. */
export function SessionInboundGroupSlot({
  messageId,
  children,
}: {
  messageId: string;
  children: ReactNode;
}) {
  const store = use(InboundGroupStoreContext);
  const member = use(InboundGroupMembersContext)?.get(messageId);
  if (!store || !member) return children;
  return (
    <InboundGroupMemberSlot member={member} store={store} messageId={messageId}>
      {children}
    </InboundGroupMemberSlot>
  );
}
