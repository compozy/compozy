// Suite: OS window frame chrome
// Invariant: each inactive-window chrome activation dispatches once without stealing the
// pointer sequence, while body pointers and keyboard focus still activate the window.
// Boundary IN: OsWindowFrame + real traffic-light DOM events; OUT: daemon command transport.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { useTopbarSlot } from "@agh/ui";

import { OsWindowFrame } from "../os-window-frame";
import type { OsTrafficLightAction } from "../os-traffic-lights";

function CatalogPublisher({ title, toolbar }: { title: string; toolbar?: ReactNode }) {
  useTopbarSlot({ crumb: title, toolbar });
  return (
    <div data-testid="catalog-body">
      <p>Catalog body — identity lives in the head.</p>
    </div>
  );
}

function ScrollBody() {
  return (
    <div style={{ height: 800 }} data-testid="tall-content">
      Tall content for scroll elevation
    </div>
  );
}

const CHROME_ACTIONS: ReadonlyArray<{
  action: OsTrafficLightAction;
  label: string;
}> = [
  { action: "close", label: "Close window" },
  { action: "minimize", label: "Minimize window" },
  { action: "zoom", label: "Zoom window" },
];

function BackgroundFrameHarness({
  onTrafficLight,
  onWindowActivation,
}: {
  onTrafficLight: (action: OsTrafficLightAction) => void;
  onWindowActivation: () => void;
}) {
  const [activationEpoch, setActivationEpoch] = useState(0);
  const activateWindow = () => {
    onWindowActivation();
    setActivationEpoch(current => current + 1);
  };

  return (
    <OsWindowFrame
      key={activationEpoch}
      title="Tasks"
      focused={activationEpoch > 0}
      onTrafficLight={onTrafficLight}
      onPointerDownCapture={activateWindow}
      onFocusCapture={activateWindow}
    >
      <p>Tasks content</p>
    </OsWindowFrame>
  );
}

describe("OsWindowFrame", () => {
  it.each(CHROME_ACTIONS)(
    "Should dispatch background $action exactly once through a focus rerender",
    async ({ action, label }) => {
      const user = userEvent.setup();
      const onTrafficLight = vi.fn();
      const onWindowActivation = vi.fn();
      render(
        <BackgroundFrameHarness
          onTrafficLight={onTrafficLight}
          onWindowActivation={onWindowActivation}
        />
      );

      await user.click(screen.getByRole("button", { name: label }));

      expect(onTrafficLight).toHaveBeenCalledOnce();
      expect(onTrafficLight).toHaveBeenCalledWith(action);
      expect(onWindowActivation).not.toHaveBeenCalled();
    }
  );

  it("Should preserve body pointer activation and native keyboard chrome activation", async () => {
    const user = userEvent.setup();
    const onPointerDownCapture = vi.fn();
    const onFocusCapture = vi.fn();
    const onTrafficLight = vi.fn();
    render(
      <OsWindowFrame
        title="Tasks"
        onTrafficLight={onTrafficLight}
        onPointerDownCapture={onPointerDownCapture}
        onFocusCapture={onFocusCapture}
      >
        <button type="button">Body action</button>
      </OsWindowFrame>
    );

    fireEvent.pointerDown(screen.getByRole("button", { name: "Body action" }));
    expect(onPointerDownCapture).toHaveBeenCalledOnce();

    await user.tab();
    expect(screen.getByRole("button", { name: "Close window" })).toHaveFocus();
    expect(onFocusCapture).toHaveBeenCalledOnce();

    await user.keyboard("{Enter}");
    expect(onTrafficLight).toHaveBeenCalledOnce();
    expect(onTrafficLight).toHaveBeenCalledWith("close");
  });

  it("Should render identity once in the head with no body page-head (UT-090)", () => {
    render(
      <OsWindowFrame title="Fallback" data-testid="frame">
        <CatalogPublisher title="Tasks" />
      </OsWindowFrame>
    );

    expect(screen.getByRole("heading", { level: 1, name: "Tasks" })).toBeInTheDocument();
    expect(screen.getByTestId("catalog-body").querySelector('[data-slot="page-head"]')).toBeNull();
    expect(document.querySelectorAll('[data-slot="page-head"]')).toHaveLength(0);
  });

  it("Should show and clear the optional 38px toolbar strip (UT-091)", () => {
    function Harness({ withToolbar }: { withToolbar: boolean }) {
      return (
        <OsWindowFrame title="Tasks">
          <CatalogPublisher
            title="Tasks"
            toolbar={withToolbar ? <span data-testid="strip-tools">filters</span> : undefined}
          />
        </OsWindowFrame>
      );
    }

    const { rerender } = render(<Harness withToolbar />);
    expect(document.querySelector('[data-slot="os-window-toolbar"]')).toContainElement(
      screen.getByTestId("strip-tools")
    );

    rerender(<Harness withToolbar={false} />);
    expect(document.querySelector('[data-slot="os-window-toolbar"]')).toBeNull();
  });

  it("Should elevate the head when the body scrolls past 2px (UT-094)", () => {
    render(
      <OsWindowFrame title="Tasks" style={{ height: 200 }}>
        <ScrollBody />
      </OsWindowFrame>
    );

    const head = document.querySelector('[data-slot="os-window-head"]');
    const body = document.querySelector('[data-slot="os-window-body"]');
    expect(head).not.toBeNull();
    expect(body).not.toBeNull();
    expect(head).not.toHaveAttribute("data-scrolled");

    Object.defineProperty(body, "scrollTop", { configurable: true, value: 8, writable: true });
    fireEvent.scroll(body as Element);
    expect(head).toHaveAttribute("data-scrolled", "");

    Object.defineProperty(body, "scrollTop", { configurable: true, value: 0, writable: true });
    fireEvent.scroll(body as Element);
    expect(head).not.toHaveAttribute("data-scrolled");
  });

  it("Should elevate the head when a nested content scroller moves", () => {
    render(
      <OsWindowFrame title="Tasks" style={{ height: 200 }}>
        <div data-testid="nested-scroller" className="overflow-auto">
          <ScrollBody />
        </div>
      </OsWindowFrame>
    );

    const head = document.querySelector('[data-slot="os-window-head"]');
    const nestedScroller = screen.getByTestId("nested-scroller");
    Object.defineProperty(nestedScroller, "scrollTop", {
      configurable: true,
      value: 8,
      writable: true,
    });
    fireEvent.scroll(nestedScroller);

    expect(head).toHaveAttribute("data-scrolled", "");
  });

  it("Should publish document self-title with a state mark and no crumb trail (UT-093)", () => {
    function DocumentPublisher() {
      useTopbarSlot({
        glyph: <span data-testid="state-dot" className="size-[7px] rounded-full bg-accent" />,
        glyphPresentation: "state",
        crumb: "nightly-audit",
      });
      return <div data-testid="document-body">session thread</div>;
    }

    render(
      <OsWindowFrame title="Session">
        <DocumentPublisher />
      </OsWindowFrame>
    );

    expect(screen.getByRole("heading", { level: 1, name: "nightly-audit" })).toBeInTheDocument();
    expect(screen.getByTestId("state-dot").parentElement).toHaveAttribute(
      "data-presentation",
      "state"
    );
    expect(document.querySelector('[data-slot="topbar-crumbs"]')).toBeNull();
    expect(screen.queryByRole("button", { name: "Back one level" })).toBeNull();
  });
});
