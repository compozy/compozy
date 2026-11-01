import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { GENERAL } from "../persistent-agents-shared/fixtures";
import { SessionsWindow } from "../persistent-agents-shared/session-frame";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../persistent-agents-shared/transcript";
import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import {
  AuditScreen,
  DeletedAgentMarker,
  MentionComposer,
  OldNotification,
  UnresolvedComposer,
} from "./identity-views";
import {
  ApprovalDock,
  ExpiredApproval,
  FailureReport,
  InterruptedReport,
  MainSession,
  Proposal,
  ProviderEnded,
  RotatedMarker,
  RotationReport,
  SupersededNotice,
} from "./session-views";
import { RAIL } from "./report-rail";

export const meta = definePrototype({
  title: "Persistent reports",
  spec: "always-on-agent",
  surface: "S11 S12 S13 S14",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "proposal", label: "S11 · Proposal" },
    { id: "failure", label: "S11 · Failure report" },
    { id: "approval", label: "S11 · Relayed child approval" },
    { id: "expired-approval", label: "S11 · Expired approval" },
    { id: "provider-ended", label: "S11 · Provider-ended approval" },
    { id: "interrupted", label: "S11 · Interrupted run" },
    { id: "rotation", label: "S11 · After rotation" },
    { id: "superseded", label: "S11 · Superseded, read-only" },
    { id: "maybe-not-received", label: "S11 · May not have been received" },
    { id: "deleted-agent", label: "S12 · Deleted agent" },
    { id: "old-notification", label: "S12 · Old notification payload" },
    { id: "mention-menu", label: "S13 · Agent mention" },
    { id: "mention-unresolved", label: "S13 · Unresolved chip" },
    { id: "audit", label: "S14 · Nobody watching" },
    { id: "audit-unknown", label: "S14 · Unknown initiator" },
  ],
});

function DeletedAgentSession({ notify = false }: { notify?: boolean }) {
  const label = notify ? "Code Implementer" : "Release Captain";
  const seed = notify ? "code_implementer" : "release-captain";
  return (
    <SessionsWindow
      overlay={notify ? <OldNotification /> : undefined}
      rail={RAIL}
      title={label}
      topbar={{
        crumb: label,
        glyph: (
          <AgentAvatar
            appearance={{ avatar: "generated", seed }}
            label={label}
            quietIdle
            size={20}
            state="unknown"
          />
        ),
        glyphPresentation: "state",
      }}
    >
      <TranscriptColumn>
        <UserTurn>Cut the release branch once CI is green.</UserTurn>
        <AgentTurn>The branch is ready and the notes are drafted.</AgentTurn>
        {notify ? null : <DeletedAgentMarker />}
      </TranscriptColumn>
      <DraftComposer disabled placeholder={`Message ${GENERAL.label}`} />
    </SessionsWindow>
  );
}

export default function PersistentReportsDraft({ state }: PrototypeProps) {
  switch (state) {
    case "failure":
      return (
        <MainSession state="failed">
          <FailureReport />
        </MainSession>
      );
    case "approval":
      return <MainSession above={<ApprovalDock />} state="needs-you" />;
    case "expired-approval":
      return (
        <MainSession>
          <ExpiredApproval />
        </MainSession>
      );
    case "provider-ended":
      return (
        <MainSession>
          <ProviderEnded />
        </MainSession>
      );
    case "interrupted":
      return (
        <MainSession>
          <InterruptedReport />
        </MainSession>
      );
    case "rotation":
      return (
        <MainSession>
          <RotationReport />
          <RotatedMarker />
        </MainSession>
      );
    case "superseded":
      return <MainSession readOnly={<SupersededNotice />} />;
    case "maybe-not-received":
      return (
        <MainSession>
          <Proposal footnote="This report may not have been received." />
        </MainSession>
      );
    case "deleted-agent":
      return <DeletedAgentSession />;
    case "old-notification":
      return <DeletedAgentSession notify />;
    case "mention-menu":
      return <MainSession composer={<MentionComposer />} />;
    case "mention-unresolved":
      return <MainSession composer={<UnresolvedComposer />} />;
    case "audit":
      return <AuditScreen initial="unattended" />;
    case "audit-unknown":
      return <AuditScreen initial="all" legacy />;
    default:
      return (
        <MainSession state="needs-you">
          <Proposal />
        </MainSession>
      );
  }
}
