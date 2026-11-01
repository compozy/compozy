/** The main session window, with every report, relayed approval and rotation state. */
import { protoHref } from "@compozy/proto";
import { Check, History, ShieldQuestion, X } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Button, Dock, Marker, Pill, Receipt, type TopbarSlotValue } from "@compozy/ui";

import { PermissionDockView } from "@/systems/session/components/permission-dock-view";
import type { PermissionRequest } from "@/systems/session/types";
import { terminalPermissionDetail } from "@/systems/terminal/lib/terminal-permission";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import { CODE_ENGINEER, REVIEWER, TRIAGE } from "../persistent-agents-shared/fixtures";
import { SessionsWindow } from "../persistent-agents-shared/session-frame";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
  WorkingLine,
} from "../persistent-agents-shared/transcript";
import { CheckTicks } from "../persistent-sessions/agent-hover-card";
import { Notice, SessionLink, ShoulderTap, type ProposalChoice } from "./report-entry";
import { RAIL } from "./report-rail";
import {
  SessionConversationProvider,
  useSessionConversation,
} from "./hooks/use-session-conversation";

function tabs(state: AgentState, pending: number) {
  return [
    {
      id: "main",
      title: "Triage Lead",
      glyph: (
        <AgentAvatar
          appearance={TRIAGE.appearance}
          label="Triage Lead"
          quietIdle
          size={16}
          state={state}
        />
      ),
      pending,
    },
    { id: "pr", title: "Topbar PR chip — web slice" },
  ];
}

function mainHead(state: AgentState): TopbarSlotValue {
  return {
    glyph: (
      <AgentAvatar
        appearance={TRIAGE.appearance}
        label="Triage Lead"
        quietIdle
        size={20}
        state={state}
      />
    ),
    glyphPresentation: "state",
    crumb: "Triage Lead",
    status: <Pill size="sm">compozy</Pill>,
  };
}

function Earlier() {
  return (
    <>
      <UserTurn>Keep an eye on the token migration while I'm out.</UserTurn>
      <AgentTurn>I'll watch run 12 and report here only if something needs you.</AgentTurn>
    </>
  );
}

export function MainSession({
  children,
  state = "idle",
  pending = 0,
  above,
  composer,
  readOnly,
  history = <Earlier />,
  sent,
}: {
  children?: ReactNode;
  state?: AgentState;
  /** Decisions waiting here, for the tab's count. */
  pending?: number;
  above?: ReactNode;
  composer?: ReactNode;
  readOnly?: ReactNode;
  history?: ReactNode;
  /** A prompt already on its way, for the states that open right after sending one. */
  sent?: string;
}) {
  const [after, setAfter] = useState<readonly ReactNode[]>(
    sent ? [<UserTurn key="sent">{sent}</UserTurn>] : []
  );
  const [replied, setReplied] = useState(Boolean(sent));
  const conversation = {
    send(text: string) {
      if (!text.trim()) return;
      setReplied(true);
      setAfter(lines => [...lines, <UserTurn key={lines.length}>{text}</UserTurn>]);
    },
    record(line: ReactNode) {
      setAfter(lines => [...lines, <div key={lines.length}>{line}</div>]);
    },
  };
  const live: AgentState = replied ? "working" : state;
  return (
    <SessionConversationProvider value={conversation}>
      <SessionsWindow
        activeTabId="main"
        rail={RAIL}
        tabs={tabs(live, pending)}
        title="Triage Lead"
        topbar={mainHead(live)}
      >
        <TranscriptColumn>
          {history}
          {children}
          {after}
          {replied ? <WorkingLine who="Triage Lead" /> : null}
        </TranscriptColumn>
        {readOnly ?? composer ?? (
          <DraftComposer
            above={above}
            onSend={conversation.send}
            placeholder="Message Triage Lead"
          />
        )}
      </SessionsWindow>
    </SessionConversationProvider>
  );
}

const CHOICES: readonly ProposalChoice[] = [
  {
    title: "Unblock the token migration",
    body: "Run 12 has been waiting on a review since yesterday.",
  },
  {
    title: "Close the stale design issue",
    body: "#918 was fixed in #931 and nobody closed it.",
  },
];

export function Proposal({ footnote, when = "23:30" }: { footnote?: ReactNode; when?: string }) {
  const { send } = useSessionConversation();
  const [chosen, setChosen] = useState<string>();
  return (
    <ShoulderTap
      agent={TRIAGE}
      ask="I found two things worth doing. Which one first?"
      choices={CHOICES}
      chosen={chosen}
      footnote={footnote}
      onChoose={choice => {
        setChosen(choice.title);
        send(choice.title);
      }}
      when={when}
    />
  );
}

/** Sending this is an ordinary prompt, so the notice can only offer what a message can do. */
function AskButton({ children, prompt }: { children: ReactNode; prompt: string }) {
  const { send } = useSessionConversation();
  return (
    <Button onClick={() => send(prompt)} size="sm" type="button" variant="secondary">
      {children}
    </Button>
  );
}

function CommandLine({ children }: { children: string }) {
  return <Dock.Pre className="mt-0">{children}</Dock.Pre>;
}

export function FailureReport() {
  return (
    <Notice
      actions={
        <>
          <AskButton prompt="Try that check again and tell me what stopped it.">
            Try again
          </AskButton>
          <Button
            onClick={() => window.location.assign(protoHref("agent-editors", "checkins"))}
            size="sm"
            type="button"
            variant="ghost"
          >
            See check-ins
          </Button>
        </>
      }
      agent={TRIAGE}
      body="The check took longer than 10 minutes, so I stopped it. This is the third time in a row."
      detail={
        <div className="flex max-w-xs flex-col gap-1.5">
          <CheckTicks
            marks={[
              "quiet",
              "quiet",
              "reported",
              "quiet",
              "quiet",
              "quiet",
              "quiet",
              "quiet",
              "quiet",
              "failed",
              "failed",
              "failed",
            ]}
          />
          <span className="text-eyebrow text-subtle">
            After 10 failures in a row, check-ins pause on their own.
          </span>
        </div>
      }
      state="failed"
      title="I couldn't finish this check"
      when="23:30"
    />
  );
}

