import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import {
  processesLeftRunningFixture,
  sessionProcessFixture,
  sessionProcessesFixture,
} from "../../mocks/process-fixtures";
import { processIdentity } from "../../lib/session-processes";
import { ProcessesLeftRunningDialog } from "../processes-left-running-dialog";
import { SessionProcessChoiceDialog } from "../session-process-choice-dialog";
import { SessionProcessesComposerControl } from "../session-processes-composer-control";

// Suite: session process controls.
// Invariant: the composer control appears only while the session has processes running; its
// popover and "Processes left running" list each process with its kind and ports, stop one
// process or all of them, and keep every read state; the lifecycle choice answers keep or
// terminate, never both.
// Owning layer: session presentational components (props in, callbacks out). Canonical suite: this file.

function renderControl(props: Partial<Parameters<typeof SessionProcessesComposerControl>[0]> = {}) {
  return render(
    <UIProvider reducedMotion="always">
      <SessionProcessesComposerControl count={3} processes={[]} {...props} />
    </UIProvider>
  );
}

describe("SessionProcessesComposerControl", () => {
  it("Should render nothing while the session has no process running", () => {
    renderControl({ count: 0, processes: sessionProcessesFixture });
    expect(screen.queryByTestId("composer-processes-button")).not.toBeInTheDocument();
  });

  it("Should list each process with its kind and ports and stop one or all", async () => {
    const user = userEvent.setup();
    const onStop = vi.fn();
    const onStopAll = vi.fn();
    const onOpenChange = vi.fn();
    renderControl({ processes: sessionProcessesFixture, onStop, onStopAll, onOpenChange });

    const trigger = screen.getByRole("button", { name: "Processes, 3 running" });
    expect(trigger).toHaveTextContent("3");
    await user.click(trigger);
    expect(onOpenChange).toHaveBeenLastCalledWith(true);

    const server = screen.getByTestId("session-process-76050");
    expect(server).toHaveTextContent("bun run dev --port 5173");
    expect(server).toHaveTextContent(/Background·started 4[23]m ago/);
    expect(within(server).getByRole("link", { name: "localhost:5173" })).toHaveAttribute(
      "href",
      "http://localhost:5173"
    );
    const terminal = screen.getByTestId("session-process-77001");
    expect(terminal).toHaveTextContent("Terminal");
    expect(
      within(terminal)
        .getAllByRole("link")
        .map(link => link.textContent)
    ).toEqual(["localhost:8080", "localhost:9090"]);

    await user.click(
      within(server).getByRole("button", { name: "Stop process: bun run dev --port 5173" })
    );
    expect(onStop).toHaveBeenCalledExactlyOnceWith(sessionProcessesFixture[0]);
    await user.click(screen.getByRole("button", { name: "Stop all" }));
    expect(onStopAll).toHaveBeenCalledOnce();
  });

  it("Should lock the stopping row, and every row while Stop all runs", async () => {
    const user = userEvent.setup();
    const view = renderControl({
      processes: sessionProcessesFixture,
      onStop: vi.fn(),
      onStopAll: vi.fn(),
      stopping: new Set([processIdentity(sessionProcessesFixture[1]!)]),
    });
    await user.click(screen.getByRole("button", { name: "Processes, 3 running" }));
    expect(
      screen.getByRole("button", { name: "Stop process: tsc --watch --preserveWatchOutput" })
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Stop process: bun run dev --port 5173" })
    ).toBeEnabled();

    view.rerender(
      <UIProvider reducedMotion="always">
        <SessionProcessesComposerControl
          count={3}
          processes={sessionProcessesFixture}
          onStop={vi.fn()}
          onStopAll={vi.fn()}
          stopping="all"
        />
      </UIProvider>
    );
    expect(screen.getByTestId("composer-processes-stop-all")).toHaveTextContent("Stopping…");
    for (const button of screen.getAllByRole("button", { name: /^Stop process:/ })) {
      expect(button).toBeDisabled();
    }
  });

  it("Should fold a multi-line command into one row", async () => {
    const user = userEvent.setup();
    renderControl({
      count: 1,
      processes: [sessionProcessFixture({ command: "bash -c 'cat <<EOF\nhello\nEOF'" })],
      onStop: vi.fn(),
    });
    await user.click(screen.getByRole("button", { name: "Processes, 1 running" }));
    expect(screen.getByTestId("session-process-76050")).toHaveTextContent(
      "bash -c 'cat <<EOF ↵ hello ↵ EOF'"
    );
  });

  it.each([
    [{ loading: true }, "Loading processes"],
    [{ failed: true }, "Couldn't load processes."],
    [{ missing: true }, "This session is no longer available."],
    [{ supported: false }, "Processes can't be listed on this system."],
    [{}, "No processes are running."],
  ])("Should show the read state %o", async (state, text) => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderControl({ ...state, onRetry, onStopAll: vi.fn() });
    await user.click(screen.getByRole("button", { name: "Processes, 3 running" }));
    const popover = screen.getByTestId("composer-processes-popover");
    if ("loading" in state) {
      expect(within(popover).getByTestId("session-processes-loading")).toHaveTextContent(text);
    } else {
      expect(popover).toHaveTextContent(text);
    }
    // Stop all needs rows to act on.
    expect(screen.queryByTestId("composer-processes-stop-all")).not.toBeInTheDocument();
    if ("failed" in state) {
      await user.click(within(popover).getByRole("button", { name: "Try again" }));
      expect(onRetry).toHaveBeenCalledOnce();
    }
  });

  it("Should keep cached rows when a refresh fails", async () => {
    const user = userEvent.setup();
    renderControl({ processes: sessionProcessesFixture, failed: true, onRetry: vi.fn() });
    await user.click(screen.getByRole("button", { name: "Processes, 3 running" }));
    expect(screen.getByTestId("session-process-76050")).toBeInTheDocument();
    expect(screen.getByText("Couldn't refresh processes.")).toBeInTheDocument();
  });
});

