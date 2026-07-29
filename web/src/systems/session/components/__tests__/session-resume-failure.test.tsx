import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SessionResumeFailure } from "../session-resume-failure";

describe("SessionResumeFailure", () => {
  it("renders the session-level banner with the provider and ids as body text", () => {
    render(
      <SessionResumeFailure
        agentName="claude-agent"
        isRetrying={false}
        message="session: validate attach infrastructure"
        missingProvider="codex"
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        sessionId="sess_123"
      />
    );

    expect(screen.getByTestId("session-resume-failure")).toBeInTheDocument();
    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent(
      "Attach failed: provider no longer available"
    );
    // The provider reads inside the body sentence — no id pills, no chips.
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "provider codex"
    );
    const meta = screen.getByTestId("session-resume-failure-meta");
    expect(meta.className).toContain("font-mono");
    expect(meta).toHaveTextContent("sess_123");
    expect(meta).toHaveTextContent("claude-agent");
    expect(meta.textContent).not.toMatch(/SESS_123|CLAUDE-AGENT/);
  });

  it("falls back to the raw message when no provider could be parsed", () => {
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="Attach failed unexpectedly."
        missingProvider={null}
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        sessionId="sess_456"
      />
    );

    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent("Attach failed");
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "Attach failed unexpectedly."
    );
    expect(screen.queryByTestId("session-resume-failure-provider")).not.toBeInTheDocument();
  });

  it("falls back to the raw message when the provider detail is only whitespace", () => {
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="Attach failed unexpectedly."
        missingProvider="   "
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        sessionId="sess_trimmed"
      />
    );

    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent("Attach failed");
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "Attach failed unexpectedly."
    );
    expect(screen.queryByTestId("session-resume-failure-provider")).not.toBeInTheDocument();
  });

  it("does not render the agent metadata when the agent name is only whitespace", () => {
    render(
      <SessionResumeFailure
        agentName="   "
        isRetrying={false}
        message="Attach failed unexpectedly."
        missingProvider="codex"
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        sessionId="sess_trimmed_meta"
      />
    );

    expect(screen.getByTestId("session-resume-failure-meta")).toHaveTextContent(
      "sess_trimmed_meta"
    );
    expect(screen.getByTestId("session-resume-failure-meta")).not.toHaveTextContent(/\bagent\b/i);
  });

  it("invokes retry and dismiss callbacks", () => {
    const onRetry = vi.fn();
    const onDismiss = vi.fn();
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="Attach failed."
        missingProvider="codex"
        onDismiss={onDismiss}
        onRetry={onRetry}
        sessionId="sess_789"
      />
    );

    fireEvent.click(screen.getByTestId("session-resume-failure-retry"));
    fireEvent.click(screen.getByTestId("session-resume-failure-dismiss"));
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("keeps the banner open when another control already handled Escape", () => {
    const onDismiss = vi.fn();
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="Attach failed."
        missingProvider="codex"
        onDismiss={onDismiss}
        onRetry={vi.fn()}
        sessionId="sess_escape"
      />
    );

    const event = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
    event.preventDefault();
    document.dispatchEvent(event);

    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("dismisses the banner when Escape was not handled", () => {
    const onDismiss = vi.fn();
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="Attach failed."
        missingProvider="codex"
        onDismiss={onDismiss}
        onRetry={vi.fn()}
        sessionId="sess_escape_unhandled"
      />
    );

    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true })
    );

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("disables retry while an attach attempt is in flight", () => {
    render(
      <SessionResumeFailure
        isRetrying
        message="Attach failed."
        missingProvider="codex"
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        sessionId="sess_spin"
      />
    );

    expect(screen.getByTestId("session-resume-failure-retry")).toBeDisabled();
    expect(screen.getByTestId("session-resume-failure-retry").querySelector("svg")).toHaveClass(
      "animate-spin"
    );
  });
});
