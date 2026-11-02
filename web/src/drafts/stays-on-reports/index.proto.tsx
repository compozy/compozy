import { definePrototype, type PrototypeProps } from "@compozy/proto";
import type { ReactNode } from "react";

import { GENERAL, TRIAGE } from "../stays-on-agents-shared/fixtures";
import { SessionsWindow } from "../stays-on-agents-shared/session-frame";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../stays-on-agents-shared/transcript";
import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import {
  DeletedAgentMarker,
  MentionComposer,
  OldNotification,
  UnresolvedComposer,
} from "./identity-views";
import { ActivityPage } from "./activity-page";
import { AwayDigest } from "./away-digest";
import {
  ApprovalDock,
  ApprovalsWaiting,
  ExpiredApproval,
  FailureReport,
  InterruptedReport,
  MainSession,
  Proposal,
  ProviderEnded,
  RotatedMarker,
  RotationDeferredMarker,
  RotationFailedReport,
  SupersededNotice,
} from "./session-views";
import { ShoulderTap } from "./report-entry";
import { RAIL } from "./report-rail";

export const meta = definePrototype({
  title: "Stays-on reports",
  spec: "always-on-agent",
  surface: "S11 S12 S13 S14",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "conversation", label: "S11 · An ordinary day" },
    { id: "proposal", label: "S11 · Proposal" },
    { id: "proposal-one", label: "S11 · Proposal, after you pick" },
    { id: "away", label: "S11 · A lot happened while you were away" },
    { id: "failure", label: "S11 · Failure report" },
    { id: "approval", label: "S11 · Relayed child approval" },
    { id: "expired-approval", label: "S11 · Expired approval" },
    { id: "provider-ended", label: "S11 · Provider-ended approval" },
    { id: "interrupted", label: "S11 · Interrupted run" },
    { id: "rotation", label: "S11 · After rotation" },
    { id: "rotation-failed", label: "S11 · Rotation failed" },
    { id: "rotation-deferred", label: "S11 · Rotation waiting" },
    { id: "superseded", label: "S11 · Superseded, read-only" },
    { id: "maybe-not-received", label: "S11 · May not have been received" },
    { id: "deleted-agent", label: "S12 · Deleted agent" },
    { id: "old-notification", label: "S12 · Old notification payload" },
    { id: "mention-menu", label: "S13 · Agent mention" },
    { id: "mention-unresolved", label: "S13 · Unresolved chip" },
    { id: "audit", label: "S14 · Nobody watching" },
    { id: "audit-all", label: "S14 · Everything" },
    { id: "audit-unknown", label: "S14 · Unknown initiator" },
    { id: "audit-empty", label: "S14 · Nothing ran on its own" },
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

const STATE_VIEWS: Record<string, ReactNode> = {
  conversation: <MainSession state="working" />,
  "proposal-one": (
    <MainSession sent="Unblock the token migration">
      <ShoulderTap
        agent={TRIAGE}
        ask=""
        choices={[]}
        chosen="Unblock the token migration"
        when="23:30"
      />
    </MainSession>
  ),
  away: (
    <MainSession pending={1} state="needs-you">
      <AwayDigest />
      <Proposal when="08:02" />
    </MainSession>
  ),
  failure: (
    <MainSession state="failed">
      <FailureReport />
    </MainSession>
  ),
  approval: (
    <MainSession above={<ApprovalDock />} pending={2} state="needs-you">
      <ApprovalsWaiting />
    </MainSession>
  ),
  "expired-approval": (
    <MainSession>
      <ExpiredApproval />
    </MainSession>
  ),
  "provider-ended": (
    <MainSession>
      <ProviderEnded />
    </MainSession>
  ),
  interrupted: (
    <MainSession>
      <InterruptedReport />
    </MainSession>
  ),
  rotation: <MainSession history={<RotatedMarker />} />,
  "rotation-failed": (
    <MainSession>
      <RotationFailedReport />
    </MainSession>
  ),
  "rotation-deferred": (
    <MainSession>
      <RotationDeferredMarker />
    </MainSession>
  ),
  superseded: <MainSession readOnly={<SupersededNotice />} />,
  "maybe-not-received": (
    <MainSession state="needs-you">
      <Proposal footnote="This may not have reached Triage Lead. If you picked one earlier, check its reply first." />
    </MainSession>
  ),
  "deleted-agent": <DeletedAgentSession />,
  "old-notification": <DeletedAgentSession notify />,
  "mention-menu": <MainSession composer={<MentionComposer />} />,
  "mention-unresolved": <MainSession composer={<UnresolvedComposer />} />,
  audit: <ActivityPage initial="unattended" />,
  "audit-all": <ActivityPage initial="all" />,
  "audit-unknown": <ActivityPage initial="all" legacy />,
  "audit-empty": <ActivityPage empty initial="unattended" />,
};

export default function StaysOnReportsDraft({ state }: PrototypeProps) {
  return (
    STATE_VIEWS[state] ?? (
      <MainSession state="needs-you">
        <Proposal />
      </MainSession>
    )
  );
}