describe("SessionProcessChoiceDialog", () => {
  it("Should answer keep or terminate for an archive, opening on Cancel", async () => {
    const user = userEvent.setup();
    const onChoose = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <SessionProcessChoiceDialog
          open
          action="archive"
          sessionCount={1}
          processCount={2}
          ports={[5173, 5173, 8080]}
          onOpenChange={vi.fn()}
          onChoose={onChoose}
        />
      </UIProvider>
    );
    const dialog = screen.getByRole("dialog", { name: "Archive this session?" });
    expect(dialog).toHaveTextContent(
      "This session started 2 processes that are still running. Listening on localhost:5173 and localhost:8080."
    );
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.click(within(dialog).getByRole("button", { name: "Keep processes running" }));
    expect(onChoose).toHaveBeenLastCalledWith("keep");
    await user.click(within(dialog).getByRole("button", { name: "Archive and stop 2 processes" }));
    expect(onChoose).toHaveBeenLastCalledWith("terminate");
  });

  it("Should name the batch for a bulk stop", () => {
    render(
      <UIProvider reducedMotion="always">
        <SessionProcessChoiceDialog
          open
          action="stop"
          sessionCount={3}
          processCount={1}
          onOpenChange={vi.fn()}
          onChoose={vi.fn()}
        />
      </UIProvider>
    );
    const dialog = screen.getByRole("dialog", { name: "Stop 3 sessions?" });
    expect(dialog).toHaveTextContent("These sessions started 1 process that is still running.");
    expect(
      within(dialog).getByRole("button", { name: "Stop sessions and 1 process" })
    ).toBeInTheDocument();
  });
});

describe("ProcessesLeftRunningDialog", () => {
  it("Should group processes by session and stop one, a session's, or every process", async () => {
    const user = userEvent.setup();
    const onStop = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <ProcessesLeftRunningDialog
          open
          onOpenChange={vi.fn()}
          groups={processesLeftRunningFixture}
          onStop={onStop}
        />
      </UIProvider>
    );
    const archived = screen.getByRole("region", { name: "Landing page polish" });
    expect(archived).toHaveTextContent("2 processes");
    expect(within(archived).getByRole("link", { name: "localhost:4321" })).toBeInTheDocument();
    const deleted = screen.getByRole("region", { name: "Deleted session" });
    expect(deleted).toHaveTextContent("sess-0b1e5c2a9f7d4e31");

    const [astro, storybook] = processesLeftRunningFixture[0]!.items;
    await user.click(
      within(archived).getByRole("button", { name: "Stop process: astro dev --port 4321" })
    );
    expect(onStop).toHaveBeenLastCalledWith([astro]);
    await user.click(
      within(archived).getByRole("button", {
        name: "Stop all processes from Landing page polish",
      })
    );
    expect(onStop).toHaveBeenLastCalledWith([storybook, astro]);
    await user.click(screen.getByTestId("left-running-stop-all"));
    expect(onStop).toHaveBeenLastCalledWith(
      processesLeftRunningFixture.flatMap(group => group.items)
    );
  });

  it("Should say so when nothing was left running", () => {
    render(
      <UIProvider reducedMotion="always">
        <ProcessesLeftRunningDialog open onOpenChange={vi.fn()} groups={[]} onStop={vi.fn()} />
      </UIProvider>
    );
    expect(screen.getByRole("dialog", { name: "Processes left running" })).toHaveTextContent(
      "Nothing was left running."
    );
    expect(screen.queryByTestId("left-running-stop-all")).not.toBeInTheDocument();
  });
});
