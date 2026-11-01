/** The main session window with each report, relayed approval and rotation state. */
import {
  CircleAlert,
  Clock,
  History,
  PlugZap,
  RefreshCw,
  ShieldAlert,
  ShieldQuestion,
  SquareTerminal,
} from "lucide-react";
import type { ReactNode } from "react";

import { Button, Dock, KindIcon, Marker, Pill, type TopbarSlotValue } from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import { TRIAGE } from "../persistent-agents-shared/fixtures";
import { SessionsWindow } from "../persistent-agents-shared/session-frame";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../persistent-agents-shared/transcript";
import { ReportEntry, SessionLink } from "./report-entry";
import { RAIL } from "./report-rail";

const TABS = [
  { id: "main", title: "Triage Lead" },
  { id: "pr", title: "Topbar PR chip — web slice" },
];

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
  above,
  composer,
  readOnly,
}: {
  children?: ReactNode;
  state?: AgentState;
  above?: ReactNode;
  composer?: ReactNode;
  readOnly?: ReactNode;
}) {
  return (
    <SessionsWindow
      activeTabId="main"
      rail={RAIL}
      tabs={TABS}
      title="Triage Lead"
      topbar={mainHead(state)}
    >
      <TranscriptColumn>
        <Earlier />
        {children}
      </TranscriptColumn>
      {readOnly ?? composer ?? <DraftComposer above={above} placeholder="Message Triage Lead" />}
    </SessionsWindow>
  );
}

const PROPOSALS = [
  { title: "Unblock the token migration", body: "Run 12 waits on a review." },
  { title: "Close the stale design issue", body: "#918 was fixed in #931 and nobody closed it." },
];

export function Proposal({ footnote }: { footnote?: ReactNode }) {
  return (
    <ReportEntry
      agent={TRIAGE}
      footnote={footnote}
      kind="Proposal"
      proposals={PROPOSALS}
      when="23:30"
    />
  );
}

export function FailureReport() {
  return (
    <ReportEntry
      actions={<SessionLink to="failure">Open heartbeat</SessionLink>}
      agent={TRIAGE}
      body="The check took longer than 10 minutes."
      icon={CircleAlert}
      kind="Check failed"
      title="Couldn't finish this check"
      tone="danger"
      when="23:30"
    />
  );
}

export function ExpiredApproval() {
  return (
    <ReportEntry
      agent={TRIAGE}
      body="Nobody answered in 12 hours, so the request was denied."
      icon={ShieldAlert}
      kind="Approval expired"
      title="Run make migrate in Code Engineer's session"
      tone="warning"
      when="11:30"
    />
  );
}

export function ProviderEnded() {
  return (
    <ReportEntry
      agent={TRIAGE}
      body="Claude stopped waiting before you answered."
      icon={Clock}
      kind="Approval expired by the provider"
      title="Edit internal/daemon/boot.go in Code Engineer's session"
      tone="warning"
      when="02:14"
    />
  );
}

export function InterruptedReport() {
  return (
    <ReportEntry
      agent={TRIAGE}
      body="CompozyOS restarted while Code Engineer's run was waiting for an approval."
      icon={PlugZap}
      kind="Interrupted"
      title="This run stopped"
      tone="warning"
      when="08:02"
    />
  );
}

export function RotatedMarker() {
  return (
    <Marker icon={<History aria-hidden="true" className="size-3" />}>
      This conversation continues from an earlier session.{" "}
      <SessionLink to="superseded">Read previous session</SessionLink>
    </Marker>
  );
}

export function RotationReport() {
  return (
    <ReportEntry
      agent={TRIAGE}
      body="Moved to a new session because this provider can't compact."
      icon={RefreshCw}
      kind="New session"
      title="Started a fresh conversation"
      when="06:40"
    />
  );
}

export function SupersededNotice() {
  return (
    <div className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5">
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg bg-surface-2 px-3 py-2.5 shadow-hairline-inset">
        <History aria-hidden="true" className="size-4 shrink-0 text-muted" />
        <span className="min-w-0 flex-1 text-small-body text-muted">
          This conversation moved to a newer session.
        </span>
        <SessionLink to="rotation">Open current session</SessionLink>
      </div>
    </div>
  );
}

export function ApprovalDock() {
  return (
    <Dock aria-label="Permission required" role="region">
      <Dock.Head>
        <KindIcon icon={SquareTerminal} tone="well" />
        <Dock.Title>Allow Code Engineer to run make migrate?</Dock.Title>
        <Dock.Count>1/2</Dock.Count>
      </Dock.Head>
      <Dock.Body>
        <Dock.Pre>make migrate</Dock.Pre>
        <span className="flex items-center gap-2 text-eyebrow text-subtle">
          <ShieldQuestion aria-hidden="true" className="size-3" />
          From Migrate tokens · waits 11h 42m more
        </span>
      </Dock.Body>
      <Dock.Actions>
        <Button size="sm" type="button" variant="ghost">
          Reject
        </Button>
        <Button size="sm" type="button">
          Allow once
        </Button>
      </Dock.Actions>
    </Dock>
  );
}
