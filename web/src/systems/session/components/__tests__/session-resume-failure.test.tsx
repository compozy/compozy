import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SessionResumeFailure } from "../session-resume-failure";

describe("SessionResumeFailure", () => {
  it("renders the session-level banner with the provider in plain words and ids off screen", () => {
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
    const banner = screen.getByTestId("session-resume-failure");
    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent(
      "Couldn't reconnect to this session"
    );
    // The provider reads inside the body sentence — no id pills, no chips.
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "This session used codex, which isn't set up in this project anymore."
    );
    // Session id and agent stay one step deeper: attributes, not text.
    expect(banner).toHaveAttribute("data-session-id", "sess_123");
    expect(banner).toHaveAttribute("data-agent", "claude-agent");
    expect(banner).not.toHaveTextContent("sess_123");
    expect(banner).not.toHaveTextContent(/attach|daemon|runtime/i);
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

    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent(
      "Couldn't reconnect to this session"
    );
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "Attach failed unexpectedly."
    );
    expect(screen.queryByTestId("session-resume-failure-provider")).not.toBeInTheDocument();
  });

  it("renders a dead runtime as read-only history with a fork action", () => {
    render(
      <SessionResumeFailure
        isRetrying={false}
        message="This session can't continue. Start a copy to keep working — the history stays here."
        missingProvider={null}
        onDismiss={vi.fn()}
        onRetry={vi.fn()}
        retryLabel="Continue in a new session"
        sessionId="sess_dead"
        showDismiss={false}
        title="Session ended"
      />
    );

    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent("Session ended");
    expect(screen.getByTestId("session-resume-failure-message")).toHaveTextContent(
      "the history stays here"
    );
    expect(screen.getByTestId("session-resume-failure-retry")).toHaveTextContent(
      "Continue in a new session"
    );
    expect(screen.queryByTestId("session-resume-failure-dismiss")).not.toBeInTheDocument();
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

    expect(screen.getByTestId("session-resume-failure-title")).toHaveTextContent(
      "Couldn't reconnect to this session"
    );
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

    const banner = screen.getByTestId("session-resume-failure");
    expect(banner).toHaveAttribute("data-session-id", "sess_trimmed_meta");
    expect(banner).not.toHaveAttribute("data-agent");
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
