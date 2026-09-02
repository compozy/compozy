// Suite: layout canvas board summary
// Invariant: the board summary reports the exact number of visible zoomed windows.
// Boundary IN: one layout document and its projection; OUT: projection and editor behavior.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { LayoutProjection } from "@/systems/os";

import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutWindow,
} from "../../lib/window-manager-layout-types";
import { LayoutCanvasBoardBar } from "../layouts/layout-canvas-board-bar";

const DESKTOP: WindowManagerLayoutDesktop = {
  id: "desktop-one",
  name: "One",
  order: 0,
  groups: [],
  floating: [],
  floatingStacks: [],
};

function zoomedWindow(id: string): WindowManagerLayoutWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    placement: "tiled",
    desktopId: DESKTOP.id,
    floatingRect: { x: 0, y: 0, w: 1, h: 1 },
    minimized: false,
    zoomed: true,
    returnAnchor: null,
  };
}

describe("LayoutCanvasBoardBar", () => {
  it("Should render the calculated visible zoom count", () => {
    const document: WindowManagerLayoutDocument = {
      version: 4,
      workspaceId: "workspace-a",
      desktops: [DESKTOP],
      windows: {
        one: zoomedWindow("one"),
        two: zoomedWindow("two"),
      },
      overrides: {},
    };
    const projection: LayoutProjection = {
      revision: 1,
      desktopId: DESKTOP.id,
      workArea: { x: 0, y: 0, w: 1440, h: 900 },
      windows: [],
      stacks: [],
      seams: [],
      frameSeams: [],
      diagnostics: [],
    };

    render(<LayoutCanvasBoardBar desktop={DESKTOP} document={document} projection={projection} />);

    expect(screen.getByText("2 zoomed")).toBeInTheDocument();
  });
});
