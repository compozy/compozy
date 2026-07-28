// Suite: OS window runtime component
// Invariant: the live OsWindow mounts safely in StrictMode and remains mounted across minimized/compact presentation.
// Owning layer: the current OsWindow + react-rnd integration.
import { fireEvent, render, screen } from "@testing-library/react";
import { createRef, StrictMode, useState } from "react";
import type { Rnd } from "react-rnd";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsWindow, type OsWindowModel } from "../../hooks/use-os-window";
import type { OsDesktopRuntimeStore, OsWindow as OsWindowState } from "../../lib/os-types";
import { OsWindow } from "../os-window";

vi.mock("../../hooks/use-desktop", () => ({ useDesktop: vi.fn() }));
vi.mock("../../hooks/use-os-window", () => ({
  OS_WINDOW_DRAG_CANCEL_SELECTOR: "[data-slot='test-cancel']",
  OS_WINDOW_DRAG_HANDLE_CLASS: "test-drag-handle",
  useOsWindow: vi.fn(),
}));

function TasksController() {
  const [draft, setDraft] = useState("");
  return (
    <input aria-label="Task draft" onChange={event => setDraft(event.target.value)} value={draft} />
  );
}

vi.mock("../../lib/app-registry", () => ({
  getOsApp: () => ({ title: "Tasks", Controller: TasksController }),
  getOsAppMinimum: () => ({ width: 280, height: 180 }),
}));
vi.mock("../os-zoom-menu", () => ({
  OsZoomMenu: ({ children }: { children: React.ReactNode }) => children,
}));

function windowState(overrides: Partial<OsWindowState> = {}): OsWindowState {
  return {
    id: "window:tasks",
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    desktopId: "desktop:main",
    placement: "floating",
    rect: { x: 40, y: 50, w: 720, h: 480 },
    layer: 2,
    minimized: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...overrides,
  };
}

function windowModel(win: OsWindowState): OsWindowModel {
  return {
    win,
    focused: true,
    keepMounted: true,
    rect: win.rect,
    rndRef: createRef<Rnd>(),
    overlayHost: null,
    setOverlayHost: vi.fn(),
    handleTrafficLight: vi.fn(),
    handlePointerEnter: vi.fn(),
    handlePointerDownCapture: vi.fn(),
    handleFocusCapture: vi.fn(),
    handleDragStart: vi.fn(),
    handleDrag: vi.fn(),
    handleDragStop: vi.fn(),
    handleResizeStop: vi.fn(),
  };
}

let presentation: OsDesktopRuntimeStore["presentation"];
let model: OsWindowModel;

beforeEach(() => {
  presentation = "floating";
  model = windowModel(windowState());
  vi.mocked(useOsWindow).mockImplementation(() => model);
  vi.mocked(useDesktop).mockImplementation(selector =>
    selector({ presentation } as OsDesktopRuntimeStore)
  );
});

describe("OsWindow", () => {
  it("Should mount the react-rnd window under StrictMode", () => {
    render(
      <StrictMode>
        <OsWindow windowId="window:tasks" />
      </StrictMode>
    );

    expect(screen.getByRole("region", { name: "Tasks window" })).toBeInTheDocument();
    expect(screen.getByTestId("os-window-window:tasks")).toHaveAttribute(
      "data-window-placement",
      "floating"
    );
  });

  it("Should keep minimized windows mounted while hiding floating and compact presentations", () => {
    model = windowModel(windowState({ minimized: true }));
    const view = render(<OsWindow windowId="window:tasks" />);

    let window = screen.getByTestId("os-window-window:tasks");
    expect(window.parentElement).toHaveStyle({ display: "none" });

    presentation = "compact";
    view.rerender(<OsWindow windowId="window:tasks" />);

    window = screen.getByTestId("os-window-window:tasks");
    expect(window.parentElement).toHaveStyle({ display: "none" });
    expect(window).toHaveAttribute("data-minimized");
  });

  it("Should preserve controller state across floating and compact presentations", () => {
    model = { ...windowModel(windowState()), overlayHost: document.createElement("div") };
    const view = render(<OsWindow windowId="window:tasks" />);
    const draft = screen.getByRole("textbox", { name: "Task draft" });

    fireEvent.change(draft, { target: { value: "Keep this task context" } });

    presentation = "compact";
    view.rerender(<OsWindow windowId="window:tasks" />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");

    presentation = "floating";
    view.rerender(<OsWindow windowId="window:tasks" />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");
  });
});
