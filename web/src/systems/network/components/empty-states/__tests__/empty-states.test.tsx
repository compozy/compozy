// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DaemonDown, DirectEmpty, DirectsEmpty, NetworkEmpty, ThreadEmpty, ThreadsEmpty } from "..";

describe("Empty / disabled / error state copy (`_design.md` §7.2 + §7.3)", () => {
  it("NetworkEmpty matches the ready-state orientation copy and opens settings", async () => {
    const onOpenSettings = vi.fn();
    const user = userEvent.setup();
    render(<NetworkEmpty onOpenSettings={onOpenSettings} />);
    expect(screen.getByTestId("network-empty")).toBeInTheDocument();
    expect(screen.getByText("Network is ready when you are.")).toBeInTheDocument();
    expect(screen.getByText(/Executions stay Local by default/i)).toBeInTheDocument();
    expect(screen.getByText(/Choose Live explicitly/i)).toBeInTheDocument();
    expect(screen.getByText(/never enrolls an execution/i)).toBeInTheDocument();
    await user.click(screen.getByTestId("network-empty-open-settings"));
    expect(onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it("NetworkEmpty names who can change availability when disabled", () => {
    render(<NetworkEmpty disabledByAdmin />);
    expect(screen.getByText("Network is disabled.")).toBeInTheDocument();
    expect(screen.getByText(/operator with admin access/i)).toBeInTheDocument();
    expect(screen.getByText(/Local work remains available/i)).toBeInTheDocument();
  });

  it("ThreadsEmpty matches the no-threads copy verbatim", () => {
    render(<ThreadsEmpty />);
    expect(screen.getByText("No threads yet.")).toBeInTheDocument();
    expect(
      screen.getByText("Start the first one; agents and humans both join.")
    ).toBeInTheDocument();
  });

  it("ThreadsEmpty exposes the [Start a thread] action when handler is provided", async () => {
    const onStartThread = vi.fn();
    const user = userEvent.setup();
    render(<ThreadsEmpty onStartThread={onStartThread} />);
    await user.click(screen.getByTestId("network-threads-empty-start"));
    expect(onStartThread).toHaveBeenCalledTimes(1);
  });

  it("DirectsEmpty matches the no-directs copy verbatim", () => {
    render(<DirectsEmpty />);
    expect(screen.getByText("No direct rooms yet.")).toBeInTheDocument();
    expect(
      screen.getByText("Open one to talk privately with a peer in this channel.")
    ).toBeInTheDocument();
  });

  it("ThreadEmpty matches the empty-thread copy verbatim", () => {
    render(<ThreadEmpty />);
    expect(screen.getByText("Thread has no replies.")).toBeInTheDocument();
    expect(screen.getByText("Reply below to keep the context alive.")).toBeInTheDocument();
  });

  it("DirectEmpty matches the empty-direct copy verbatim", () => {
    render(<DirectEmpty />);
    expect(screen.getByText("Quiet so far.")).toBeInTheDocument();
    expect(screen.getByText("Send the first message; they'll be notified.")).toBeInTheDocument();
  });

  it("DaemonDown renders the unreachable error copy verbatim", () => {
    render(<DaemonDown />);
    expect(screen.getByText("Network is unreachable.")).toBeInTheDocument();
    expect(screen.getByText("Make sure the AGH daemon is running.")).toBeInTheDocument();
  });
});
