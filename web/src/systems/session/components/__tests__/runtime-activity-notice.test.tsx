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

function providerErrorEvent(
  overrides: Partial<NonNullable<AgentEventPayload["provider_error"]>> = {}
): AgentEventPayload {
  return {
    type: "error",
    error: "provider authentication required",
    failure: { kind: "prompt_failure", summary: "provider authentication required" },
    provider_error: {
      code: "provider_auth_required",
      provider: "claude-code",
      next_action: "login",
      guidance: "run provider auth login for this provider",
      occurrence_count: 1,
      first_seen_at: "2026-09-05T14:02:00Z",
      last_seen_at: "2026-09-05T14:02:00Z",
      ...overrides,
    },
  };
}

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

  it("does not read a stop the daemon attributed as a session failure", () => {
    // The live tail carries a supervisor stop as an `error` event whose text is the stop detail.
    const inactivity: AgentEventPayload = {
      type: "error",
      stop_reason: "timeout",
      error: "inactivity",
      timestamp: "2026-09-06T13:55:28.539506Z",
    };
    expect(isSessionErrorEvent(inactivity)).toBe(false);
    expect(
      isSessionErrorEvent({ ...inactivity, stop_reason: "user_canceled", error: "stopped" })
    ).toBe(false);
    // A failure stop reason, a failure record, or a provider diagnostic still is one.
    expect(isSessionErrorEvent({ ...inactivity, stop_reason: "agent_crashed" })).toBe(true);
    expect(
      isSessionErrorEvent({
        ...inactivity,
        failure: { kind: "provider_exit", summary: "exit 137" },
      })
    ).toBe(true);
    const { container } = render(<RuntimeActivityNotice event={inactivity} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("recognizes an actionable provider error even without free-text detail", () => {
    const { error: _error, failure: _failure, ...bare } = providerErrorEvent();
    expect(isSessionErrorEvent(bare)).toBe(true);
    expect(
      isSessionErrorEvent({ ...bare, provider_error: { ...bare.provider_error!, code: "unknown" } })
    ).toBe(false);
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
    // Meta reads as language: sans, with tabular figures so the numbers stay aligned.
    expect(meta.className).not.toContain("font-mono");
    expect(meta.className).toContain("tabular-nums");
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

  it.each(["prompt_queued", "prompt_interrupted", "prompt_dropped", "prompt_cancel"])(
    "suppresses %s acknowledgement markers represented by input state",
    markerKind => {
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
    }
  );

  // VC-07: how the operator's message reached the turn is the one lifecycle
  // line the transcript keeps, from the daemon's own marker evidence.
  it.each([
    ["injected", "steer_ext-injected", "Steered — delivered into the live turn"],
    [
      "pending_injection",
      "pending_injection",
      "Steered — the agent sees it when the current tool finishes",
    ],
    ["interrupt_fallback", "interrupt_fallback", "Steered — interrupted and replaced"],
  ] as const)("renders a steered prompt marker as its %s meta line", (kind, delivery, text) => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.prompt_steered",
            summary: "Steer delivered.",
            occurred_at: "2026-08-03T18:00:00Z",
            evidence: { steer_delivery: delivery === "steer_ext-injected" ? "injected" : delivery },
          },
        }}
      />
    );
    const notice = screen.getByTestId("steer-marker-notice");
    expect(notice).toHaveAttribute("data-steer", kind);
    expect(notice).toHaveAttribute("role", "status");
    expect(screen.getByTestId("steer-marker-text")).toHaveTextContent(text);
  });

  it("renders a superseded steer quieter and a queued prompt with its former position", () => {
    const { unmount } = render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.prompt_superseded",
            summary: "Superseded.",
            occurred_at: "2026-08-03T18:00:00Z",
          },
        }}
      />
    );
    expect(screen.getByTestId("steer-marker-notice")).toHaveAttribute("data-steer", "superseded");
    expect(screen.getByTestId("steer-marker-text")).toHaveTextContent(
      "Superseded by your next steer"
    );
    unmount();

    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.prompt_accepted",
            summary: "Prompt accepted.",
            occurred_at: "2026-08-03T18:00:00Z",
            evidence: { queue_position: 2 },
          },
        }}
      />
    );
    expect(screen.getByTestId("steer-marker-notice")).toHaveAttribute("data-steer", "queued");
    expect(screen.getByTestId("steer-marker-text")).toHaveTextContent("From the queue");
    expect(screen.getByTestId("steer-marker-meta")).toHaveTextContent("was #2");
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

  it("renders an auth lapse as a provider notice naming the provider and the daemon status command", () => {
    render(<RuntimeActivityNotice event={providerErrorEvent()} />);

    const notice = screen.getByTestId("session-error-notice");
    expect(screen.getByRole("alert")).toBe(notice);
    expect(notice).toHaveAttribute("data-tone", "danger");
    expect(notice).toHaveAttribute("data-provider-error", "provider_auth_required");
    expect(notice).toHaveAttribute("data-provider-next-action", "login");
    expect(screen.getByTestId("provider-error-subject")).toHaveTextContent(
      "claude-code needs sign-in"
    );
    // The real recovery command, with a literal placeholder — never an interpolated shell string.
    expect(screen.getByTestId("provider-error-command")).toHaveTextContent(
      "compozy provider auth status <provider> --remote"
    );
    // The session survives: no "Session failed", no raw code or failure kind in the sentence.
    expect(notice).not.toHaveTextContent("Session failed");
    expect(notice).not.toHaveTextContent("provider_auth_required");
    expect(notice).not.toHaveTextContent("prompt_failure");
    expect(screen.queryByTestId("provider-error-occurrence")).not.toBeInTheDocument();
  });

  it.each([
    { next_action: "bind_secret", subject: "claude-code credential needs updating" },
    { next_action: "inspect", subject: "claude-code authentication failed" },
  ])(
    "renders the daemon's $next_action auth recovery without sign-in copy",
    ({ next_action, subject }) => {
      render(<RuntimeActivityNotice event={providerErrorEvent({ next_action })} />);

      const notice = screen.getByTestId("session-error-notice");
      expect(notice).toHaveAttribute("data-provider-next-action", next_action);
      expect(screen.getByTestId("provider-error-subject")).toHaveTextContent(subject);
      expect(notice).not.toHaveTextContent("sign-in");
      expect(notice).not.toHaveTextContent("signed in");
      expect(screen.queryByTestId("provider-error-command")).not.toBeInTheDocument();
    }
  );

  it("renders a rate limit as a wait-then-retry notice with occurrence context on repeats", () => {
    render(
      <RuntimeActivityNotice
        event={providerErrorEvent({
          code: "provider_rate_limited",
          next_action: "retry",
          occurrence_count: 3,
          first_seen_at: "2026-09-05T14:02:00Z",
          last_seen_at: "2026-09-05T14:09:00Z",
        })}
      />
    );

    const notice = screen.getByTestId("session-error-notice");
    expect(notice).toHaveAttribute("data-provider-next-action", "retry");
    expect(screen.getByTestId("provider-error-subject")).toHaveTextContent(
      "claude-code is rate limited"
    );
    expect(screen.getByTestId("session-error-detail")).toHaveTextContent(
      "Wait for the provider to recover, then send your message again."
    );
    const occurrence = screen.getByTestId("provider-error-occurrence");
    expect(occurrence).toHaveTextContent(/^3 times since \d{1,2}:\d{2}/);
    expect(occurrence.className).toContain("tabular-nums");
  });

  it.each([
    { code: "provider_auth_required", subject: "claude-code authentication failed" },
    { code: "provider_rate_limited", subject: "claude-code is rate limited" },
  ])(
    "falls back to neutral inspection for an unknown next_action on $code",
    ({ code, subject }) => {
      render(<RuntimeActivityNotice event={providerErrorEvent({ code, next_action: "wait" })} />);

      const notice = screen.getByTestId("session-error-notice");
      expect(notice).toHaveAttribute("data-provider-next-action", "inspect");
      expect(screen.getByTestId("provider-error-subject")).toHaveTextContent(subject);
      expect(notice).not.toHaveTextContent("sign-in");
    }
  );

  it("keeps the generic failure notice for error events without a known provider diagnostic", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "error",
          error: "peer disconnected before response",
          failure: { kind: "process_exit", summary: "peer disconnected before response" },
          provider_error: null,
        }}
      />
    );

    expect(screen.getByTestId("session-error-notice")).toHaveTextContent("Session failed");
    expect(screen.getByTestId("session-error-meta")).toHaveTextContent("process_exit");
    expect(screen.queryByTestId("provider-error-subject")).not.toBeInTheDocument();
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
    // The summary is the sentence; the raw kind string is faint sans meta —
    // never a pill, never a "Transcript marker" card title.
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(
      "Runtime activity timed out."
    );
    const kind = screen.getByTestId("transcript-marker-kind");
    expect(kind).toHaveTextContent("transcript_marker.prompt_timeout");
    expect(kind.className).not.toContain("font-mono");
    expect(kind.className).toContain("tabular-nums");
  });

  it("renders the post-stop marker as a neutral discard note, never as an alert", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          turn_id: "turn_9f2",
          text: "Late agent output discarded after the session stopped.",
          title: "transcript_marker.post_stop",
          raw: {
            kind: "transcript_marker.post_stop",
            occurred_at: "2026-09-05T12:00:00Z",
            summary: "Late agent output discarded after the session stopped.",
            evidence: { event_type: "agent_message" },
          },
        }}
      />
    );

    const notice = screen.getByTestId("transcript-marker-notice");
    expect(notice).toHaveAttribute("data-tone", "neutral");
    expect(notice).toHaveAttribute("role", "status");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(
      "The agent sent more output after you stopped it — discarded; the reply was not changed."
    );
    expect(screen.getByTestId("transcript-marker-kind")).toHaveTextContent(
      "transcript_marker.post_stop"
    );
  });

  it("renders the queue-cleared marker as a neutral attributed trace", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          turn_id: "turn_9f2",
          text: "Queued input removed by explicit clear.",
          title: "transcript_marker.queue_cleared",
          raw: {
            kind: "transcript_marker.queue_cleared",
            occurred_at: "2026-09-06T12:00:00Z",
            summary: "Queued input removed by explicit clear.",
            evidence: { actor_kind: "user", queue_entry_id: "inp_4d8", queue_status: "canceled" },
          },
        }}
      />
    );

    const notice = screen.getByTestId("transcript-marker-notice");
    expect(notice).toHaveAttribute("data-tone", "neutral");
    expect(notice).toHaveAttribute("role", "status");
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(
      "You cleared the queue — a queued follow-up was removed"
    );
  });

  it("names another actor on the queue-cleared marker", () => {
    render(
      <RuntimeActivityNotice
        event={{
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.queue_cleared",
            occurred_at: "2026-09-06T12:00:00Z",
            summary: "Queued input removed by explicit clear.",
            evidence: { actor_id: "reviewer", actor_kind: "agent" },
          },
        }}
      />
    );
    expect(screen.getByTestId("transcript-marker-summary")).toHaveTextContent(
      "agent reviewer cleared the queue"
    );
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
