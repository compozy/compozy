/** The Sessions window in List: the rail beside the open session, the scope beside the view toggle. */
import { protoHref } from "@protohq/cli";
import { Info, MessageSquare, RefreshCw, WifiOff } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, type TopbarSlotValue } from "@compozy/ui";

import type { ComputerFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import { SessionViewToggle } from "@/systems/session/components/session-board/session-view-toggle";
import { SessionPanelToggle } from "@/systems/session/components/session-panel-toggle";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../always-on-agents-shared/transcript";
import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { projectName } from "./fixtures";

const LIST_SLUG = "sessions-by-computer";
const KANBAN_SLUG = "sessions-by-computer-kanban";

export function ViewToggle({ value }: { value: "list" | "kanban" }) {
  return (
    <SessionViewToggle
      onValueChange={view =>
        window.location.assign(
          view === "list"
            ? protoHref(LIST_SLUG, "list-separate-project")
            : protoHref(KANBAN_SLUG, "kanban-bands-projects")
        )
      }
      value={value}
    />
  );
}

export interface ListWindowProps {
  rail: ReactNode;
  /** The computer scope; absent when this computer is the only one. */
  scope?: ReactNode;
  session: SessionPayload;
  children: ReactNode;
}

/** List: the rail beside the open session, its project before the title. */
export function ListWindow({ rail, scope, session, children }: ListWindowProps) {
  const topbar: TopbarSlotValue = {
    overflow: (
      <span className="flex items-center gap-2">
        <ViewToggle value="list" />
        {scope}
      </span>
    ),
    actions: (
      <>
        <SessionPanelToggle onToggle={() => undefined} open panel="sidebar" />
        <SessionPanelToggle onToggle={() => undefined} open={false} panel="inspector" />
      </>
    ),
  };
  return (
    <DraftShell
      app="sessions"
      glyph={<MessageSquare />}
      leading={
        <WorkspaceBadge
          identity={{ kind: "project", name: projectName(session.workspace_id) }}
          variant="chrome"
        />
      }
      sessionBadge={4}
      title={session.name}
      topbar={topbar}
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        {rail}
        <div className="flex min-w-0 flex-1 flex-col border-l border-line">{children}</div>
      </div>
    </DraftShell>
  );
}

const TRANSCRIPTS: Record<string, { ask: string; answer: string; placeholder: string }> = {
  d1: {
    ask: "SPEI webhooks time out during the partner’s nightly batch. Fix the retries.",
    answer:
      "Two retry policies fit the partner’s limits, and they settle refunds at different times. Which one should I ship?",
    placeholder: "Type here to answer the agent's question",
  },
  l1: {
    ask: "Raise the Brazil canary to 50% once the error budget holds for an hour.",
    answer:
      "The budget held for the full hour. Raising the canary changes production traffic, so it needs your approval first.",
    placeholder: "Type here to answer the agent's question",
  },
  w1: {
    ask: "Rebuild the card velocity feature store from October’s events.",
    answer:
      "Three of seven partitions are rebuilt. The other four continue when workstation is back online.",
    placeholder: "workstation is offline. Messages send when it reconnects.",
  },
};

export function OpenSession({ session, offline }: { session: SessionPayload; offline: boolean }) {
  const turns = TRANSCRIPTS[session.id] ?? TRANSCRIPTS.d1!;
  return (
    <>
      <TranscriptColumn>
        <UserTurn>{turns.ask}</UserTurn>
        <AgentTurn>{turns.answer}</AgentTurn>
      </TranscriptColumn>
      <div className="mx-auto w-full max-w-3xl px-6 pb-5">
        <DraftComposer disabled={offline} placeholder={turns.placeholder} />
      </div>
    </>
  );
}

function RailNotice({
  icon,
  title,
  body,
  action,
}: {
  icon: ReactNode;
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <div className="mx-2.5 mb-2 flex items-start gap-2.5 rounded-md border border-line bg-surface-2/60 px-3 py-2.5">
      {icon}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="m-0 text-small-body text-fg">{title}</p>
        <p className="m-0 text-eyebrow text-muted-foreground">{body}</p>
      </div>
      {action}
    </div>
  );
}

/** A scope on an offline computer: its last known rows stay, dimmed, under one way back. */
export function OfflineNotice({ computer }: { computer: ComputerFixture }) {
  return (
    <RailNotice
      action={
        <Button
          className="h-6 shrink-0 px-2 text-badge"
          size="xs"
          type="button"
          variant="secondary"
        >
          <RefreshCw aria-hidden="true" className="size-3" />
          Reconnect
        </Button>
      }
      body={`Last seen ${computer.lastConnectedLabel}. These sessions continue when it reconnects.`}
      icon={<WifiOff aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
      title={`${computer.name} is offline`}
    />
  );
}

/** The scoped computer left the account: the window falls back to every computer and says so. */
export function ScopeRemovedNotice({ name }: { name: string }) {
  return (
    <RailNotice
      body="This window shows every computer now."
      icon={<Info aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
      title={`${name} was removed`}
    />
  );
}
