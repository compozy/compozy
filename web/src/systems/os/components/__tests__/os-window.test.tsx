// Suite: OS window runtime component
// Invariant: the live OsWindow frame mounts safely in StrictMode, keeps every member mounted
// across minimize/compact/tab switches, hosts the deck only at ≥2 members, and maps semantic
// frame order onto visual layers where floating windows remain above tiled resize seams.
import { fireEvent, render, screen, within } from "@testing-library/react";
import { StrictMode, useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsWindow, type OsWindowModel } from "../../hooks/use-os-window";
import type { OsWindowFrameModel } from "../../lib/group-projection";
import type { OsDesktopRuntimeStore, OsWindow as OsWindowState } from "../../lib/os-types";
import type { LayoutProjection, ProjectedSeam } from "../../lib/window-manager-types";
import { WINDOW_VISUAL_LAYER, windowVisualLayer } from "../../lib/window-visual-layer";
import { OsSnapSeamLayer } from "../os-snap-seam";
import { OsWindow } from "../os-window";

vi.mock("../../hooks/use-desktop", () => ({ useDesktop: vi.fn() }));
vi.mock("../../hooks/use-os-shell", () => {
  const manager = {};
  return {
    useOsShell: () => ({ coordinator: { noteNavigateMode: vi.fn() }, manager }),
  };
});
vi.mock("../../hooks/use-os-window", () => ({
  OS_WINDOW_DRAG_CANCEL_SELECTOR: "[data-slot='test-cancel']",
  OS_WINDOW_DRAG_HANDLE_CLASS: "test-drag-handle",
  useOsWindow: vi.fn(),
}));
vi.mock("../../hooks/use-os-window-deck", () => ({
  useOsWindowDeck: vi.fn(() => ({
    sessionById: new Map(),
    tabDrag: null,
    dropIndex: null,
    otherWindowCount: 0,
    registerTabs: vi.fn(),
    registerTab: vi.fn(),
    handleTabPointerDown: vi.fn(),
    wasDragged: () => false,
    activateTab: vi.fn(),
    closeTab: vi.fn(),
    closeOthers: vi.fn(),
    closeRight: vi.fn(),
    pinTab: vi.fn(),
    moveToNewWindow: vi.fn(),
    mergeAllWindows: vi.fn(),
    openNewTab: () => deckOpenNewTab(),
  })),
}));

function TasksController() {
  const [draft, setDraft] = useState("");
  return (
    <input aria-label="Task draft" onChange={event => setDraft(event.target.value)} value={draft} />
  );
}

const pendingController = new Promise<never>(() => undefined);

function PendingController(): never {
  throw pendingController;
}

let pendingJobsController = false;
let deckOpenNewTab = vi.fn();

