import { describe, expect, it } from "vitest";

import { buildTaskTimelineItemFixture } from "../../mocks/fixtures";
import { humanizeTaskEvent } from "../task-activity-copy";

describe("humanizeTaskEvent", () => {
  it.each(["task.run.completed", "task.run_completed"])(
    "Should humanize the canonical completion alias %s as run activity",
    eventType => {
      const view = humanizeTaskEvent(
        buildTaskTimelineItemFixture({ event_type: eventType, run: { attempt: 3 } as never })
      );

      expect(view).toMatchObject({ title: "Attempt 3 completed", category: "runs" });
    }
  );

  it("Should attribute audited changes to actor.ref and trim payload copy", () => {
    const view = humanizeTaskEvent(
      buildTaskTimelineItemFixture({
        event_type: "task.updated",
        actor: { kind: "human", ref: "operator@example.com" },
        origin: { kind: "cli", ref: "agh-cli" },
        payload: { message: "  Scope updated  " },
      })
    );

    expect(view).toEqual({
      title: "Details updated by operator@example.com",
      detail: "Scope updated",
      category: "changes",
    });
  });
});
