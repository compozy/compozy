import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AgentEventPayload, RuntimeActivityPayload } from "../../types";
import {
  isOperationalStatusEvent,
  isSessionErrorEvent,
  isRuntimeActivityEvent,
  isTranscriptMarkerEvent,
} from "../runtime-activity-notice.logic";
import { RuntimeActivityNotice } from "../runtime-activity-notice";

const runtime: RuntimeActivityPayload = {
  turn_id: "turn_001",
  turn_source: "user",
  current_tool: "Bash",
  idle_seconds: 42,
  elapsed_ms: 660_000,
  elapsed_seconds: 660,
  last_activity_kind: "tool_call",
  last_activity_detail: "running command",
};

describe("RuntimeActivityNotice", () => {
  it("recognizes runtime progress and warning events only when activity exists", () => {
    expect(isRuntimeActivityEvent({ type: "runtime_progress", runtime })).toBe(true);
    expect(isRuntimeActivityEvent({ type: "runtime_warning", runtime })).toBe(true);
    expect(isRuntimeActivityEvent({ type: "runtime_progress" })).toBe(false);
    expect(isRuntimeActivityEvent({ type: "agent_message", runtime })).toBe(false);
  });

  it("recognizes fatal session errors when error or failure details exist", () => {
    expect(
      isSessionErrorEvent({
        type: "error",
        error: '{"code":-32603,"message":"Internal error"}',
      })
    ).toBe(true);
    expect(isSessionErrorEvent({ type: "error", failure: { kind: "process_exit" } })).toBe(false);
    expect(
      isSessionErrorEvent({
        type: "error",
        failure: { kind: "process_exit", summary: "peer disconnected before response" },
      })
    ).toBe(true);
    expect(isSessionErrorEvent({ type: "runtime_warning", error: "failed" })).toBe(false);
  });

  it("recognizes transcript marker events", () => {
    expect(isTranscriptMarkerEvent({ type: "transcript_marker.created" })).toBe(true);
    expect(isTranscriptMarkerEvent({ type: "transcript_marker.redacted" })).toBe(true);
    expect(isTranscriptMarkerEvent({ type: "runtime_warning" })).toBe(false);
  });

  it("recognizes prompt lifecycle events as operational status", () => {
    for (const type of [
      "prompt_queued",
      "prompt_steered",
      "prompt_accepted",
      "prompt_dropped",
      "canceled",
    ]) {
      expect(isOperationalStatusEvent({ type })).toBe(true);
    }
    expect(isOperationalStatusEvent({ type: "runtime_warning" })).toBe(false);
  });

  it("renders progress as a quiet neutral marker line", () => {
    const event: AgentEventPayload = {
      type: "runtime_progress",
      text: "Still working",
      runtime,
    };

    render(<RuntimeActivityNotice event={event} />);

    const notice = screen.getByTestId("runtime-activity-notice");
    // Progress is a neutral marker — no signal tone, no tinted card.
    expect(notice).toHaveAttribute("data-tone", "neutral");
    expect(screen.getByText("Still working")).toBeInTheDocument();
    expect(screen.getByTestId("runtime-activity-detail")).toHaveTextContent("Using Bash");
    const meta = screen.getByTestId("runtime-activity-meta");
    expect(meta).toHaveTextContent("11m elapsed · 42s idle");
    expect(meta.className).toContain("font-mono");
  });

  it("renders warnings with alert semantics", () => {
    const event: AgentEventPayload = {
      type: "runtime_warning",
      runtime: {
        ...runtime,
        current_tool: undefined,
        last_activity_detail: "no provider activity observed",
      },
    };

    render(<RuntimeActivityNotice event={event} />);

    expect(screen.getByRole("alert")).toHaveAttribute("data-tone", "warning");
    expect(screen.getByText("Runtime warning")).toBeInTheDocument();
    expect(screen.getByTestId("runtime-activity-detail")).toHaveTextContent(
      "no provider activity observed"
    );
  });

  it("suppresses prompt lifecycle acknowledgements already represented by input state", () => {
    const { container } = render(
      <RuntimeActivityNotice event={{ type: "prompt_queued", text: "Prompt queued" }} />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it.each([
    "prompt_queued",
    "prompt_steered",
    "prompt_accepted",
    "prompt_interrupted",
    "prompt_dropped",
    "prompt_cancel",
  ])("suppresses %s acknowledgement markers represented by input state", markerKind => {
    const { container } = render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          marker: {
            kind: `transcript_marker.${markerKind}`,
            summary: "Prompt lifecycle acknowledgement.",
            occurred_at: "2026-08-03T18:00:00Z",
          },
        }}
      />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders session errors with alert semantics and failure detail", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "error",
          error:
            '{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}',
          failure: {
            kind: "process_exit",
            summary: "peer disconnected before response",
          },
        }}
      />
    );

    expect(screen.getByRole("alert")).toHaveAttribute("data-tone", "danger");
    expect(screen.getByTestId("session-error-notice")).toHaveTextContent("Session failed");
    expect(screen.getByTestId("session-error-meta")).toHaveTextContent("process_exit");
    expect(screen.getByTestId("session-error-detail")).toHaveTextContent(
      "peer disconnected before response"
    );
  });

  it("renders transcript markers with marker semantics", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          text: "Runtime activity timed out.",
          title: "transcript_marker.prompt_timeout",
          raw: {
            kind: "transcript_marker.prompt_timeout",
            occurred_at: "2026-04-20T12:00:00Z",
            summary: "Runtime activity timed out.",
          },
        }}
      />
    );

    expect(screen.getByRole("alert")).toHaveAttribute("data-tone", "danger");
    // The summary is the sentence; the raw kind string is faint mono meta —
    // never a pill, never a "Transcript marker" card title.
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(
      "Runtime activity timed out."
    );
    const kind = screen.getByTestId("transcript-marker-kind");
    expect(kind).toHaveTextContent("transcript_marker.prompt_timeout");
    expect(kind.className).toContain("font-mono");
  });

  it("renders the file-mutation verifier marker with warning semantics", () => {
    const summary =
      "1 file mutation failed and was not recovered in this turn. Verify the affected file before trusting completion claims.";
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          text: summary,
          title: "transcript_marker.file_mutation_unverified",
          raw: {
            kind: "transcript_marker.file_mutation_unverified",
            occurred_at: "2026-04-20T12:05:00Z",
            summary,
            evidence: { failure_count: 1, paths: ["checkout/retry.go"] },
          },
        }}
      />
    );

    const notice = screen.getByTestId("transcript-marker-notice");
    expect(notice).toHaveAttribute("data-tone", "warning");
    expect(screen.getByRole("alert")).toBe(notice);
    expect(screen.getByTestId("transcript-marker-kind")).toHaveTextContent(
      "transcript_marker.file_mutation_unverified"
    );
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(summary);
  });

  it("does not render non-runtime events", () => {
    render(<RuntimeActivityNotice event={{ type: "agent_message", text: "hello" }} />);

    expect(screen.queryByTestId("runtime-activity-notice")).not.toBeInTheDocument();
  });
});