vi.mock("../../lib/app-registry", () => ({
  OS_APPS: {},
  getOsApp: (app: string) => ({
    title: app === "jobs" ? "Jobs" : "Tasks",
    icon: () => null,
    Controller: pendingJobsController && app === "jobs" ? PendingController : TasksController,
  }),
  getOsAppMinimum: () => ({ width: 280, height: 180 }),
  resolveAppForPath: () => null,
  matchSessionInstance: () => null,
  dockApps: () => [],
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
    navStack: [],
    pinned: false,
    desktopId: "desktop:main",
    placement: "floating",
    rect: { x: 40, y: 50, w: 720, h: 480 },
    layer: 2,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...overrides,
  };
}

function frameModel(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
  return {
    id: "window:tasks",
    desktopId: "desktop:main",
    kind: "floating",
    rect: { x: 40, y: 50, w: 720, h: 480 },
    members: ["window:tasks"],
    activeWindowId: "window:tasks",
    stackId: null,
    minimized: false,
    zoomed: false,
    adapted: false,
    layer: 2,
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
    ...overrides,
  };
}

function hookModel(frame: OsWindowFrameModel): OsWindowModel {
  return {
    focused: true,
    keepMounted: true,
    rect: frame.rect,
    registerRnd: vi.fn(),
    handleTrafficLight: vi.fn(),
    handlePointerEnter: vi.fn(),
    handlePointerDownCapture: vi.fn(),
    handleFocusCapture: vi.fn(),
    handleDragStart: vi.fn(),
    handleDrag: vi.fn(),
    handleDragStop: vi.fn(),
    handleResizeStart: vi.fn(),
    handleResizeStop: vi.fn(),
    resizeMax: null,
  };
}

let presentation: OsDesktopRuntimeStore["presentation"];
let windows: Record<string, OsWindowState>;
let model: OsWindowModel;

beforeEach(() => {
  presentation = "floating";
  windows = { "window:tasks": windowState() };
  model = hookModel(frameModel());
  pendingJobsController = false;
  deckOpenNewTab = vi.fn();
  vi.mocked(useOsWindow).mockImplementation(() => model);
  vi.mocked(useDesktop).mockImplementation(selector =>
    // Window chrome renders the daemon's effective chords, so the fixture has
    // to carry a keymap for the deck to label anything with one.
    selector({
      presentation,
      windows,
      windowManagerConfig: {
        effectiveShortcuts: { "window.tab.new": ["meta+KeyT"], "window.close": ["meta+KeyW"] },
      },
    } as unknown as OsDesktopRuntimeStore)
  );
});

describe("OsWindow", () => {
  it("Should mount the react-rnd frame under StrictMode", () => {
    render(
      <StrictMode>
        <OsWindow frame={frameModel()} />
      </StrictMode>
    );

    expect(screen.getByRole("region", { name: "Tasks window" })).toBeInTheDocument();
    expect(screen.getByTestId("os-window-window:tasks")).toHaveAttribute(
      "data-window-placement",
      "floating"
    );
  });

  it("Should render floating frames above tiled windows and their resize seams", () => {
    const view = render(<OsWindow frame={frameModel({ layer: 2 })} />);

    expect(screen.getByTestId("os-window-frame-window:tasks").parentElement).toHaveStyle({
      zIndex: 5,
    });

    const tiledFrame = frameModel({ kind: "tiled", layer: 1 });
    view.rerender(<OsWindow frame={tiledFrame} />);
    expect(screen.getByTestId("os-window-frame-window:tasks").parentElement).toHaveStyle({
      zIndex: 1,
    });

    expect(windowVisualLayer(frameModel({ kind: "floating", layer: 1 }))).toBe(
      WINDOW_VISUAL_LAYER.zoomed + 1
    );
    expect(windowVisualLayer(frameModel({ kind: "tiled", layer: 1, zoomed: true }))).toBe(
      WINDOW_VISUAL_LAYER.zoomed
    );
  });

  it("Should pin a zoomed frame and report the zoom control as pressed", () => {
    render(<OsWindow frame={frameModel({ kind: "tiled", layer: 1, zoomed: true })} />);

    const chrome = screen.getByTestId("os-window-frame-window:tasks");
    expect(chrome).toHaveAttribute("data-zoomed", "");
    expect(chrome.parentElement).toHaveStyle({ zIndex: WINDOW_VISUAL_LAYER.zoomed });
    expect(screen.getByRole("button", { name: "Zoom window" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
  });

  it("Should flush tiled chrome without a cast window shadow", () => {
    const classesOf = (node: HTMLElement) => node.className.split(/\s+/);
    const view = render(<OsWindow frame={frameModel({ kind: "floating" })} />);
    const floatingChrome = screen.getByTestId("os-window-frame-window:tasks");
    expect(floatingChrome).toHaveAttribute("data-kind", "floating");
    expect(classesOf(floatingChrome)).toContain("shadow-window");

    view.rerender(<OsWindow frame={frameModel({ kind: "tiled", layer: 1 })} />);
    const tiledChrome = screen.getByTestId("os-window-frame-window:tasks");
    expect(tiledChrome).toHaveAttribute("data-kind", "tiled");
    expect(classesOf(tiledChrome)).not.toContain("shadow-window");
    expect(classesOf(tiledChrome)).not.toContain("shadow-window-unfocused");
  });

  it("Should render resize seams on the semantic seam layer", () => {
    const seam: ProjectedSeam = {
      id: "split:main:0",
      splitId: "split:main",
      boundaryIndex: 0,
      orientation: "vertical",
      rect: { x: 320, y: 0, w: 0, h: 480 },
      value: 320,
      minValue: 200,
      maxValue: 520,
      axisSpan: 720,
      leadingWeight: 0.5,
      trailingWeight: 0.5,
      leadingWindowIds: ["window:tasks"],
      trailingWindowIds: ["window:jobs"],
    };
    const projection: LayoutProjection = {
      revision: 1,
      desktopId: "desktop:main",
      workArea: { x: 0, y: 0, w: 720, h: 480 },
      windows: [],
      stacks: [],
      seams: [seam],
      frameSeams: [],
      diagnostics: [],
    };

    render(
      <OsSnapSeamLayer
        projection={projection}
        onResize={vi.fn()}
        onFrameResize={vi.fn()}
        onSeamPreview={vi.fn()}
        onFrameSeamPreview={vi.fn()}
        onSeamPreviewEnd={vi.fn()}
      />
    );

    expect(screen.getByRole("separator", { name: "Resize boundary 1" })).toHaveStyle({
      zIndex: WINDOW_VISUAL_LAYER.seam,
    });
  });

  it("Should keep minimized frames mounted while hiding floating and compact presentations", () => {
    windows = { "window:tasks": windowState({ minimized: true }) };
    const minimizedFrame = frameModel({ minimized: true });
    const view = render(<OsWindow frame={minimizedFrame} />);

    let frameHost = screen.getByTestId("os-window-frame-window:tasks");
    expect(frameHost.parentElement).toHaveStyle({ display: "none" });

    presentation = "compact";
    view.rerender(<OsWindow frame={minimizedFrame} />);

    frameHost = screen.getByTestId("os-window-frame-window:tasks");
    expect(frameHost.parentElement).toHaveStyle({ display: "none" });
    expect(screen.getByTestId("os-window-window:tasks")).toHaveAttribute("data-minimized");
  });

  it("Should preserve controller state across floating and compact presentations", () => {
    const frame = frameModel();
    const view = render(<OsWindow frame={frame} />);
    const draft = screen.getByRole("textbox", { name: "Task draft" });

    fireEvent.change(draft, { target: { value: "Keep this task context" } });

    presentation = "compact";
    view.rerender(<OsWindow frame={frame} />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");

    presentation = "floating";
    view.rerender(<OsWindow frame={frame} />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");
  });

  it("Should mount and unmount the deck exactly as a frame crosses the second-member boundary (UT-070, UT-071)", () => {
    windows = {
      "window:tasks": windowState(),
      "window:jobs": windowState({ id: "window:jobs", app: "jobs", layer: 3 }),
    };
    const group = frameModel({
      id: "stack:main",
      members: ["window:tasks", "window:jobs"],
      activeWindowId: "window:tasks",
      stackId: "stack:main",
    });
    const view = render(<OsWindow frame={group} />);

    expect(screen.getByTestId("os-window-deck-stack:main")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Close window" })).toHaveLength(1);

    const solo = frameModel();
    view.rerender(<OsWindow frame={solo} />);
    expect(screen.queryByTestId("os-window-deck-stack:main")).toBeNull();
    expect(screen.getAllByRole("button", { name: "Close window" })).toHaveLength(1);
  });

  it("Should keep both member bodies mounted while only the active surface is exposed (UT-073)", () => {
    windows = {
      "window:tasks": windowState(),
      "window:jobs": windowState({ id: "window:jobs", app: "jobs", layer: 3 }),
    };
    const group = frameModel({
      id: "stack:main",
      members: ["window:tasks", "window:jobs"],
      activeWindowId: "window:tasks",
      stackId: "stack:main",
    });
    const view = render(<OsWindow frame={group} />);

    const tasksBody = screen.getByTestId("os-window-window:tasks");
    const jobsBody = screen.getByTestId("os-window-window:jobs");
    expect(tasksBody).not.toHaveAttribute("hidden");
    expect(jobsBody).toHaveAttribute("hidden");

    view.rerender(<OsWindow frame={group} />);
    expect(screen.getByTestId("os-window-window:tasks")).toBe(tasksBody);
    expect(screen.getByTestId("os-window-window:jobs")).toBe(jobsBody);
  });

  it("Should keep a sole pinned member in the normal one-window composition (UT-072)", () => {
    windows = { "window:tasks": windowState({ pinned: true }) };
    render(<OsWindow frame={frameModel()} />);

    expect(screen.queryByTestId("os-window-deck-window:tasks")).toBeNull();
    expect(screen.getByRole("button", { name: "Close window" })).toBeInTheDocument();
  });

  it("Should expose a loading body immediately for a suspense-pending active tab without freezing the deck (UT-074)", () => {
    pendingJobsController = true;
    windows = {
      "window:tasks": windowState(),
      "window:jobs": windowState({ id: "window:jobs", app: "jobs", layer: 3 }),
    };
    const group = frameModel({
      id: "stack:main",
      members: ["window:tasks", "window:jobs"],
      activeWindowId: "window:jobs",
      stackId: "stack:main",
    });
    render(<OsWindow frame={group} />);

    expect(
      within(screen.getByTestId("os-window-window:jobs")).getByRole("status", { name: "Loading" })
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "New tab (⌘T)" }));
    expect(deckOpenNewTab).toHaveBeenCalledOnce();
  });
});
