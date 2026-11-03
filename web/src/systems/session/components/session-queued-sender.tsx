import { Folder } from "@compozy/ui/icons";

import { cn } from "@/lib/utils";
import { Skeleton } from "@compozy/ui";
import { AgentIdentityMark } from "@/systems/agent/components/agent-identity-mark";

import {
  sessionPartyMark,
  type SessionMessageParty,
} from "./session-messages/session-message-party";
import { SubagentAvatar } from "./subagents/subagent-avatar";

/** "From {title}" with the sender's provider mark and, across workspaces, its workspace (Gap 5). */
export function SessionQueuedSender({
  party,
  title,
}: {
  party: SessionMessageParty;
  title: string;
}) {
  return (
    <span
      data-testid="composer-queued-sender"
      aria-busy={party.pending || undefined}
      title={party.pending ? undefined : `From ${title}`}
      className="inline-flex max-w-[70%] min-w-0 items-center gap-1.5 text-micro text-muted-foreground"
    >
      {party.staysOnAgent ? (
        <AgentIdentityMark aria-hidden="true" name={party.staysOnAgent} quietIdle size={18} />
      ) : (
        <SubagentAvatar provider={sessionPartyMark(party)} size="sm" className="size-4.5" />
      )}
      <span className="shrink-0">From</span>
      {party.pending ? (
        <Skeleton aria-hidden="true" className="inline-block h-3 w-16 rounded-xs" />
      ) : (
        <span
          className={cn(
            "min-w-0 truncate",
            party.title === null ? "font-medium text-muted-foreground" : "font-medium text-fg-2"
          )}
        >
          {title}
        </span>
      )}
      {party.workspaceName ? (
        <span
          className="inline-flex min-w-0 items-center gap-0.5 text-subtle"
          data-testid="composer-queued-sender-workspace"
        >
          <Folder aria-hidden="true" className="size-2.75 shrink-0 text-faint" />
          <span className="truncate">{party.workspaceName}</span>
        </span>
      ) : null}
    </span>
  );
}
