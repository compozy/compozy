import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { Button } from "@compozy/ui";

import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";

import { deriveAwayDigest } from "../../../lib/session-away";
import {
  AWAY_FIXTURE,
  AWAY_NOW,
  REPORT_CHOICES,
  REPORT_ENGINEER,
  REPORT_TRIAGE,
} from "../../../mocks/report-fixtures";
import { SessionReportSendContext } from "../../../lib/session-report-send";
import { AwayDigest } from "./away-digest";
import {
  ContextBlockedReport,
  ContextFullReport,
  ContextRotatedNotice,
  PauseNotice,
  PauseRecord,
  ResumeButton,
  ResumedReceipt,
  WakeLostReport,
} from "./pause-reports";
import { RotatedMarker } from "./rotated-marker";
import { Notice, SettledAsk, ShoulderTap } from "./report-entry";
import { SupersededNotice } from "./superseded-notice";

export const meta = definePrototype({
  title: "Session reports",
  spec: "always-on-agent",
  surface: "S11",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "proposal", label: "Proposal" },
    { id: "proposal-one", label: "Proposal, after you pick" },
    { id: "maybe-not-received", label: "May not have been received" },
    { id: "away", label: "A lot happened while you were away" },
    { id: "failure", label: "Failure report" },
    { id: "expired-approval", label: "Expired approval" },
    { id: "provider-ended", label: "Provider-ended approval" },
    { id: "interrupted", label: "Interrupted run" },
    { id: "rotation-failed", label: "Rotation failed" },
    { id: "superseded", label: "Superseded, read-only" },
    { id: "pause", label: "S2 · Paused at the limit" },
    { id: "pause-resuming", label: "S2 · Resume pending" },
    { id: "pause-resume-failed", label: "S2 · Resume failed" },
    { id: "pause-resumed", label: "S2 · After you resume" },
    { id: "pause-record", label: "S2 · Paused since for another reason" },
    { id: "away-paused", label: "S3 · Paused while you were away" },
    { id: "expired-action", label: "S8 · Expired, tool and action" },
    { id: "wake-lost", label: "A wake-up was lost" },
    { id: "context-full", label: "Context full" },
    { id: "context-blocked", label: "Context full, can't move yet" },
    { id: "context-rotated", label: "Context full, new conversation" },
  ],
});

const ask = "I found two things worth doing. Which one first?";

function Tap({ chosen, footnote }: { chosen?: string; footnote?: string }) {
  return (
    <ShoulderTap
      agent={REPORT_TRIAGE}
      ask={ask}
      choices={REPORT_CHOICES}
      chosen={chosen}
      footnote={footnote}
      onChoose={() => undefined}
      when="23:30"
    />
  );
}

function Pause({ pending, failed }: { pending?: boolean; failed?: boolean }) {
  return (
    <PauseNotice
      action={<ResumeButton failed={failed} onResume={() => undefined} pending={pending} />}
      agent={REPORT_TRIAGE}
      held={2}
      steps={300}
      when="04:12"
    />
  );
}

const PAUSED_AWAY = {
  ...AWAY_FIXTURE,
  reports: [
    ...AWAY_FIXTURE.reports,
    {
      id: "arpt-pause",
      kind: "failure" as const,
      reason: "unattended",
      state: "pending" as const,
      created_at: "2026-11-01T04:12:00Z",
    },
  ],
};

function Settled({ provider = false }: { provider?: boolean }) {
  return (
    <SettledAsk
      actions={
        <Button size="sm" type="button" variant="secondary">
          Ask it to try again
        </Button>
      }
      agent={REPORT_ENGINEER}
      outcome={provider ? "Didn't happen" : "Didn't run"}
      session="Migrate tokens"
      subject="make migrate"
      title={`Code Engineer's request ${provider ? "ended early" : "went unanswered"}`}
      when="11:30"
      why={
        provider
          ? "The agent stopped waiting for an answer before you got to it."
          : "Nobody answered in time, so it wasn't allowed."
      }
    />
  );
}

