// Suite: OS window runtime component
// Invariant: the live OsWindow frame mounts safely in StrictMode, keeps every member mounted
// across minimize/compact/tab switches, hosts the deck only at ≥2 members, and maps semantic
// frame order onto visual layers where floating windows remain above tiled resize seams. On the
// All projects desktop a bound item window names its project (or User) in the chrome; catalog
// windows and project desktops show no badge. While an app's surface loads, the window body shows
// the app's registered loading frame for the window's route, in that window's own scope, or the
// generic rows for an app without one.
import { fireEvent, render, screen, within } from "@testing-library/react";
import { StrictMode, useContext, useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsWindow, type OsWindowModel } from "../../hooks/use-os-window";
import type { OsWindowFrameModel } from "../../lib/group-projection";
import type {
  OsDesktopRuntimeStore,
  OsWindow as OsWindowState,
  OsWindowRoute,
} from "../../lib/os-types";
import type { LayoutProjection, ProjectedSeam } from "../../lib/window-manager-types";
import { WINDOW_VISUAL_LAYER, windowVisualLayer } from "../../lib/window-visual-layer";
import { OsSnapSeamLayer } from "../os-snap-seam";
import { OsWindow } from "../os-window";
import { WindowScopeContext } from "@/hooks/use-window-scope";

vi.mock("../../hooks/use-desktop", () => ({ useDesktop: vi.fn() }));
vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useActiveWorkspace: () => ({
    registeredWorkspaces: [{ id: "ws_beta", name: "beta" }],
  }),
}));
const navigation = vi.hoisted(() => ({ noteNavigateMode: vi.fn() }));

