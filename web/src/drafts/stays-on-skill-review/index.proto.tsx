import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { Notice } from "../stays-on-reports/report-entry";
import { MainSession } from "../stays-on-reports/session-views";
import { TRIAGE } from "../stays-on-agents-shared/fixtures";
import { AUTOMATIC_SKILL, HIDDEN_TEXT_SKILL, PLAIN_SKILL } from "./fixtures";
import { SkillReviewCard } from "./skill-review-card";

export const meta = definePrototype({
  title: "Stays-on skill review",
  spec: "stays-on-hardening",
  surface: "S9",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "instructions", label: "S9 · Instructions only" },
    { id: "automatic", label: "S9 · Something runs automatically" },
    { id: "viewer", label: "S9 · Reading the skill" },
    { id: "viewer-script", label: "S9 · Reading a script" },
    { id: "viewer-hidden", label: "S9 · Text that Read doesn't show" },
    { id: "stale", label: "S9 · Changed since shown" },
    { id: "two", label: "S9 · Two waiting" },
    { id: "limit", label: "S9 · Five already waiting" },
  ],
});

export default function StaysOnSkillReviewDraft({ state }: PrototypeProps) {
  if (state === "automatic") {
    return (
      <MainSession pending={1} state="needs-you">
        <SkillReviewCard skill={AUTOMATIC_SKILL} />
      </MainSession>
    );
  }
  if (state === "viewer") {
    return (
      <MainSession pending={1} state="needs-you">
        <SkillReviewCard defaultOpen skill={PLAIN_SKILL} />
      </MainSession>
    );
  }
  if (state === "viewer-hidden") {
    return (
      <MainSession pending={1} state="needs-you">
        <SkillReviewCard defaultOpen skill={HIDDEN_TEXT_SKILL} />
      </MainSession>
    );
  }
  if (state === "viewer-script") {
    return (
      <MainSession pending={1} state="needs-you">
        <SkillReviewCard defaultOpen initialPath="scripts/sync-board.sh" skill={AUTOMATIC_SKILL} />
      </MainSession>
    );
  }
  if (state === "stale") {
    return (
      <MainSession pending={1} state="needs-you">
        <SkillReviewCard skill={PLAIN_SKILL} variant="stale" />
      </MainSession>
    );
  }
  if (state === "two") {
    return (
      <MainSession pending={2} state="needs-you">
        <SkillReviewCard skill={PLAIN_SKILL} when="09:02" />
        <SkillReviewCard skill={AUTOMATIC_SKILL} />
      </MainSession>
    );
  }
  if (state === "limit") {
    return (
      <MainSession pending={5} state="needs-you">
        <Notice
          agent={TRIAGE}
          body="Five skills are already waiting for you, so I'll hold this one until you've looked at them."
          state="idle"
          title="I couldn't add another skill"
          when="09:40"
        />
      </MainSession>
    );
  }
  return (
    <MainSession pending={1} state="needs-you">
      <SkillReviewCard skill={PLAIN_SKILL} />
    </MainSession>
  );
}
