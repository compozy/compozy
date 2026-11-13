// Suite: the layout review gate
// Invariant: a layout reaches the workspace only at the exact revision the daemon
// validated and previewed. Any edit after a check disarms Apply, and a document
// the daemon refused never reaches preview at all. The checked status counts
// what changes in the product's nouns, singular for one and left out at zero.
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
  convertWindowManagerLayoutDocument: vi.fn(),
  previewWindowManagerLayout: apiMocks.preview,
  validateWindowManagerLayout: apiMocks.validate,
}));

import { useWindowManagerLayoutEditor } from "../../hooks/use-window-manager-layout-editor";
import type {
  WindowManagerLayoutChanges,
  WindowManagerLayoutDocument,
  WindowManagerLayoutState,
} from "../../lib/window-manager-layout-types";
import { LayoutReviewBar } from "../layouts/layout-review-bar";

const INITIAL: WindowManagerLayoutState = {
  revision: 7,
  document: {
    version: 6,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: "Build",
        order: 0,
        activeTabId: "tab-main",
        tabs: [
          {
            id: "tab-main",
            pinned: false,
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
        navStack: [],
        desktopId: "desktop-one",
        tabId: "tab-main",
      },
    },
    overrides: {},
  },
} as unknown as WindowManagerLayoutState;

/** One desktop whose tab holds a split nested in a split: two split nodes, three leaves. */
const SPLIT_DOCUMENT = {
  ...INITIAL.document,
  desktops: [
    {
      id: "desktop-one",
      name: "Build",
      order: 0,
      activeTabId: "tab-main",
      tabs: [
        {
          id: "tab-main",
          pinned: false,
          root: {
            id: "split-outer",
            kind: "split",
            axis: "horizontal",
            weights: [0.5, 0.5],
            children: [
              { id: "leaf-one", kind: "leaf", windowId: "app:tasks" },
              {
                id: "split-inner",
                kind: "split",
                axis: "vertical",
                weights: [0.5, 0.5],
                children: [
                  { id: "leaf-two", kind: "leaf", windowId: "app:terminal" },
                  { id: "leaf-three", kind: "leaf", windowId: "app:files" },
                ],
              },
            ],
          },
        },
      ],
    },
  ],
} as unknown as WindowManagerLayoutDocument;

function changesWith(counts: Partial<WindowManagerLayoutChanges>): WindowManagerLayoutChanges {
  return {
    desktopIds: [],
    windowIds: [],
    tabIds: [],
    nodeIds: [],
    clientIds: [],
    tabSplit: [],
    tabUnsplit: [],
    ...counts,
  };
}

function Harness() {
  const editor = useWindowManagerLayoutEditor("workspace-a", "marketing", INITIAL);
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
        tabIds: [],
        nodeIds: [],
        clientIds: [],
        tabSplit: [],
        tabUnsplit: [],
      },
      diagnostics: [],
      document: INITIAL.document,
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
    expect(apiMocks.validate.mock.calls.at(-1)?.[1]).toBe("marketing");
    expect(apiMocks.preview.mock.calls.at(-1)?.[1]).toBe("marketing");
    fireEvent.click(screen.getByTestId("layout-review-apply"));

    await waitFor(() => expect(apiMocks.apply).toHaveBeenCalledTimes(1));
    const [, profile, revision, document] = apiMocks.apply.mock.calls[0] ?? [];
    expect(profile).toBe("marketing");
    expect(revision).toBe(7);
    expect(document.desktops[0].name).toBe("Build!!");
  });

  it("Should never preview a document the daemon refused", async () => {
    apiMocks.validate.mockResolvedValue({
      workspaceId: "workspace-a",
      valid: false,
      diagnostics: [
        {
          code: "topology.split_weight_sum",
          path: "desktops[0].tabs[0].root",
          message: "split weights must sum to 1",
        },
      ],
    });
    renderBar();
    fireEvent.click(screen.getByTestId("edit-desktop"));
    fireEvent.click(screen.getByTestId("layout-review-check"));

    await waitFor(() => expect(screen.getByText("topology.split_weight_sum")).toBeInTheDocument());
    expect(apiMocks.preview).not.toHaveBeenCalled();
    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();
    expect(screen.getByTestId("layout-review-status")).toHaveTextContent("1 problem to fix");

    fireEvent.click(screen.getByTestId("edit-desktop"));
    expect(screen.queryByText("topology.split_weight_sum")).not.toBeInTheDocument();
    expect(screen.getByTestId("layout-review-apply")).toBeDisabled();
  });

  it.each([
    {
      name: "one of each",
      changes: changesWith({
        desktopIds: ["desktop-one"],
        tabIds: ["tab-main"],
        nodeIds: ["split-inner", "leaf-two"],
        windowIds: ["app:terminal"],
      }),
      summary: "Checked · changes 1 desktop, 1 tab, 1 split, 1 window",
    },
    {
      name: "many of each",
      changes: changesWith({
        desktopIds: ["desktop-one", "desktop-two"],
        tabIds: ["tab-main", "tab-b", "tab-c", "tab-d", "tab-e"],
        // Seven changed nodes: two splits and five leaves. Leaves are windows, not splits.
        nodeIds: [
          "split-outer",
          "split-inner",
          "leaf-one",
          "leaf-two",
          "leaf-three",
          "leaf-4",
          "leaf-5",
        ],
        windowIds: ["app:tasks", "app:terminal", "app:files", "app:a", "app:b", "app:c"],
      }),
      summary: "Checked · changes 2 desktops, 5 tabs, 2 splits, 6 windows",
    },
    {
      name: "zero splits and windows",
      changes: changesWith({ tabIds: ["tab-main", "tab-b"], nodeIds: ["leaf-one"] }),
      summary: "Checked · changes 2 tabs",
    },
    {
      name: "nothing counted",
      changes: changesWith({ clientIds: ["client-a"] }),
      summary: "Checked · nothing changes",
    },
  ])("Should count $name in the checked status", async ({ changes, summary }) => {
    apiMocks.preview.mockResolvedValue({
      revision: 7,
      changed: true,
      changes,
      diagnostics: [],
      document: SPLIT_DOCUMENT,
    });
    renderBar();
    fireEvent.click(screen.getByTestId("edit-desktop"));
    fireEvent.click(screen.getByTestId("layout-review-check"));

    await waitFor(() => expect(screen.getByTestId("layout-review-apply")).toBeEnabled());
    // The whole message, so a left-out zero count cannot hide behind a substring match.
    expect(screen.getByText(summary)).toBeInTheDocument();
    expect(screen.getByTestId("layout-review-status")).not.toHaveTextContent(/branch/);
  });
});