vi.mock("../../hooks/use-os-shell", () => {
  const manager = {};
  return {
    useOsShell: () => ({ coordinator: navigation, manager }),
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

const controllerRenders: string[] = [];

function TasksController({ windowId }: { windowId: string }) {
  controllerRenders.push(windowId);
  const [draft, setDraft] = useState("");
  return (
    <input aria-label="Task draft" onChange={event => setDraft(event.target.value)} value={draft} />
  );
}

/**
 * Two body links: a plain one (a drill) and one inside content that owns its own in-window
 * transitions (Settings sections). Both are pushes; ownership only changes the animation.
 */
function LinksController() {
  return (
    <div>
      <a href="/tasks/task-1">Open task</a>
      <div data-owns-view-transition="">
        <a href="/settings/terminal">Terminal section</a>
      </div>
    </div>
  );
}

let linksController = false;

const pendingController = new Promise<never>(() => undefined);

function PendingController(): never {
  throw pendingController;
}

let pendingJobsController = false;
let jobsLoadingFallback = false;
let deckOpenNewTab = vi.fn();

function JobsLoadingFallback({ route }: { route: OsWindowRoute }) {
  return (
    <div
      aria-label="Loading Automations"
      data-route={route.pathname}
      data-scope={useContext(WindowScopeContext) ?? ""}
      role="status"
    />
  );
}

vi.mock("../../lib/app-registry", () => ({
  OS_APPS: {},
  getOsApp: (app: string) => ({
    title: app === "automations" ? "Automations" : "Tasks",
    icon: () => null,
    Controller: linksController
      ? LinksController
      : pendingJobsController && app === "automations"
        ? PendingController
        : TasksController,
    LoadingFallback: jobsLoadingFallback && app === "automations" ? JobsLoadingFallback : undefined,
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
let partition: string;
let windows: Record<string, OsWindowState>;
let model: OsWindowModel;

beforeEach(() => {
  controllerRenders.length = 0;
  presentation = "floating";
  partition = "ws_alpha";
  windows = { "window:tasks": windowState() };
  model = hookModel(frameModel());
  pendingJobsController = false;
  linksController = false;
  navigation.noteNavigateMode.mockReset();
  jobsLoadingFallback = false;
  deckOpenNewTab = vi.fn();
  vi.mocked(useOsWindow).mockImplementation(() => model);
  vi.mocked(useDesktop).mockImplementation(selector =>
    // Window chrome renders the daemon's effective chords, so the fixture has
    // to carry a keymap for the deck to label anything with one.
    selector({
      presentation,
      snapshot: { workspaceId: partition },
      windows,
      windowManagerConfig: {
        effectiveShortcuts: { "window.tab.new": ["meta+KeyT"], "window.close": ["meta+KeyW"] },
      },
    } as unknown as OsDesktopRuntimeStore)
  );
});

describe("OsWindow geometry render isolation", () => {
  it("Should retain an unchanged sibling while updating the moved frame", () => {
    // JSDOM has no layout: start at its zero origin so react-rnd does not
    // infer a parent offset from a nonzero initial position.
    const primary = frameModel({ rect: { x: 0, y: 0, w: 720, h: 480 } });
    const sibling = frameModel({
      id: "window:other",
      members: ["window:other"],
      activeWindowId: "window:other",
    });
    windows[sibling.id] = windowState({ id: sibling.id });
    vi.mocked(useOsWindow).mockImplementation(frame => ({ ...model, rect: frame.rect }));
    const view = render(
      <>
        <OsWindow frame={primary} />
        <OsWindow frame={sibling} />
      </>
    );
    expect(screen.getAllByRole("textbox", { name: "Task draft" })).toHaveLength(2);
    controllerRenders.length = 0;
    const moved = { ...primary, rect: { ...primary.rect, x: 200 } };
    view.rerender(
      <>
        <OsWindow frame={moved} />
        <OsWindow frame={sibling} />
      </>
    );
    expect(controllerRenders).not.toContain(sibling.id);
    expect(screen.getByTestId(`os-window-frame-${primary.id}`).parentElement).toHaveStyle({
      transform: "translate(200px,0px)",
    });
  });
});

describe("OsWindow project badge", () => {
  it.each([
    ["ws_beta", "beta"],
    ["@user", "User"],
  ])(
    "Should name the bound scope %s in the chrome on the All projects desktop",
    (binding, name) => {
      partition = "global";
      windows = {
        "window:tasks": windowState({
          route: { pathname: "/tasks", search: { workspace: binding } },
        }),
      };
      render(<OsWindow frame={frameModel()} />);

      expect(screen.getByTestId("os-window-project-window:tasks")).toHaveTextContent(name);
    }
  );

  it("Should show no badge on a catalog window or on a project desktop", () => {
    partition = "global";
    const view = render(<OsWindow frame={frameModel()} />);
    expect(screen.queryByTestId("os-window-project-window:tasks")).not.toBeInTheDocument();

    partition = "ws_alpha";
    windows = {
      "window:tasks": windowState({
        route: { pathname: "/tasks", search: { workspace: "ws_beta" } },
      }),
    };
    view.rerender(<OsWindow frame={frameModel()} />);
    expect(screen.queryByTestId("os-window-project-window:tasks")).not.toBeInTheDocument();
  });
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
    expect(screen.getByRole("button", { name: "Restore window" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
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
    // This suite stubs subscriptions; a new frame prop delivers the simulated update.
    view.rerender(<OsWindow frame={{ ...minimizedFrame }} />);

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
    // This suite stubs subscriptions; a new frame prop delivers the simulated update.
    view.rerender(<OsWindow frame={{ ...frame }} />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");

    presentation = "floating";
    // This suite stubs subscriptions; a new frame prop delivers the simulated update.
    view.rerender(<OsWindow frame={{ ...frame }} />);
    expect(screen.getByRole("textbox", { name: "Task draft" })).toBe(draft);
    expect(draft).toHaveValue("Keep this task context");
  });

  it("Should mount and unmount the deck exactly as a frame crosses the second-member boundary (UT-070, UT-071)", () => {
    windows = {
      "window:tasks": windowState(),
      "window:jobs": windowState({ id: "window:jobs", app: "automations", layer: 3 }),
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
      "window:jobs": windowState({ id: "window:jobs", app: "automations", layer: 3 }),
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
      "window:jobs": windowState({ id: "window:jobs", app: "automations", layer: 3 }),
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

  it("Should show the app's loading frame for the window's route while its surface suspends, in place of the generic rows", () => {
    pendingJobsController = true;
    jobsLoadingFallback = true;
    windows = {
      "window:jobs": windowState({
        id: "window:jobs",
        app: "automations",
        route: { pathname: "/automations/jobs/morning-digest", search: {} },
      }),
    };
    render(
      <OsWindow
        frame={frameModel({
          id: "window:jobs",
          members: ["window:jobs"],
          activeWindowId: "window:jobs",
        })}
      />
    );

    const body = screen.getByTestId("os-window-window:jobs");
    const fallback = within(body).getByRole("status", { name: "Loading Automations" });
    expect(fallback).toHaveAttribute("data-route", "/automations/jobs/morning-digest");
    // Hosted in its own window's scope, like the surface it stands in for.
    expect(fallback).toHaveAttribute("data-scope", "window:jobs");
    expect(within(body).queryByRole("status", { name: "Loading" })).not.toBeInTheDocument();
  });
});

// History semantics of in-window navigation: a link inside the window body is a push (the window's
// back stack keeps it), including links in content that owns its own view transition; ownership
// only exempts the body from the drill's snapshot (tokens-runtime.css), never from history.
describe("OsWindow navigation classification", () => {
  it.each([
    ["a plain body link", "Open task"],
    ["a link inside content that owns its view transition", "Terminal section"],
  ])("Should record %s as a push", (_case, linkName) => {
    linksController = true;
    render(<OsWindow frame={frameModel()} />);

    fireEvent.click(screen.getByRole("link", { name: linkName }));

    expect(navigation.noteNavigateMode).toHaveBeenCalledExactlyOnceWith("push");
  });
});