export function ExpiredApproval() {
  return (
    <Notice
      actions={
        <AskButton prompt="Try make migrate again, I'm here now.">Ask it to try again</AskButton>
      }
      agent={TRIAGE}
      body="Code Engineer asked to run this and nobody answered in 12 hours, so it wasn't allowed."
      detail={<CommandLine>make migrate</CommandLine>}
      title="A request went unanswered"
      when="11:30"
    />
  );
}

export function ProviderEnded() {
  return (
    <Notice
      actions={
        <AskButton prompt="Make that edit again, I'm here now.">Ask it to try again</AskButton>
      }
      agent={TRIAGE}
      body="Claude stopped waiting for an answer before you got to it, so the edit didn't happen."
      detail={<CommandLine>internal/daemon/boot.go</CommandLine>}
      title="A request ended early"
      when="02:14"
    />
  );
}

export function InterruptedReport() {
  return (
    <Notice
      actions={
        <AskButton prompt="Pick the migration back up where it stopped.">Pick it back up</AskButton>
      }
      agent={TRIAGE}
      body="CompozyOS restarted while Code Engineer was waiting for your answer, so its work stopped there."
      title="Code Engineer's work stopped"
      when="08:02"
    />
  );
}

export function ApprovalsWaiting() {
  return (
    <Marker icon={<ShieldQuestion aria-hidden="true" className="size-3" />}>
      <b>Two requests</b> are waiting for you, from sessions Triage Lead started. Nothing moves
      until you answer.
    </Marker>
  );
}

export function RotationFailedReport() {
  return (
    <Notice
      agent={TRIAGE}
      body="I tried to start a fresh conversation and couldn't, so we stay here. You don't need to do anything."
      title="Still in this conversation"
      when="06:40"
    />
  );
}

export function RotationDeferredMarker() {
  return (
    <Marker icon={<History aria-hidden="true" className="size-3" />}>
      A fresh conversation is waiting for Code Engineer to finish. That's taken over an hour.
    </Marker>
  );
}

export function RotatedMarker() {
  return (
    <Marker icon={<History aria-hidden="true" className="size-3" />}>
      This conversation continues from an earlier one.{" "}
      <SessionLink to="superseded">Read it</SessionLink>
    </Marker>
  );
}

export function SupersededNotice() {
  return (
    <div className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5">
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg bg-surface-2 px-3 py-2.5 shadow-hairline-inset">
        <History aria-hidden="true" className="size-4 shrink-0 text-muted" />
        <span className="min-w-0 flex-1 text-small-body text-muted">
          This conversation moved on. You can read it here.
        </span>
        <SessionLink to="rotation">Open the current one</SessionLink>
      </div>
    </div>
  );
}

const TERMINAL_TOOL = "compozy__terminal_exec";

const ASKS = [
  {
    agent: CODE_ENGINEER,
    session: "Migrate tokens",
    waits: "Waits 11h 42m more",
    subject: null,
    permission: {
      requestId: "perm-1",
      toolId: TERMINAL_TOOL,
      toolName: TERMINAL_TOOL,
      toolInput: { command: "make migrate", cwd: "~/Dev/compozy", risk: "ordinary" },
      action: "execute",
      resource: "",
    } satisfies PermissionRequest,
    verb: "make migrate",
  },
  {
    agent: REVIEWER,
    session: "Read the diff for #931",
    waits: "Waits 9h 10m more",
    subject: "internal/daemon/boot.go",
    permission: {
      requestId: "perm-2",
      toolName: "edit",
      toolInput: { path: "internal/daemon/boot.go" },
      action: "edit",
      resource: "",
    } satisfies PermissionRequest,
    verb: "internal/daemon/boot.go",
  },
];

/** Asks from sessions this agent started. Answering here is the same as answering there. */
export function ApprovalDock() {
  const { record } = useSessionConversation();
  const [index, setIndex] = useState(0);
  const ask = ASKS[index];
  if (!ask) return null;
  return (
    <PermissionDockView
      agentName={ask.agent.label}
      countLabel={`${index + 1}/${ASKS.length}`}
      decide={decision => {
        const allowed = decision.startsWith("allow");
        record(
          <Receipt
            icon={allowed ? <Check aria-hidden="true" /> : <X aria-hidden="true" />}
            tone={allowed ? "allowed" : "rejected"}
          >
            {allowed ? "You allowed" : "You declined"} <code>{ask.verb}</code> for{" "}
            <b>{ask.agent.label}</b>
          </Receipt>
        );
        setIndex(index + 1);
      }}
      decisionOptions={["allow-once", "allow-always", "reject-once"]}
      isSubmitting={false}
      origin={
        <div className="flex items-center gap-2 text-small-body text-muted">
          <AgentAvatar
            appearance={ask.agent.appearance}
            label={ask.agent.label}
            quietIdle
            size={18}
            surfaceClassName="ring-card"
          />
          <span className="min-w-0 truncate">In {ask.session}</span>
          <Dock.Deadline>{ask.waits}</Dock.Deadline>
        </div>
      }
      permission={ask.permission}
      subject={ask.subject}
      terminalDetail={terminalPermissionDetail(ask.permission.toolName, ask.permission.toolInput)}
    />
  );
}
