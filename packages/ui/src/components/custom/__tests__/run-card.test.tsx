import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RUN_STATUS_LABEL, RUN_STATUS_TONE, RunCard } from "../run-card";

const RUN_ID = "run_2026_05_11_abc";

describe("RunCard", () => {
  it("Should render the pill row + 4-col grid in CHANNEL / QUEUED / STARTED / ELAPSED order", () => {
    const { container } = render(
      <RunCard
        status="in_progress"
        runId={RUN_ID}
        sessionInfo="session 42"
        attempt={3}
        channel="cli"
        queuedAt="2026-05-11T12:00:00Z"
        startedAt="2026-05-11T12:00:05Z"
        elapsed="3m 42s"
      />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="run-card"]');
    expect(root?.dataset.status).toBe("in_progress");

    const status = container.querySelector<HTMLElement>('[data-slot="run-card-status"]');
    expect(status?.dataset.tone).toBe(RUN_STATUS_TONE.in_progress);
    expect(status?.textContent).toBe(RUN_STATUS_LABEL.in_progress);

    const monoId = container.querySelector<HTMLElement>('[data-slot="run-card-id"]');
    expect(monoId?.textContent).toContain(RUN_ID.toLowerCase());

    const session = container.querySelector<HTMLElement>('[data-slot="run-card-session-info"]');
    expect(session?.textContent).toBe("session 42");

    const attempt = container.querySelector<HTMLElement>('[data-slot="run-card-attempt"]');
    expect(attempt?.textContent).toBe("attempt 3");

    const grid = container.querySelector<HTMLElement>('[data-slot="run-card-grid"]');
    const labelSlots = Array.from(
      grid?.querySelectorAll<HTMLElement>("[data-slot$='-label']") ?? []
    ).map(el => el.textContent?.trim());
    expect(labelSlots).toEqual(["CHANNEL", "QUEUED", "STARTED", "ELAPSED"]);
  });

  it("Should map every RunCardStatus to its expected PillTone", () => {
    expect(RUN_STATUS_TONE).toEqual({
      pending: "neutral",
      in_progress: "info",
      needs_attention: "warning",
      completed: "success",
      failed: "danger",
      canceled: "neutral",
    });
  });

  it("Should render <Time> components for queued/started timestamps", () => {
    const { container } = render(
      <RunCard
        status="completed"
        runId={RUN_ID}
        queuedAt="2026-05-11T12:00:00Z"
        startedAt="2026-05-11T12:00:05Z"
      />
    );
    const queued = container.querySelector<HTMLElement>('[data-slot="run-card-queued-value"] time');
    expect(queued).not.toBeNull();
    expect(queued?.getAttribute("datetime")).toBe("2026-05-11T12:00:00Z");

    const started = container.querySelector<HTMLElement>(
      '[data-slot="run-card-started-value"] time'
    );
    expect(started?.getAttribute("datetime")).toBe("2026-05-11T12:00:05Z");
  });

  it("Should render an inline warning tinted by tone when present", () => {
    const { container } = render(
      <RunCard
        status="in_progress"
        runId={RUN_ID}
        warning={{ tone: "warning", message: "Awaiting permission" }}
      />
    );
    const warning = container.querySelector<HTMLElement>('[data-slot="run-card-warning"]');
    expect(warning?.dataset.tone).toBe("warning");
    expect(warning?.textContent).toBe("Awaiting permission");
  });

  it("Should render em-dash placeholders for missing fields", () => {
    const { container } = render(<RunCard status="pending" runId={RUN_ID} />);
    const queuedValue = container.querySelector<HTMLElement>('[data-slot="run-card-queued-value"]');
    expect(queuedValue?.textContent).toBe("—");
  });
});