function View({ state }: { state?: string }) {
  const digest = deriveAwayDigest(AWAY_FIXTURE, REPORT_TRIAGE.label, AWAY_NOW);
  switch (state) {
    case "proposal-one":
      return <Tap chosen="Unblock the token migration" />;
    case "maybe-not-received":
      return (
        <Tap footnote="This may not have reached Triage Lead. If you picked one earlier, check its reply first." />
      );
    case "away":
      return (
        <>
          {digest ? <AwayDigest agent={REPORT_TRIAGE} digest={digest} /> : null}
          <Tap />
        </>
      );
    case "failure":
      return (
        <Notice
          actions={
            <Button size="sm" type="button" variant="secondary">
              Try again
            </Button>
          }
          agent={REPORT_TRIAGE}
          body="The check took longer than 10 minutes, so I stopped it. That's 3 in a row."
          state="failed"
          title="I couldn't finish this check"
          when="23:30"
        />
      );
    case "expired-approval":
      return <Settled />;
    case "provider-ended":
      return <Settled provider />;
    case "interrupted":
      return (
        <Notice
          actions={
            <Button size="sm" type="button" variant="secondary">
              Pick it back up
            </Button>
          }
          agent={REPORT_TRIAGE}
          body="CompozyOS restarted while Code Engineer was waiting for your answer, so its work stopped there."
          title="Code Engineer's work stopped"
          when="08:02"
        />
      );
    case "rotation-failed":
      return (
        <Notice
          agent={REPORT_TRIAGE}
          body="I tried to start a fresh conversation and couldn't, so we stay here. You don't need to do anything."
          title="Still in this conversation"
          when="06:40"
        />
      );
    case "superseded":
      return <SupersededNotice onOpenCurrent={() => undefined} />;
    case "pause":
      return <Pause />;
    case "pause-resuming":
      return <Pause pending />;
    case "pause-resume-failed":
      return <Pause failed />;
    case "pause-record":
      return <PauseRecord agent={REPORT_TRIAGE} steps={300} when="04:12" />;
    case "pause-resumed":
      return <ResumedReceipt agent={REPORT_TRIAGE} />;
    case "away-paused": {
      const paused = deriveAwayDigest(PAUSED_AWAY, REPORT_TRIAGE.label, AWAY_NOW, [], {
        held: 2,
        pausedNow: true,
      });
      return paused ? (
        <AwayDigest
          action={<ResumeButton onResume={() => undefined} />}
          agent={REPORT_TRIAGE}
          digest={paused}
        />
      ) : null;
    }
    case "expired-action":
      return (
        <SettledAsk
          actions={
            <Button size="sm" type="button" variant="secondary">
              Ask it to try again
            </Button>
          }
          agent={REPORT_ENGINEER}
          outcome="Didn't run"
          session="Migrate tokens"
          subject="make migrate"
          title="Code Engineer's request went unanswered"
          when="11:30"
          why="Nobody answered in time, so the Bash request wasn't allowed."
        />
      );
    case "wake-lost":
      return <WakeLostReport agent={REPORT_TRIAGE} detail="CI on #1166" when="03:02" />;
    case "context-full":
      return <ContextFullReport agent={REPORT_TRIAGE} when="05:48" />;
    case "context-blocked":
      return (
        <ContextBlockedReport
          agent={REPORT_TRIAGE}
          blockers={["live_descendant", "queued_input"]}
          when="05:48"
        />
      );
    case "context-rotated":
      return (
        <>
          <ContextRotatedNotice agent={REPORT_TRIAGE} when="05:48" />
          <RotatedMarker predecessorId="sess-before" />
        </>
      );
    default:
      return <Tap />;
  }
}

export default function SessionReportsProto({ state }: PrototypeProps) {
  return (
    <SessionReportSendContext value={() => undefined}>
      <ThreadContentRail inset="px-8">
        <View state={state} />
      </ThreadContentRail>
    </SessionReportSendContext>
  );
}
