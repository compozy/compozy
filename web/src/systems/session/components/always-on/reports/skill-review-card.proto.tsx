import { definePrototype, type PrototypeProps } from "@compozy/proto";
import type { ReactNode } from "react";

import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";

import {
  SKILL_REVIEW_APPROVED_REVERTIBLE,
  SKILL_REVIEW_APPROVED_SUPERSEDED,
  SKILL_REVIEW_AUTO_APPROVED,
  SKILL_REVIEW_AUTOMATIC,
  SKILL_REVIEW_HIDDEN_TEXT,
  SKILL_REVIEW_LEGACY,
  SKILL_REVIEW_PLAIN,
  SKILL_REVIEW_REVERTED_EDIT,
  skillReviewViewFixture,
} from "@/systems/agent/mocks";

import { REPORT_CHOICES, REPORT_TRIAGE } from "../../../mocks/report-fixtures";
import { ReportEventSourceLine } from "./report-event-source";
import { ShoulderTap } from "./report-entry";
import { SkillReviewCardView } from "./skill-review-card-view";

export const meta = definePrototype({
  title: "Skill review and event proposals",
  spec: "stays-on-hardening",
  surface: "S9 S13",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "instructions", label: "S9 · Instructions only" },
    { id: "automatic", label: "S9 · Something runs automatically" },
    { id: "viewer", label: "S9 · Reading the skill" },
    { id: "viewer-script", label: "S9 · Reading a script" },
    { id: "viewer-hidden", label: "S9 · Text that Read doesn't show" },
    { id: "viewer-legacy", label: "S2 · No lesson recorded" },
    { id: "viewer-evidence-unavailable", label: "S2 · Evidence session unavailable" },
    { id: "stale", label: "S9 · Changed since shown" },
    { id: "approved", label: "S9 · Approved" },
    { id: "rejected", label: "S9 · Rejected" },
    { id: "approved-revert", label: "S1 · Approved, revertible" },
    { id: "auto-approved", label: "S1 · Approved automatically" },
    { id: "approved-superseded", label: "S1 · Approved, a newer change exists" },
    { id: "revert-conflict", label: "S1 · Revert refused" },
    { id: "reverted", label: "S1 · Reverted edit" },
    { id: "event-proposal", label: "S13 · Proposal from an event" },
  ],
});

function card(view: Parameters<typeof SkillReviewCardView>[0]["view"], extra = {}) {
  return (
    <ThreadContentRail inset="px-8">
      <SkillReviewCardView agent={REPORT_TRIAGE} view={view} when="09:14" {...extra} />
    </ThreadContentRail>
  );
}

function EventProposal() {
  return (
    <ThreadContentRail inset="px-8">
      <ShoulderTap
        agent={REPORT_TRIAGE}
        ask="I found two things worth doing. Which one first?"
        choices={REPORT_CHOICES}
        onChoose={() => undefined}
        source={
          <ReportEventSourceLine
            source={{
              triggerName: "deploy-failed",
              event: "webhook",
              summary: "Deploy 412 to production failed at the migration step.",
            }}
          />
        }
        when="09:20"
      />
    </ThreadContentRail>
  );
}

const STATES: Readonly<Record<string, () => ReactNode>> = {
  automatic: () => card(skillReviewViewFixture(SKILL_REVIEW_AUTOMATIC)),
  viewer: () => card(skillReviewViewFixture(SKILL_REVIEW_PLAIN), { defaultOpen: true }),
  "viewer-script": () =>
    card(skillReviewViewFixture(SKILL_REVIEW_AUTOMATIC), {
      defaultOpen: true,
      initialPath: "scripts/sync-board.sh",
    }),
  "viewer-hidden": () =>
    card(skillReviewViewFixture(SKILL_REVIEW_HIDDEN_TEXT), { defaultOpen: true }),
  "viewer-legacy": () => card(skillReviewViewFixture(SKILL_REVIEW_LEGACY), { defaultOpen: true }),
  "viewer-evidence-unavailable": () =>
    card(
      skillReviewViewFixture(SKILL_REVIEW_PLAIN, {
        unavailableEvidenceSessionIds: ["sess-4f1c0b2a9d3e4f51"],
      }),
      { defaultOpen: true }
    ),
  stale: () => card(skillReviewViewFixture(SKILL_REVIEW_PLAIN, { stale: true })),
  approved: () => card(skillReviewViewFixture({ ...SKILL_REVIEW_PLAIN, state: "approved" })),
  rejected: () => card(skillReviewViewFixture({ ...SKILL_REVIEW_PLAIN, state: "rejected" })),
  "approved-revert": () => card(skillReviewViewFixture(SKILL_REVIEW_APPROVED_REVERTIBLE)),
  "auto-approved": () => card(skillReviewViewFixture(SKILL_REVIEW_AUTO_APPROVED)),
  "approved-superseded": () => card(skillReviewViewFixture(SKILL_REVIEW_APPROVED_SUPERSEDED)),
  "revert-conflict": () =>
    card(
      skillReviewViewFixture(SKILL_REVIEW_APPROVED_SUPERSEDED, {
        decideError: new Error("A newer change to this skill exists. Revert that one first."),
      })
    ),
  reverted: () => card(skillReviewViewFixture(SKILL_REVIEW_REVERTED_EDIT)),
  "event-proposal": () => <EventProposal />,
};

export default function SkillReviewPrototype({ state }: PrototypeProps) {
  const render = STATES[state];
  return render ? render() : card(skillReviewViewFixture(SKILL_REVIEW_PLAIN));
}
