import { act, fireEvent, render, screen } from "@testing-library/react";
import { LayoutDashboard } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import {
  Topbar,
  TopbarSlotProvider,
  type TopbarSlotValue,
  useTopbarSlot,
  useTopbarSlotValue,
} from "../topbar";

function ProbeSlot({ slot, label }: { slot: TopbarSlotValue | null; label: string }) {
  useTopbarSlot(slot);
  return <span data-testid={`probe-${label}`} />;
}

function SlotInspector({ probeId }: { probeId: string }) {
  const slot = useTopbarSlotValue();
  return (
    <span data-testid={probeId}>
      actions:{slot?.actions ? "yes" : "no"} overflow:{slot?.overflow ? "yes" : "no"} crumb:
      {slot?.crumb ? "yes" : "no"} crumb-value:
      {typeof slot?.crumb === "string" ? slot.crumb : "no"} toolbar:
      {slot?.toolbar ? "yes" : "no"} nav:{slot?.nav ? "yes" : "no"} status:
      {slot?.status ? "yes" : "no"}
    </span>
  );
}

describe("Topbar", () => {
  it("Should render left-aligned identity with a quiet glyph and title", () => {
    render(
      <TopbarSlotProvider>
        <Topbar
          leading={<span data-testid="window-controls">lights</span>}
          glyph={<LayoutDashboard data-testid="glyph" />}
          title="Tasks"
        />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-leading']")).toContainElement(
      screen.getByTestId("window-controls")
    );
    expect(document.querySelector("[data-slot='topbar-glyph']")).toContainElement(
      screen.getByTestId("glyph")
    );
    const title = screen.getByRole("heading", { level: 1, name: "Tasks" });
    expect(title).toHaveAttribute("tabindex", "-1");
    expect(title).toHaveAttribute("data-slot", "topbar-title");
    expect(document.querySelector("[data-slot='topbar-breadcrumb']")).toBeNull();
  });

  it("Should render route identity without leading or slots", () => {
    const { container } = render(
      <TopbarSlotProvider>
        <Topbar title="Home" />
      </TopbarSlotProvider>
    );
    expect(container.querySelector("[data-slot='topbar']")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    expect(container.querySelector("[data-slot='topbar-leading']")).toBeNull();
    expect(container.querySelector("[data-slot='topbar-actions']")).toBeNull();
  });

  it("Should render the published crumb as the leaf title", () => {
    function Setup() {
      useTopbarSlot({ crumb: "Nightly audit" });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );

    expect(screen.getByRole("heading", { level: 1, name: "Nightly audit" })).toBeInTheDocument();
  });

  it("Should render count and status from the slot", () => {
    function Setup() {
      useTopbarSlot({
        count: 6,
        status: <span data-testid="status-chip">2 running</span>,
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-count']")).toHaveTextContent("6");
    expect(screen.getByTestId("status-chip")).toBeInTheDocument();
  });

  it("Should render peer route nav after identity and before the flex gutter", () => {
    function Setup() {
      useTopbarSlot({
        nav: <nav data-testid="route-tabs">List · Kanban</nav>,
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    const head = document.querySelector("[data-slot='topbar']");
    const identity = document.querySelector("[data-slot='topbar-identity']");
    const nav = document.querySelector("[data-slot='topbar-nav']");
    const flex = document.querySelector("[data-slot='topbar-flex']");
    expect(head).not.toBeNull();
    expect(identity).not.toBeNull();
    expect(nav).not.toBeNull();
    expect(flex).not.toBeNull();
    expect(nav).toContainElement(screen.getByTestId("route-tabs"));
    expect(identity!.compareDocumentPosition(nav!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(nav!.compareDocumentPosition(flex!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("Should omit topbar-nav when the publisher does not supply nav (drill-in)", () => {
    function Setup() {
      useTopbarSlot({
        onBack: () => undefined,
        crumbs: [{ id: "tasks", label: "Tasks", onSelect: () => undefined }],
        crumb: "Run #128",
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-nav']")).toBeNull();
  });

  it("Should render drill-in back + crumbs without a workspace prefix (UT-092)", () => {
    const onBack = vi.fn();
    const onSelect = vi.fn();
    function Setup() {
      useTopbarSlot({
        onBack,
        crumbs: [{ id: "tasks", label: "Tasks", onSelect }],
        crumb: "Run #128",
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-glyph']")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Back one level" }));
    expect(onBack).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Tasks" }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("heading", { level: 1, name: "Run #128" })).toBeInTheDocument();
    expect(screen.queryByText("agh")).toBeNull();
  });

  it("Should keep edge parents visible and expose collapsed middle parents as menu items", () => {
    const selectProjects = vi.fn();
    const selectWeb = vi.fn();
    const selectTasks = vi.fn();

    function Setup() {
      useTopbarSlot({
        crumbs: [
          { id: "knowledge", label: "Knowledge", onSelect: vi.fn() },
          { id: "projects", label: "Projects", onSelect: selectProjects },
          { id: "web", label: "Web", onSelect: selectWeb },
          { id: "tasks", label: "Tasks", onSelect: selectTasks },
        ],
        crumb: "streaming.md",
      });
      return null;
    }

    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Knowledge" />
      </TopbarSlotProvider>
    );

    expect(screen.queryByRole("button", { name: "Projects" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Tasks" }));
    expect(selectTasks).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Show hidden path levels" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Projects" }));

    expect(selectProjects).toHaveBeenCalledTimes(1);
    expect(selectWeb).not.toHaveBeenCalled();
  });

  it("Should let leading coexist with published actions/overflow slots", () => {
    function Setup() {
      useTopbarSlot({
        actions: <span data-testid="action-btn">action</span>,
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar leading={<span data-testid="window-controls">lights</span>} title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-leading']")).toContainElement(
      screen.getByTestId("window-controls")
    );
    expect(document.querySelector("[data-slot='topbar-actions']")).toContainElement(
      screen.getByTestId("action-btn")
    );
  });

  it("Should expose actions and overflow slots in their zones", () => {
    function Setup() {
      useTopbarSlot({
        actions: <span data-testid="action-btn">action</span>,
        overflow: <span data-testid="overflow-trigger">…</span>,
      });
      return null;
    }
    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(document.querySelector("[data-slot='topbar-actions']")).toContainElement(
      screen.getByTestId("action-btn")
    );
    expect(screen.getByTestId("topbar-overflow")).toContainElement(
      screen.getByTestId("overflow-trigger")
    );
  });

  it("Should publish a slot without rerendering its producer subtree", () => {
    let producerRenders = 0;
    function Setup() {
      producerRenders += 1;
      useTopbarSlot({ actions: <span data-testid="live-action" /> });
      return null;
    }

    render(
      <TopbarSlotProvider>
        <Setup />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );

    expect(screen.getByTestId("live-action")).toBeInTheDocument();
    expect(producerRenders).toBe(1);
  });

  it("Should re-push the slot when the consumer's slot reference changes", () => {
    function Setup({ label }: { label: string }) {
      useTopbarSlot({ actions: <span>{label}</span> });
      return null;
    }
    const { rerender } = render(
      <TopbarSlotProvider>
        <Setup label="first" />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );
    expect(screen.getByText("first")).toBeInTheDocument();
    act(() => {
      rerender(
        <TopbarSlotProvider>
          <Setup label="second" />
          <Topbar title="Tasks" />
        </TopbarSlotProvider>
      );
    });
    expect(screen.getByText("second")).toBeInTheDocument();
  });

  it("Should clear slot subfields when the slot consumer unmounts", () => {
    function Harness({ mounted }: { mounted: boolean }) {
      return (
        <>
          {mounted ? (
            <ProbeSlot
              slot={{
                crumb: "loop-a",
                actions: <span data-testid="a" />,
                overflow: <span data-testid="o" />,
                nav: <span data-testid="nv" />,
                toolbar: <span data-testid="tb" />,
              }}
              label="a"
            />
          ) : null}
          <SlotInspector probeId="inspector" />
        </>
      );
    }
    const { rerender } = render(
      <TopbarSlotProvider>
        <Harness mounted />
      </TopbarSlotProvider>
    );
    expect(screen.getByTestId("inspector")).toHaveTextContent("actions:yes");
    expect(screen.getByTestId("inspector")).toHaveTextContent("overflow:yes");
    expect(screen.getByTestId("inspector")).toHaveTextContent("crumb:yes");
    expect(screen.getByTestId("inspector")).toHaveTextContent("nav:yes");
    expect(screen.getByTestId("inspector")).toHaveTextContent("toolbar:yes");

    act(() => {
      rerender(
        <TopbarSlotProvider>
          <Harness mounted={false} />
        </TopbarSlotProvider>
      );
    });

    expect(screen.getByTestId("inspector")).toHaveTextContent("actions:no");
    expect(screen.getByTestId("inspector")).toHaveTextContent("overflow:no");
    expect(screen.getByTestId("inspector")).toHaveTextContent("crumb:no");
    expect(screen.getByTestId("inspector")).toHaveTextContent("nav:no");
    expect(screen.getByTestId("inspector")).toHaveTextContent("toolbar:no");
  });

  it("Should preserve the active slot when an older consumer unmounts", () => {
    function Harness({ showOlder }: { showOlder: boolean }) {
      return (
        <>
          {showOlder ? <ProbeSlot slot={{ crumb: "Older" }} label="older" /> : null}
          <ProbeSlot slot={{ crumb: "Active" }} label="active" />
          <SlotInspector probeId="inspector" />
        </>
      );
    }

    const { rerender } = render(
      <TopbarSlotProvider>
        <Harness showOlder />
      </TopbarSlotProvider>
    );
    expect(screen.getByTestId("inspector")).toHaveTextContent("crumb-value:Active");

    act(() => {
      rerender(
        <TopbarSlotProvider>
          <Harness showOlder={false} />
        </TopbarSlotProvider>
      );
    });

    expect(screen.getByTestId("inspector")).toHaveTextContent("crumb-value:Active");
  });

  it("Should not let a non-owning null consumer erase the active slot", () => {
    render(
      <TopbarSlotProvider>
        <ProbeSlot slot={{ crumb: "Active" }} label="active" />
        <ProbeSlot slot={null} label="inactive" />
        <SlotInspector probeId="inspector" />
      </TopbarSlotProvider>
    );

    expect(screen.getByTestId("inspector")).toHaveTextContent("crumb:yes");
  });

  it("Should publish replaced action handlers when only the callback identity changes", () => {
    const firstAction = vi.fn();
    const secondAction = vi.fn();

    function Harness({ onRun }: { onRun: () => void }) {
      useTopbarSlot({
        actions: (
          <button onClick={onRun} type="button">
            Run
          </button>
        ),
      });
      return null;
    }

    const { rerender } = render(
      <TopbarSlotProvider>
        <Harness onRun={firstAction} />
        <Topbar title="Tasks" />
      </TopbarSlotProvider>
    );

    fireEvent.click(screen.getByRole("button", { name: "Run" }));
    expect(firstAction).toHaveBeenCalledTimes(1);
    expect(secondAction).not.toHaveBeenCalled();

    act(() => {
      rerender(
        <TopbarSlotProvider>
          <Harness onRun={secondAction} />
          <Topbar title="Tasks" />
        </TopbarSlotProvider>
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "Run" }));
    expect(firstAction).toHaveBeenCalledTimes(1);
    expect(secondAction).toHaveBeenCalledTimes(1);
  });

  it("Should be a no-op when used outside a TopbarSlotProvider (test ergonomics)", () => {
    function Harness() {
      useTopbarSlot({ actions: <span data-testid="a" /> });
      return <span data-testid="probe">ok</span>;
    }
    expect(() => render(<Harness />)).not.toThrow();
    expect(screen.getByTestId("probe")).toHaveTextContent("ok");
  });
});
