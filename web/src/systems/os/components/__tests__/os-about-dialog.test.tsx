// Suite: OS About dialog
// Invariant: the dialog names its rows while the daemon status is read and never claims a status it
// does not have: one busy region while pending, the identity rows once the status answers.
// Boundary IN: OsAboutDialog over the status hooks.
// Boundary OUT: the status transport (systems/status).
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const status = vi.hoisted(() => ({
  daemon: { isPending: true, isError: false, data: undefined as unknown },
}));

vi.mock("@/systems/status", () => ({
  useDaemonStatus: () => status.daemon,
  useStatus: () => ({ data: undefined }),
}));

import { OsAboutDialog } from "../os-about-dialog";

function renderDialog() {
  return render(<OsAboutDialog open onOpenChange={vi.fn()} />);
}

describe("OsAboutDialog", () => {
  beforeEach(() => {
    status.daemon = { isPending: true, isError: false, data: undefined };
  });

  it("Should hold the identity rows in one busy region while the daemon status is read", () => {
    renderDialog();

    expect(screen.getByRole("status")).toHaveTextContent("Reading CompozyOS status");
    expect(screen.getByText("Version")).toBeInTheDocument();
    expect(screen.queryByTestId("os-about-row-version")).not.toBeInTheDocument();
  });

  it("Should swap the placeholder for the identity rows once the status answers", () => {
    const { rerender } = renderDialog();
    status.daemon = {
      isPending: false,
      isError: false,
      data: {
        version: "0.4.0",
        status: "ready",
        started_at: "2026-10-24T12:00:00Z",
        pid: 4242,
        http_host: "127.0.0.1",
        http_port: 7777,
        socket: "/tmp/compozy.sock",
        user_home_dir: "/Users/tester",
      },
    };
    rerender(<OsAboutDialog open onOpenChange={vi.fn()} />);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByTestId("os-about-row-version")).toHaveTextContent("0.4.0");
  });
});
