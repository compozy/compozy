// Suite: the layout canvas ↔ projector contract
// Invariant: the canvas draws what `projectLayout` produced — one tile per
// projected window, one handle per seam — and marks the topology the daemon
// refuses (overlapping groups) rather than drawing it as if it were fine.
// Boundary IN: a draft desktop + the global config's gaps.
// Boundary OUT: drag arithmetic (owned by seam-preview), the inspector.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { WindowManagerConfig } from "@/systems/os";

import { layoutCanvasProjection } from "../../lib/window-manager-layout-canvas";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
} from "../../lib/window-manager-layout-types";
import { LayoutCanvas } from "../layouts/layout-canvas";

const CONFIG = {
  gaps: { inner: 8, top: 8, right: 8, bottom: 8, left: 8 },
} as WindowManagerConfig;

function windowAt(id: string, desktopId: string) {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    placement: "tiled" as const,
    desktopId,
    floatingRect: { x: 0.2, y: 0.2, w: 0.4, h: 0.4 },
    minimized: false,
    returnAnchor: null,
  };
}

function documentWith(desktop: WindowManagerLayoutDesktop): WindowManagerLayoutDocument {
  return {
    version: 2,
    workspaceId: "workspace-a",
    desktops: [desktop],
    windows: {
      "app:one": windowAt("app:one", desktop.id),
      "app:two": windowAt("app:two", desktop.id),
      "app:float": { ...windowAt("app:float", desktop.id), placement: "floating" },
    },
    overrides: {},
  };
}

const SPLIT_DESKTOP: WindowManagerLayoutDesktop = {
  id: "desktop-one",
  name: "Build",
  order: 0,
  purpose: "standard",
  focusOwner: null,
  floating: ["app:float"],
  groups: [
    {
      id: "group-main",
      frame: { x: 0, y: 0, w: 1, h: 1 },
      root: {
        id: "split-main",
        kind: "split",
        axis: "horizontal",
        weights: [0.5, 0.5],
        children: [
          { id: "leaf-one", kind: "leaf", windowId: "app:one" },
          { id: "leaf-two", kind: "leaf", windowId: "app:two" },
        ],
      },
    },
  ],
};

function renderCanvas(desktop: WindowManagerLayoutDesktop) {
  const document = documentWith(desktop);
  const projection = layoutCanvasProjection(document, desktop, CONFIG, 1);
  render(
    <LayoutCanvas
      config={CONFIG}
      desktop={desktop}
      document={document}
      projection={projection}
      selection={null}
      onDesktopChange={vi.fn()}
      onFloatingRectChange={vi.fn()}
      onSelect={vi.fn()}
    />
  );
  return projection;
}

describe("LayoutCanvas", () => {
  it("Should draw one tile per projected window and one handle per seam", () => {
    const projection = renderCanvas(SPLIT_DESKTOP);

    expect(projection.windows).toHaveLength(2);
    expect(projection.seams).toHaveLength(1);
    for (const projected of projection.windows) {
      expect(screen.getByTestId(`layout-canvas-tile-${projected.windowId}`)).toBeInTheDocument();
    }
    for (const seam of projection.seams) {
      expect(screen.getByTestId(`layout-canvas-seam-${seam.id}`)).toBeInTheDocument();
    }
  });

  it("Should give every divider and group edge a keyboard-operable slider", () => {
    renderCanvas(SPLIT_DESKTOP);

    const sliders = screen.getAllByRole("slider");
    // One seam plus four edges of the single group.
    expect(sliders).toHaveLength(5);
    for (const slider of sliders) {
      expect(slider).toHaveAttribute("aria-valuenow");
      expect(slider).toHaveAttribute("tabindex", "0");
    }
  });

  it("Should show floating windows above the tiled ones", () => {
    renderCanvas(SPLIT_DESKTOP);

    expect(screen.getByTestId("layout-canvas-floating-app:float")).toBeInTheDocument();
  });

  it("Should mark the groups whose frames cover each other", () => {
    const overlapping: WindowManagerLayoutDesktop = {
      ...SPLIT_DESKTOP,
      floating: [],
      groups: [
        {
          id: "group-a",
          frame: { x: 0, y: 0, w: 0.7, h: 1 },
          root: { id: "leaf-one", kind: "leaf", windowId: "app:one" },
        },
        {
          id: "group-b",
          frame: { x: 0.5, y: 0, w: 0.5, h: 1 },
          root: { id: "leaf-two", kind: "leaf", windowId: "app:two" },
        },
      ],
    };
    renderCanvas(overlapping);

    expect(screen.getByTestId("layout-canvas-group-group-a")).toHaveAttribute(
      "data-overlapping",
      "true"
    );
    expect(screen.getByTestId("layout-canvas-group-group-b")).toHaveAttribute(
      "data-overlapping",
      "true"
    );
  });

  it("Should say when a desktop has nothing on it", () => {
    renderCanvas({ ...SPLIT_DESKTOP, groups: [], floating: [] });

    expect(screen.getByText("No windows on this desktop")).toBeInTheDocument();
  });
});
