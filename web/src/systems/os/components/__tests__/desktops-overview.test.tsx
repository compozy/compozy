// Suite: desktops overview
// Invariant: overview entry focuses the nearest requested hidden desktop, each management action emits
// one semantic callback, and async states stay recoverable.
// Boundary IN: DesktopsOverview state rendering, local forms, accessibility, and callback payloads.
// Boundary OUT: TanStack Query snapshots, Zustand coordination, mutations, revisions, and persistence.
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewProps,
} from "../desktops-overview";

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Control",
    purpose: "standard",
    thumbnail: <span />,
    windows: [{ id: "dashboard", title: "Dashboard", detail: "Workspace status" }],
  },
  {
    id: "build",
    name: "Build",
    purpose: "focus",
    thumbnail: <span />,
    windows: [{ id: "agents", title: "Agents", detail: "Window manager" }],
  },
  {
    id: "research",
    name: "Research",
    purpose: "standard",
    thumbnail: <span />,
    windows: [],
  },
];

function renderOverview(overrides: Partial<DesktopsOverviewProps> = {}) {
  const callbacks = {
    onOpenChange: vi.fn(),
    onCreateDesktop: vi.fn(),
    onSwitchDesktop: vi.fn(),
    onRenameDesktop: vi.fn(),
    onReorderDesktop: vi.fn(),
    onDeleteDesktop: vi.fn(),
    onMoveWindow: vi.fn(),
    onRetry: vi.fn(),
    onResolveConflict: vi.fn(),
  };
  const props: DesktopsOverviewProps = {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    ...callbacks,
    ...overrides,
  };
  render(<DesktopsOverview {...props} />);
  return callbacks;
}

describe("DesktopsOverview", () => {
  it.each([
    {
      direction: "earlier" as const,
      hiddenDesktopIds: ["control", "build"],
      expectedName: "Switch to Build",
    },
    {
      direction: "later" as const,
      hiddenDesktopIds: ["research"],
      expectedName: "Switch to Research",
    },
  ])(
    "Should focus the nearest $direction hidden desktop when opened from pager overflow",
    async ({ direction, hiddenDesktopIds, expectedName }) => {
      renderOverview({ initialFocusSegment: { direction, hiddenDesktopIds } });

      await waitFor(() => expect(screen.getByRole("button", { name: expectedName })).toHaveFocus());
    }
  );

  it("Should emit switch, create, reorder, rename, move, and transfer-on-delete actions", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.click(screen.getByRole("button", { name: "Create desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Switch to Research" }));
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);

    await user.click(screen.getByRole("button", { name: "Move Research left" }));
    expect(callbacks.onReorderDesktop).toHaveBeenCalledWith("research", 1);

    await user.click(screen.getByRole("button", { name: "Rename Build" }));
    const name = screen.getByLabelText("Desktop name");
    await user.clear(name);
    await user.type(name, "Implementation");
    await user.click(screen.getByRole("button", { name: "Save name" }));
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("build", "Implementation");

    await user.selectOptions(
      screen.getByRole("combobox", { name: "Move Agents to another desktop" }),
      "research"
    );
    expect(callbacks.onMoveWindow).toHaveBeenCalledWith("agents", "build", "research");

    await user.click(screen.getByRole("button", { name: "Delete Build" }));
    await user.click(screen.getByRole("button", { name: "Delete desktop" }));
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("build", "control");
  });

  it("Should expose loading semantics without rendering plausible desktop data", () => {
    renderOverview({ state: { status: "loading" } });

    expect(screen.getByRole("dialog", { name: "Desktops" })).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByRole("button", { name: /Switch to/ })).not.toBeInTheDocument();
  });

  it("Should make every desktop mutation unavailable when no registered client is ready", () => {
    renderOverview({ canMutate: false });

    expect(screen.getByRole("button", { name: "Create desktop" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Switch to Research" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Move Research left" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Rename Build" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete Build" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Move Agents to another desktop" })).toBeDisabled();
  });

  it("Should retry an explicit load failure", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "error", message: "Desktop snapshot unavailable." },
      canMutate: false,
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Desktop snapshot unavailable.");
    await user.click(screen.getByRole("button", { name: "Retry loading" }));
    expect(callbacks.onRetry).toHaveBeenCalledTimes(1);
  });

  it("Should resolve a revision conflict before accepting another snapshot", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "conflict", message: "Desktop order changed remotely." },
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Desktop order changed remotely.");
    await user.click(screen.getByRole("button", { name: "Reload desktops" }));
    expect(callbacks.onResolveConflict).toHaveBeenCalledTimes(1);
  });

  it("Should offer creation when the authoritative desktop list is empty", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      state: { status: "ready", desktops: [], activeDesktopId: null },
    });

    expect(screen.getByText("No desktops")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Create desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);
  });
});
