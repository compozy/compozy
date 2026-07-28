// Suite: the layout review gate
// Invariant: a layout reaches the workspace only at the exact revision the daemon
// validated and previewed. Any edit after a check disarms Apply, and a document
// the daemon refused never reaches preview at all.
// Boundary IN: the editor model plus the validate/preview/apply adapter.
// Boundary OUT: the canvas, the inspector, the profile grid.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMocks = vi.hoisted(() => ({
  apply: vi.fn(),
  preview: vi.fn(),
  validate: vi.fn(),
}));

vi.mock("../../adapters/window-manager-layouts-api", () => ({
  applyWindowManagerLayout: apiMocks.apply,
  previewWindowManagerLayout: apiMocks.preview,
  validateWindowManagerLayout: apiMocks.validate,
}));

import { useWindowManagerLayoutEditor } from "../../hooks/use-window-manager-layout-editor";
import type { WindowManagerLayoutState } from "../../lib/window-manager-layout-types";
import { LayoutReviewBar } from "../layouts/layout-review-bar";

const INITIAL: WindowManagerLayoutState = {
  revision: 7,
  document: {
    version: 2,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: "Build",
        order: 0,
        purpose: "standard",
        focusOwner: null,
        floating: [],
        groups: [
          {
            id: "group-main",
            frame: { x: 0, y: 0, w: 1, h: 1 },
            root: { id: "leaf-one", kind: "leaf", windowId: "app:tasks" },
          },
        ],
      },
    ],
    windows: {
      "app:tasks": {
        id: "app:tasks",
        app: "tasks",
        instanceKey: null,
        route: { pathname: "/tasks", search: {} },
        placement: "tiled",
        desktopId: "desktop-one",
        floatingRect: { x: 0.2, y: 0.2, w: 0.4, h: 0.4 },
        minimized: false,
        returnAnchor: null,
      },
    },
    overrides: {},
  },
};

function Harness() {
  const editor = useWindowManagerLayoutEditor("workspace-a", INITIAL);
  return (
    <>
      <button
        data-testid="edit-desktop"
        type="button"
        onClick={() => {
          const next = structuredClone(editor.draft);
          const desktop = next.desktops[0];
          if (desktop) desktop.name = `${desktop.name}!`;
          editor.updateDraft(next);
        }}
      >
        Rename desktop
      </button>
      <LayoutReviewBar editor={editor} />
    </>
  );
}

function renderBar() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  render(<Harness />, { wrapper });
}

describe("LayoutReviewBar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMocks.validate.mockResolvedValue({
      workspaceId: "workspace-a",
      valid: true,
      diagnostics: [],
    });
    apiMocks.preview.mockResolvedValue({
      revision: 7,
      changed: true,
      changes: {
        desktopIds: ["desktop-one"],
        windowIds: [],
        groupIds: [],
        nodeIds: [],
        clientIds: [],
      },
      diagnostics: [],
    });
    apiMocks.apply.mockResolvedValue({ revision: 8, applied: true, diagnostics: [] });
  });

  it("Should keep every action shut while the draft matches the workspace", () => {
    renderBar();

    expect(screen.getByTestId("layout-review-check")).toBeDisabled();
    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();
    expect(screen.getByTestId("layout-review-status")).toHaveTextContent("Revision 7");
  });

  it("Should apply only the exact document that passed validate and preview", async () => {
    renderBar();
    fireEvent.click(screen.getByTestId("edit-desktop"));

    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();
    fireEvent.click(screen.getByTestId("layout-review-check"));
    await waitFor(() => expect(screen.getByTestId("layout-review-apply")).toBeEnabled());

    // A further edit invalidates the check it has not been through.
    fireEvent.click(screen.getByTestId("edit-desktop"));
    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();

    fireEvent.click(screen.getByTestId("layout-review-check"));
    await waitFor(() => expect(screen.getByTestId("layout-review-apply")).toBeEnabled());
    fireEvent.click(screen.getByTestId("layout-review-apply"));

    await waitFor(() => expect(apiMocks.apply).toHaveBeenCalledTimes(1));
    const [, revision, document] = apiMocks.apply.mock.calls[0] ?? [];
    expect(revision).toBe(7);
    expect(document.desktops[0].name).toBe("Build!!");
  });

  it("Should never preview a document the daemon refused", async () => {
    apiMocks.validate.mockResolvedValue({
      workspaceId: "workspace-a",
      valid: false,
      diagnostics: [
        { code: "topology.group_overlap", path: "groups.group-main", message: "groups overlap" },
      ],
    });
    renderBar();
    fireEvent.click(screen.getByTestId("edit-desktop"));
    fireEvent.click(screen.getByTestId("layout-review-check"));

    await waitFor(() => expect(screen.getByText("topology.group_overlap")).toBeInTheDocument());
    expect(apiMocks.preview).not.toHaveBeenCalled();
    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();
    expect(screen.getByTestId("layout-review-status")).toHaveTextContent("1 problem to fix");
  });
});
