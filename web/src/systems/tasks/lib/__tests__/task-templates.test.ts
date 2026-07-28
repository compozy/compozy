import { describe, expect, it } from "vitest";

import {
  DEFAULT_TASK_TEMPLATE_ID,
  TASK_TEMPLATES,
  type TaskTemplateBadgeTone,
  applyTemplateToCreatePayload,
  getTaskTemplate,
} from "../task-templates";

const ALLOWED_BADGE_TONES = new Set<TaskTemplateBadgeTone>([
  "neutral",
  "accent",
  "info",
  "warning",
]);

describe("task-templates", () => {
  it("exposes the default template id and includes it in the catalog", () => {
    expect(DEFAULT_TASK_TEMPLATE_ID).toBe("one_shot");
    expect(TASK_TEMPLATES.find(template => template.id === DEFAULT_TASK_TEMPLATE_ID)).toBeDefined();
  });

  it("returns the requested template by id", () => {
    expect(getTaskTemplate("recurring").label).toBe("Recurring via automation");
    expect(getTaskTemplate("epic").defaults.priority).toBe("high");
    expect(getTaskTemplate("human_in_loop").defaults.approval_policy).toBe("manual");
  });

  it("falls back to the one-shot template when an unknown id is requested", () => {
    expect(getTaskTemplate("unknown" as never).id).toBe("one_shot");
  });

  it("Should restrict every template badge tone to the accent / info / warning / neutral vocabulary", () => {
    for (const template of TASK_TEMPLATES) {
      for (const badge of template.badges) {
        expect(ALLOWED_BADGE_TONES.has(badge.tone)).toBe(true);
      }
    }
  });

  it("Should not carry any `violet` or `amber` legacy tone on template badges", () => {
    const tones = TASK_TEMPLATES.flatMap(template => template.badges.map(badge => badge.tone));
    expect(tones).not.toContain("violet");
    expect(tones).not.toContain("amber");
  });

  it("merges template defaults into the create payload without overriding explicit values", () => {
    const payload = applyTemplateToCreatePayload(
      { title: "Sample", scope: "workspace" },
      "human_in_loop"
    );

    expect(payload.priority).toBe("high");
    expect(payload.approval_policy).toBe("manual");
    expect(payload.draft).toBe(false);

    const overridden = applyTemplateToCreatePayload(
      {
        title: "Sample",
        scope: "workspace",
        priority: "low",
        approval_policy: "none",
        draft: true,
      },
      "human_in_loop"
    );

    expect(overridden.priority).toBe("low");
    expect(overridden.approval_policy).toBe("none");
    expect(overridden.draft).toBe(true);
  });
});
