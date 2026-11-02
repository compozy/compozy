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
import { AwayDigest } from "./away-digest";
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
    default:
      return <Tap />;
  }
}

export default function SessionReportsProto({ state }: PrototypeProps) {
  return (
    <ThreadContentRail inset="px-8">
      <View state={state} />
    </ThreadContentRail>
  );
}
