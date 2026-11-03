import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";

import {
  SKILL_REVIEW_AUTOMATIC,
  SKILL_REVIEW_HIDDEN_TEXT,
  SKILL_REVIEW_PLAIN,
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
    { id: "stale", label: "S9 · Changed since shown" },
    { id: "approved", label: "S9 · Approved" },
    { id: "rejected", label: "S9 · Rejected" },
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

export default function SkillReviewPrototype({ state }: PrototypeProps) {
  switch (state) {
    case "automatic":
      return card(skillReviewViewFixture(SKILL_REVIEW_AUTOMATIC));
    case "viewer":
      return card(skillReviewViewFixture(SKILL_REVIEW_PLAIN), { defaultOpen: true });
    case "viewer-script":
      return card(skillReviewViewFixture(SKILL_REVIEW_AUTOMATIC), {
        defaultOpen: true,
        initialPath: "scripts/sync-board.sh",
      });
    case "viewer-hidden":
      return card(skillReviewViewFixture(SKILL_REVIEW_HIDDEN_TEXT), { defaultOpen: true });
    case "stale":
      return card(skillReviewViewFixture(SKILL_REVIEW_PLAIN, { stale: true }));
    case "approved":
      return card(skillReviewViewFixture({ ...SKILL_REVIEW_PLAIN, state: "approved" }));
    case "rejected":
      return card(skillReviewViewFixture({ ...SKILL_REVIEW_PLAIN, state: "rejected" }));
    case "event-proposal":
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
    default:
      return card(skillReviewViewFixture(SKILL_REVIEW_PLAIN));
  }
}
