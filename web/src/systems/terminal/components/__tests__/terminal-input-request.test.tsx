import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ANSWERED_PASSWORD_REQUEST,
  CONFIRMATION_REQUEST,
  EXPIRED_PASSWORD_REQUEST,
  PASSWORD_REQUEST,
  PSQL_TERMINAL,
  REJECTED_PASSWORD_REQUEST,
  SUPERSEDED_PASSWORD_REQUEST,
} from "../../mocks/terminal-fixtures";
import { TerminalInputRequestStack } from "../terminal-input-request-stack";
import { TerminalInputRequestCard, TerminalInputResolvedRow } from "../terminal-input-request";

/**
 * Canonical suite for the input-request surface (UT-114).
 *
 * Invariant: the pin names the requester the daemon published, a redacted
 * answer never reaches the DOM as readable text, a watcher can request an atomic
 * handoff, expired requests omit the write row, and each of the four frozen outcomes renders
 * its own copy including "by you" when a human resolved it.
 */

const ONE_MINUTE_IN = Date.parse(PASSWORD_REQUEST.requested_at) + 60_000;
const CONFIRMATION_ONE_MINUTE_IN = Date.parse(CONFIRMATION_REQUEST.requested_at) + 60_000;

describe("TerminalInputRequestCard", () => {
  it("Should name who is waiting, for what, and how long is left", () => {
    const now = ONE_MINUTE_IN;
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        now={now}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    expect(screen.getByText("claude-code needs a password")).toBeInTheDocument();
    expect(screen.getByText(PASSWORD_REQUEST.reason)).toBeInTheDocument();
    expect(screen.getByText("Password for user atlas:")).toBeInTheDocument();
    expect(
      screen.getByTestId(`terminal-input-request-expiry-${PASSWORD_REQUEST.id}`)
    ).toHaveTextContent("expires in 14m");
  });

  it("Should say a request is expired and omit the write row", () => {
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
    expect(
      screen.queryByTestId(`terminal-input-request-field-${PASSWORD_REQUEST.id}`)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(`terminal-input-request-send-${PASSWORD_REQUEST.id}`)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(`terminal-input-request-decline-${PASSWORD_REQUEST.id}`)
    ).not.toBeInTheDocument();
  });

  it("Should mask a redacted answer and never expose it as readable text", async () => {
    const onAnswer = vi.fn();
    const { container } = render(
      <TerminalInputRequestCard
        canAnswerDirectly
        now={ONE_MINUTE_IN}
        onAnswer={onAnswer}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    const field = screen.getByTestId(`terminal-input-request-field-${PASSWORD_REQUEST.id}`);
    await userEvent.type(field, "hunter2hunter2");

    expect(field).toHaveAttribute("type", "password");
    expect(container.textContent).not.toContain("hunter2hunter2");
    expect(container.innerHTML).not.toContain("hunter2hunter2");

    await userEvent.click(screen.getByTestId(`terminal-input-request-send-${PASSWORD_REQUEST.id}`));

    expect(onAnswer).toHaveBeenCalledWith("hunter2hunter2");
    expect(field).toHaveValue("");
  });

  it("Should render a plain question without masking", async () => {
    const onAnswer = vi.fn();
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        now={CONFIRMATION_ONE_MINUTE_IN}
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

  it("Should offer an atomic handoff to a watcher in the destination profile", () => {
    render(
      <TerminalInputRequestCard
        canAnswerDirectly={false}
        now={ONE_MINUTE_IN}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        request={PASSWORD_REQUEST}
      />
    );

    expect(screen.getByText("claude-code needs a password")).toBeInTheDocument();
    expect(
      screen.getByTestId(`terminal-input-request-field-${PASSWORD_REQUEST.id}`)
    ).toHaveAttribute("type", "password");
    expect(
      screen.getByTestId(`terminal-input-request-send-${PASSWORD_REQUEST.id}`)
    ).toHaveTextContent("Take control & send");
  });

  it("Should let the controller decline without answering", async () => {
    const onReject = vi.fn();
    render(
      <TerminalInputRequestCard
        canAnswerDirectly
        now={ONE_MINUTE_IN}
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
      <TerminalInputRequestStack
        canAnswerDirectly
        now={Date.parse(PASSWORD_REQUEST.requested_at) + 60_000}
        onAnswer={vi.fn()}
        onReject={vi.fn()}
        pending={[PASSWORD_REQUEST, CONFIRMATION_REQUEST]}
        titles={
          new Map([
            [PASSWORD_REQUEST.terminal_id, PSQL_TERMINAL.title],
            [CONFIRMATION_REQUEST.terminal_id, "ssh staging"],
          ])
        }
      />
    );

    expect(screen.getByText(PSQL_TERMINAL.title)).toBeInTheDocument();
    expect(screen.getByText(PSQL_TERMINAL.id)).toBeInTheDocument();
    expect(screen.getByText("ssh staging")).toBeInTheDocument();
  });
});

describe("TerminalInputResolvedRow", () => {
  it("Should state an answered request by length, never by content", () => {
    const { container } = render(<TerminalInputResolvedRow request={ANSWERED_PASSWORD_REQUEST} />);

    expect(screen.getByText("Answered by you")).toBeInTheDocument();
    expect(container.textContent).toContain("hidden input · 10 characters");
    expect(container.textContent).not.toMatch(/hunter2|password\s*=/i);
  });

  it("Should give each remaining outcome its own copy", () => {
    const declined = render(<TerminalInputResolvedRow request={REJECTED_PASSWORD_REQUEST} />);
    expect(declined.container.textContent).toContain("Declined by you");
    expect(declined.container.textContent).toContain("no input is coming");

    const superseded = render(<TerminalInputResolvedRow request={SUPERSEDED_PASSWORD_REQUEST} />);
    expect(superseded.container.textContent).toContain("Superseded");
    expect(superseded.container.textContent).toContain("marina took control");

    const expired = render(<TerminalInputResolvedRow request={EXPIRED_PASSWORD_REQUEST} />);
    expect(expired.container.textContent).toContain("Expired");
    expect(expired.container.textContent).toContain("unanswered for 15 minutes");
  });
});
