import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { destroyTerminalInstances } from "@compozy/ui";

import { buildTerminalQuote } from "../../lib/terminal-quote";
import { DEV_SERVER_TERMINAL } from "../../mocks/terminal-fixtures";
import { SessionTerminalBlock } from "../session-terminal-block";
import { TerminalExpiredState } from "../terminal-empty-states";
import { TerminalQuoteBlock, TerminalSelectionActions } from "../terminal-quote-block";
import { stubEngineLoader } from "./terminal-window-harness";

afterEach(() => destroyTerminalInstances(() => true));

/**
 * Canonical suite for what a selection can become (part of UT-117).
 *
 * Invariant: a selection always leads somewhere. With a conversation open it
 * offers to send; without one it offers to pick or start a conversation, and
 * copying the same sourced block either way. The bytes of the block itself are
 * asserted where the serializer lives.
 */

const QUOTE = buildTerminalQuote({
  terminalId: DEV_SERVER_TERMINAL.id,
  fromLine: 214,
  lines: ["12:41:04 [vite] Internal server error"],
});

describe("TerminalSelectionActions", () => {
  it("Should offer sending to the conversation that is already open", async () => {
    const onSendToConversation = vi.fn();
    const onCopy = vi.fn();
    render(
      <TerminalSelectionActions
        hasActiveSession
        onChooseSession={vi.fn()}
        onCopy={onCopy}
        onSendToConversation={onSendToConversation}
        onStartSession={vi.fn()}
      />
    );

    await userEvent.click(screen.getByRole("button", { name: "Send to conversation" }));
    expect(onSendToConversation).toHaveBeenCalledOnce();

    await userEvent.click(screen.getByRole("button", { name: "Copy" }));
    expect(onCopy).toHaveBeenCalledOnce();
  });

  it("Should offer a way in when no conversation is open", async () => {
    const onChooseSession = vi.fn();
    const onStartSession = vi.fn();
    const onCopy = vi.fn();
    render(
      <TerminalSelectionActions
        hasActiveSession={false}
        onChooseSession={onChooseSession}
        onCopy={onCopy}
        onSendToConversation={vi.fn()}
        onStartSession={onStartSession}
      />
    );

    // Never a dead end: the gesture says what is missing and offers both ways
    // to fix it, with copying as the fallback that always works.
    expect(screen.getByTestId("terminal-selection-actions-no-session")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Choose a session…" }));
    expect(onChooseSession).toHaveBeenCalledOnce();

    await userEvent.click(screen.getByRole("button", { name: "Start a session with this quote" }));
    expect(onStartSession).toHaveBeenCalledOnce();

    await userEvent.click(screen.getByRole("button", { name: "Copy as quoted block" }));
    expect(onCopy).toHaveBeenCalledOnce();
  });
});

describe("SessionTerminalBlock", () => {
  it("Should not let two blocks for the same terminal share one screen", async () => {
    const { container } = render(
      <>
        <SessionTerminalBlock
          blockId="tool-call-1"
          engineLoader={stubEngineLoader}
          preview="first"
          terminalId={DEV_SERVER_TERMINAL.id}
          title="dev server"
        />
        <SessionTerminalBlock
          blockId="tool-call-2"
          engineLoader={stubEngineLoader}
          preview="second"
          terminalId={DEV_SERVER_TERMINAL.id}
          title="dev server"
        />
      </>
    );

    // The same command scrolled past twice is two screens. One emulator between
    // them would move its host node from the first to the second and mix their
    // bytes into one.
    await waitFor(() =>
      expect(container.querySelectorAll("[data-slot='terminal-view']")).toHaveLength(2)
    );
    const grids = container.querySelectorAll("[data-slot='terminal-view']");
    expect(grids[0].firstElementChild).not.toBe(grids[1].firstElementChild);
  });

  it("Should say a terminal was cleaned up without inventing how long it waited", () => {
    const { rerender } = render(<TerminalExpiredState />);

    // `[terminal].detached_ttl` is configurable and this surface cannot read
    // it, so it says what happened and leaves the number out.
    expect(screen.getByTestId("terminal-expired")).not.toHaveTextContent(/\d+\s*(hours|h)\b/);

    rerender(<TerminalExpiredState idleFor="6 hours" />);
    expect(screen.getByTestId("terminal-expired")).toHaveTextContent(
      "Nobody was watching for 6 hours"
    );
  });
});

describe("TerminalQuoteBlock", () => {
  it("Should show the terminal and the lines the excerpt was true for", () => {
    render(<TerminalQuoteBlock onRemove={vi.fn()} quote={QUOTE} />);

    const block = screen.getByTestId("terminal-quote-block");
    expect(block).toHaveTextContent(DEV_SERVER_TERMINAL.id);
    // Scrollback numbering shifts as old output is trimmed, which is exactly
    // why the block records the range it was taken from.
    expect(block).toHaveTextContent("214");
  });

  it("Should let the excerpt be taken back out", async () => {
    const onRemove = vi.fn();
    render(<TerminalQuoteBlock onRemove={onRemove} quote={QUOTE} />);

    await userEvent.click(screen.getByRole("button", { name: /remove/i }));

    expect(onRemove).toHaveBeenCalledOnce();
  });
});
