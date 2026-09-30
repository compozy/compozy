// Suite: desktops overview
// Invariant: overview entry focuses the nearest requested hidden desktop (else the current one), each
// management action emits one semantic callback, the card grid is keyboard-navigable, and async states
// stay recoverable.
// Boundary IN: DesktopsOverview state rendering, local forms, accessibility, keyboard, and callback payloads.
// Boundary OUT: TanStack Query snapshots, window-manager coordination, mutations, revisions, and persistence.
import { render, screen, waitFor, within } from "@testing-library/react";
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
    thumbnail: <span />,
    windows: [{ id: "dashboard", title: "Dashboard", detail: "Workspace status" }],
  },
  {
    id: "build",
    name: "Build",
    thumbnail: <span />,
    windows: [{ id: "agents", title: "Agents", detail: "Window manager" }],
    needsYou: true,
    switchShortcut: "⌃2",
  },
  {
    id: "research",
    name: "Research",
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
      expectedName: "Switch to Build — needs you",
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

  it("Should focus the current desktop when opened without a pager request", async () => {
    renderOverview();

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Current desktop Control" })).toHaveFocus()
    );
  });

  it("Should emit switch, create, reorder, rename, move, and transfer-on-delete actions", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    const [headerCreate, cardCreate] = screen.getAllByRole("button", { name: "New desktop" });
    await user.click(headerCreate);
    await user.click(cardCreate);
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(2);

    await user.click(screen.getByRole("button", { name: "Switch to Research" }));
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);

    await user.click(screen.getByRole("button", { name: "Actions for Research" }));
    await user.click(await screen.findByRole("menuitem", { name: "Move left" }));
    expect(callbacks.onReorderDesktop).toHaveBeenCalledWith("research", 1);

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    const name = screen.getByLabelText("Desktop name");
    await user.clear(name);
    await user.type(name, "Implementation");
    await user.click(screen.getByRole("button", { name: "Save name" }));
    expect(callbacks.onRenameDesktop).toHaveBeenCalledWith("build", "Implementation");

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    // Keyboard through the nested submenus: window, then destination.
    (await screen.findByRole("menuitem", { name: "Move a window" })).focus();
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Agents" })).toHaveFocus());
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Control" })).toHaveFocus());
    await user.keyboard("{ArrowDown}{Enter}");
    expect(callbacks.onMoveWindow).toHaveBeenCalledWith("agents", "build", "research");

    await user.click(screen.getByRole("button", { name: "Actions for Build" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete…" }));
    await user.click(screen.getByRole("button", { name: "Delete desktop" }));
    expect(callbacks.onDeleteDesktop).toHaveBeenCalledWith("build", "control");
  });

  it("Should mark an off-screen desktop that needs you and show its switch chord", () => {
    renderOverview();

    const build = screen.getByRole("button", { name: "Switch to Build — needs you" });
    expect(within(build).getByText("⌃2")).toBeInTheDocument();
    expect(within(build).getByLabelText("Needs you")).toBeInTheDocument();
  });

  it("Should move between cards with the arrow keys and close with Done", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();
    const current = screen.getByRole("button", { name: "Current desktop Control" });
    await waitFor(() => expect(current).toHaveFocus());

    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: "Switch to Build — needs you" })).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getAllByRole("button", { name: "New desktop" }).at(-1)).toHaveFocus();
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("button", { name: "Switch to Research" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(callbacks.onSwitchDesktop).toHaveBeenCalledWith("research");

    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(callbacks.onOpenChange).toHaveBeenLastCalledWith(false);
  });

  it("Should expose loading semantics without rendering plausible desktop data", () => {
    renderOverview({ state: { status: "loading" } });

    expect(screen.getByRole("dialog", { name: "Desktops" })).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByRole("button", { name: /Switch to/ })).not.toBeInTheDocument();
  });

  it("Should make every desktop mutation unavailable when no registered client is ready", () => {
    renderOverview({ canMutate: false });

    for (const create of screen.getAllByRole("button", { name: "New desktop" })) {
      expect(create).toBeDisabled();
    }
    expect(screen.getByRole("button", { name: "Switch to Research" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Actions for Research" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Actions for Build" })).toBeDisabled();
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
    await user.click(screen.getByRole("button", { name: "New desktop" }));
    expect(callbacks.onCreateDesktop).toHaveBeenCalledTimes(1);
  });
});
