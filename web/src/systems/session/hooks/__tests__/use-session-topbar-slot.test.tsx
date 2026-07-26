// Suite: session document topbar publisher
// Invariant: one session publisher exposes self-title, state mark, runtime meta, lifecycle controls, and inspector toggle before overflow.
// Boundary IN: useSessionTopbarSlot and the real Topbar slot consumer.
// Boundary OUT: daemon mutations and window manager behavior.

import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

import { primarySessionFixture } from "../../mocks/fixtures";
import { useSessionTopbarSlot } from "../use-session-topbar-slot";

function SessionPublisher({
  onStop,
  inspectorOpen = false,
  onInspectorToggle = vi.fn(),
}: {
  onStop: () => void;
  inspectorOpen?: boolean;
  onInspectorToggle?: () => void;
}) {
  useSessionTopbarSlot({
    session: primarySessionFixture,
    isDeleting: false,
    isStopping: false,
    isResuming: false,
    isClearing: false,
    canClear: true,
    inspectorOpen,
    onInspectorToggle,
    onDelete: vi.fn(),
    onStop,
    onResume: vi.fn(),
    onClear: vi.fn(),
  });
  return null;
}

describe("useSessionTopbarSlot", () => {
  it("Should publish the complete document head through one slot owner", () => {
    const onStop = vi.fn();
    renderWithTopbar(<SessionPublisher onStop={onStop} />);

    expect(
      screen.getByRole("heading", { level: 1, name: primarySessionFixture.name })
    ).toBeInTheDocument();
    expect(document.querySelector('[data-slot="topbar-glyph"]')).toHaveAttribute(
      "data-presentation",
      "state"
    );
    expect(screen.getByTestId("session-status-agent")).toHaveTextContent(
      primarySessionFixture.agent_name
    );
    expect(screen.getByText("Session badge: running")).toHaveClass("sr-only");
    expect(document.querySelector('[data-slot="topbar-crumbs"]')).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("session-inspector-toggle")).toHaveAttribute("aria-pressed", "false");
  });

  it("Should place the inspector toggle immediately before overflow and report pressed state", () => {
    const onInspectorToggle = vi.fn();
    renderWithTopbar(
      <SessionPublisher onStop={vi.fn()} inspectorOpen onInspectorToggle={onInspectorToggle} />
    );

    const toggle = screen.getByTestId("session-inspector-toggle");
    const overflow = screen.getByTestId("session-topbar-overflow");
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveAttribute("aria-label", "Close session inspector");
    expect(
      toggle.compareDocumentPosition(overflow) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();

    fireEvent.click(toggle);
    expect(onInspectorToggle).toHaveBeenCalledTimes(1);
  });

  it("Should keep an idle attachable session to one immediate action plus overflow", () => {
    const onStop = vi.fn();

    function IdlePublisher() {
      useSessionTopbarSlot({
        session: { ...primarySessionFixture, badge: "idle" },
        isDeleting: false,
        isStopping: false,
        isResuming: false,
        isClearing: false,
        canClear: true,
        inspectorOpen: false,
        onInspectorToggle: vi.fn(),
        onDelete: vi.fn(),
        onStop,
        onResume: vi.fn(),
        onClear: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<IdlePublisher />);

    expect(screen.getByRole("button", { name: "Attach session" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop session" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open session inspector" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });
});
