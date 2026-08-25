import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  CONFIRMATION_REQUEST,
  PASSWORD_REQUEST,
  PSQL_TERMINAL,
} from "../../mocks/terminal-fixtures";
import { TerminalInputRequestCard, TerminalInputResolvedRow } from "../terminal-input-request";

/**
 * Canonical suite for the input-request surface (UT-114).
 *
 * Invariant: the request names the agent and its reason, a redacted answer never
 * reaches the DOM as readable text, a watcher is offered the one-gesture answer,
 * and each of the four frozen outcomes renders its own copy.
 */

describe("TerminalInputRequestCard", () => {
  it("Should name who is waiting, for what, and how long is left", () => {
    // One minute into the request's frozen fifteen-minute lifetime.
    const now = Date.parse(PASSWORD_REQUEST.requested_at) + 60_000;
    render(
      <TerminalInputRequestCard
        askedBy="Claude Code"
        canAnswerDirectly
        now={now}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    expect(screen.getByText("Claude Code needs a password")).toBeInTheDocument();
    expect(screen.getByText(PASSWORD_REQUEST.reason)).toBeInTheDocument();
    expect(screen.getByText("Password for user atlas:")).toBeInTheDocument();
    // Time left to answer — never time elapsed, which reads as the opposite
    // once the fifteen minutes are gone.
    expect(
      screen.getByTestId(`terminal-input-request-expiry-${PASSWORD_REQUEST.id}`)
    ).toHaveTextContent("expires in 14m");
  });

  it("Should say a request is expired rather than counting past its lifetime", () => {
    const now = Date.parse(PASSWORD_REQUEST.requested_at) + 16 * 60_000;
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        now={now}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    expect(
      screen.getByTestId(`terminal-input-request-expiry-${PASSWORD_REQUEST.id}`)
    ).toHaveTextContent("expired");
    // With no controller named, the title still states what is needed.
    expect(screen.getByText("A password is needed")).toBeInTheDocument();
  });

  it("Should mask a redacted answer and never expose it as readable text", async () => {
    const onAnswer = vi.fn();
    const { container } = render(
      <TerminalInputRequestCard
        canAnswerDirectly
        onAnswer={onAnswer}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    const field = screen.getByTestId(`terminal-input-request-field-${PASSWORD_REQUEST.id}`);
    await userEvent.type(field, "hunter2hunter2");

    expect(field).toHaveAttribute("type", "password");
    // Masked on screen, and absent from the serialized DOM: the field is
    // uncontrolled, so the secret never reaches React state or a `value`
    // attribute that a snapshot or error report could carry off.
    expect(container.textContent).not.toContain("hunter2hunter2");
    expect(container.innerHTML).not.toContain("hunter2hunter2");

    await userEvent.click(screen.getByTestId(`terminal-input-request-send-${PASSWORD_REQUEST.id}`));

    expect(onAnswer).toHaveBeenCalledWith("hunter2hunter2");
    // Sending clears the field: the secret does not linger for the next prompt.
    expect(field).toHaveValue("");
  });

  it("Should render a plain question without masking", async () => {
    const onAnswer = vi.fn();
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        onAnswer={onAnswer}
        onReject={vi.fn()}
        request={CONFIRMATION_REQUEST}
      />
    );

    const field = screen.getByTestId(`terminal-input-request-field-${CONFIRMATION_REQUEST.id}`);
    expect(field).toHaveAttribute("type", "text");

    await userEvent.type(field, "y");
    await userEvent.click(
      screen.getByTestId(`terminal-input-request-send-${CONFIRMATION_REQUEST.id}`)
    );

    expect(onAnswer).toHaveBeenCalledWith("y");
  });

  it("Should offer a watcher one gesture that takes control and sends", () => {
    render(
      <TerminalInputRequestCard
        canAnswerDirectly={false}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    expect(
      screen.getByTestId(`terminal-input-request-send-${PASSWORD_REQUEST.id}`)
    ).toHaveTextContent("Take control & send");
    // Declining needs the keyboard too, so a watcher is not offered it in place.
    expect(
      screen.queryByTestId(`terminal-input-request-decline-${PASSWORD_REQUEST.id}`)
    ).not.toBeInTheDocument();
  });

  it("Should let the controller decline without answering", async () => {
    const onReject = vi.fn();
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        onAnswer={vi.fn()}
        onReject={onReject}
        request={PASSWORD_REQUEST}
      />
    );

    await userEvent.click(
      screen.getByTestId(`terminal-input-request-decline-${PASSWORD_REQUEST.id}`)
    );

    expect(onReject).toHaveBeenCalledOnce();
  });

  it("Should name the terminal when several are asking at once", () => {
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
        showOrigin
        terminalTitle={PSQL_TERMINAL.title}
      />
    );

    expect(screen.getByText(PSQL_TERMINAL.title)).toBeInTheDocument();
    expect(screen.getByText(PSQL_TERMINAL.id)).toBeInTheDocument();
  });
});

describe("TerminalInputResolvedRow", () => {
  it("Should state an answered request by length, never by content", () => {
    const { container } = render(
      <TerminalInputResolvedRow
        outcome="answered"
        redactedLength={10}
        resolvedAt="2026-08-25T12:44:00Z"
      />
    );

    expect(screen.getByText("Answered")).toBeInTheDocument();
    expect(container.textContent).toContain("hidden input, 10 characters");
  });

  it("Should give each remaining outcome its own copy", () => {
    const declined = render(
      <TerminalInputResolvedRow outcome="rejected" resolvedAt="2026-08-25T12:41:00Z" />
    );
    expect(declined.container.textContent).toContain("Declined");
    expect(declined.container.textContent).toContain("no input is coming");

    const superseded = render(
      <TerminalInputResolvedRow
        outcome="superseded"
        resolvedAt="2026-08-25T12:39:00Z"
        supersededBy="marina"
      />
    );
    expect(superseded.container.textContent).toContain("Superseded");
    expect(superseded.container.textContent).toContain("marina took control");

    const expired = render(
      <TerminalInputResolvedRow outcome="expired" resolvedAt="2026-08-25T12:20:00Z" />
    );
    expect(expired.container.textContent).toContain("Expired");
    expect(expired.container.textContent).toContain("unanswered for 15 minutes");
  });
});
