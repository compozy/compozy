// Suite: layout draft reconciliation
// Invariant: the workspace layout is live daemon state shared with the OS shell,
// so it can move while someone is editing. A clean editor follows it; a dirty one
// keeps the edits and loses its review, because Apply carries `expected_revision`
// and must not ride a revision the daemon never checked the draft against.
// Imported files and layouts saved before tabs and splits reach the draft only
// as the version 6 document the daemon converted them to, and an exported file
// is that wire document, so it imports back unchanged. Every import failure reads
// in product wording; parser, transport and daemon codes never reach the editor.
// Boundary IN: successive layout states from the query; the convert adapter.
// Boundary OUT: canvas rendering, transport, the profile editor.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMocks = vi.hoisted(() => ({ convert: vi.fn() }));

vi.mock("../../adapters/window-manager-layouts-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/window-manager-layouts-api")>()),
  convertWindowManagerLayoutDocument: apiMocks.convert,
}));

import { renameLayoutDesktop } from "../../lib/window-manager-layout-mutations";
import { parseWindowManagerLayoutDocument } from "../../lib/window-manager-layout-schema";

import {
  useWindowManagerLayoutEditor,
  windowManagerLayoutEditorLogic,
} from "../use-window-manager-layout-editor";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutState,
} from "../../lib/window-manager-layout-types";

function documentWith(desktopName: string): WindowManagerLayoutDocument {
  return {
    version: 6,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: desktopName,
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
  } as unknown as WindowManagerLayoutDocument;
}

function stateAt(revision: number, desktopName: string): WindowManagerLayoutState {
  return { revision, document: documentWith(desktopName) };
}

function renderEditor(initial: WindowManagerLayoutState, profile = "marketing") {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(
    ({ state, profile: currentProfile }) =>
      useWindowManagerLayoutEditor("workspace-a", currentProfile, state),
    {
      initialProps: { state: initial, profile },
      wrapper,
    }
  );
}

function renameDesktop(document: WindowManagerLayoutDocument, name: string) {
  return renameLayoutDesktop(document, "desktop-one", name);
}

const EARLIER_WIRE = {
  version: 5,
  workspace_id: "workspace-b",
  desktops: [{ id: "desktop-one", name: "Classic", order: 0, groups: [], floating: [] }],
  windows: {},
  overrides: {},
};

describe("useWindowManagerLayoutEditor", () => {
  beforeEach(() => {
    apiMocks.convert.mockReset();
  });

  it("Should fence a stale daemon baseline during render-time reconciliation", () => {
    const { result, rerender } = renderEditor(stateAt(42, "Ship"));
    rerender({ state: stateAt(41, "Build"), profile: "marketing" });

    expect(result.current.revision).toBe(42);
    expect(result.current.draft.desktops[0]?.name).toBe("Ship");
  });

  it("Should ignore a late Apply result after the draft changes", () => {
    const store = windowManagerLayoutEditorLogic.createStore({ initial: stateAt(42, "Ship") });
    let snapshot = store.getInitialSnapshot();
    const fingerprint = snapshot.context.baselineFingerprint;
    [snapshot] = store.transition(snapshot, {
      type: "applyRequested",
      execute: async () => undefined,
    });
    [snapshot] = store.transition(snapshot, {
      type: "draftChanged",
      draft: renameDesktop(snapshot.context.draft, "Mine"),
    });
    [snapshot] = store.transition(snapshot, {
      type: "applySucceeded",
      fingerprint,
      operation: 1,
      revision: 42,
    });

    expect(snapshot.context.draft.desktops[0]?.name).toBe("Mine");
    expect(snapshot.context.phase).toBe("dirty");
  });

  it("Should follow the daemon while the draft is untouched", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    expect(result.current.dirty).toBe(false);

    rerender({ state: stateAt(42, "Ship"), profile: "marketing" });

    expect(result.current.revision).toBe(42);
    expect(result.current.draft.desktops[0]?.name).toBe("Ship");
    expect(result.current.dirty).toBe(false);
  });

  it("Should keep unapplied edits when the workspace moves underneath them", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });
    expect(result.current.dirty).toBe(true);

    rerender({ state: stateAt(42, "Theirs"), profile: "marketing" });

    expect(result.current.draft.desktops[0]?.name).toBe("Mine");
    expect(result.current.dirty).toBe(true);
    expect(result.current.revision).toBe(42);
    expect(result.current.reviewCurrent).toBe(false);
    expect(result.current.reviewed).toBe(null);
  });

  it("Should reset to what the daemon holds now, not to a stale snapshot", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });
    rerender({ state: stateAt(42, "Theirs"), profile: "marketing" });

    act(() => {
      result.current.reset();
    });

    expect(result.current.draft.desktops[0]?.name).toBe("Theirs");
    expect(result.current.dirty).toBe(false);
  });

  it("Should isolate drafts when only the profile changes in one workspace", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Marketing"));
    act(() => result.current.updateDraft(renameDesktop(result.current.draft, "Mine")));

    rerender({ state: stateAt(9, "Sales"), profile: "sales" });

    expect(result.current.draft.desktops[0]?.name).toBe("Sales");
    expect(result.current.revision).toBe(9);
    expect(result.current.dirty).toBe(false);
  });

  it("Should import a file of any accepted version as the document the daemon converted it to", async () => {
    apiMocks.convert.mockResolvedValue({
      ...documentWith("Converted"),
      workspaceId: "workspace-b",
    });
    const { result } = renderEditor(stateAt(41, "Build"));
    const file = new File([JSON.stringify(EARLIER_WIRE)], "layout.json", {
      type: "application/json",
    });

    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(apiMocks.convert).toHaveBeenCalledTimes(1);
    const [workspace, profile, wire] = apiMocks.convert.mock.calls[0] ?? [];
    expect(workspace).toBe("workspace-a");
    expect(profile).toBe("marketing");
    expect(wire).toEqual(EARLIER_WIRE);
    expect(result.current.draft.version).toBe(6);
    expect(result.current.draft.desktops[0]?.name).toBe("Converted");
    // The draft always belongs to the workspace being edited.
    expect(result.current.draft.workspaceId).toBe("workspace-a");
    expect(result.current.dirty).toBe(true);
  });

  it("Should export a layout file that imports back as the same draft [US-022.AC-1]", async () => {
    apiMocks.convert.mockImplementation(
      async (_workspace: string, _profile: string, wire: unknown) =>
        parseWindowManagerLayoutDocument(wire)
    );
    const { result } = renderEditor(stateAt(41, "Build"));
    act(() => result.current.updateDraft(renameDesktop(result.current.draft, "Exported")));
    const draft = result.current.draft;

    const exported = result.current.exportDocument();

    // The strict wire schema refuses unknown fields, as the daemon's decoder does.
    expect(exported).toMatchObject({ version: 6, workspace_id: "workspace-a" });
    expect(exported).not.toHaveProperty("workspaceId");
    expect(parseWindowManagerLayoutDocument(exported)).toEqual(draft);

    act(() => result.current.reset());
    expect(result.current.draft.desktops[0]?.name).toBe("Build");
    const file = new File([JSON.stringify(exported, null, 2)], "layout.json", {
      type: "application/json",
    });
    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(apiMocks.convert.mock.calls[0]?.[2]).toEqual(exported);
    expect(result.current.importError).toBeNull();
    expect(result.current.draft).toEqual(draft);
  });

  // One row per class of daemon answer to the conversion request. The bodies carry
  // codes, never sentences, and none of them may reach the editor.
  it.each([
    {
      name: "a document version or shape it does not accept",
      status: 422,
      code: "window_manager_invalid_topology",
      shown: "CompozyOS can't open this layout document. It may come from a newer version.",
    },
    {
      name: "a document whose window bindings it does not accept",
      status: 400,
      code: "window_binding_invalid",
      shown: "CompozyOS can't open this layout document. It may come from a newer version.",
    },
    {
      name: "a document over its body limit",
      status: 413,
      code: null,
      shown: "This layout document is too large to open.",
    },
    {
      name: "a revision the editor has moved past",
      status: 409,
      code: "window_manager_revision_conflict",
      shown: "Couldn't check the layout file. Try again.",
    },
    {
      name: "a project that is gone",
      status: 404,
      code: "window_manager_workspace_not_found",
      shown: "Layouts for this project and profile aren't available here.",
    },
    {
      name: "a profile that is gone",
      status: 404,
      code: "profile_not_found",
      shown: "Layouts for this project and profile aren't available here.",
    },
    {
      name: "a profile request it refuses",
      status: 400,
      code: "profile_selection_conflict",
      shown: "Layouts for this project and profile aren't available here.",
    },
    {
      name: "a profile in a state it refuses",
      status: 409,
      code: "profile_archived",
      shown: "Layouts for this project and profile aren't available here.",
    },
    {
      name: "a caller it does not recognise",
      status: 401,
      code: null,
      shown: "Layouts for this project and profile aren't available here.",
    },
    {
      name: "a caller it does not allow",
      status: 403,
      code: "agent_scope_denied",
      shown: "Layouts for this project and profile aren't available here.",
    },
  ])(
    "Should keep the draft and word the daemon's refusal of $name",
    async ({ status, code, shown }) => {
      // Read after the hook has loaded: a top-level import of the adapter would load it
      // first, and the hook, reached again through `@/systems/os` while the mock factory
      // runs, would bind the real adapter instead of the mock.
      const { WindowManagerLayoutsApiError } =
        await import("../../adapters/window-manager-layouts-api");
      apiMocks.convert.mockRejectedValue(
        new WindowManagerLayoutsApiError(code ?? "", status, code)
      );
      const { result } = renderEditor(stateAt(41, "Build"));
      const file = new File([JSON.stringify({ ...EARLIER_WIRE, version: 7 })], "layout.json");

      await act(async () => {
        await result.current.importDocument(file);
      });

      expect(result.current.importError).toBe(shown);
      expect(result.current.draft.desktops[0]?.name).toBe("Build");
    }
  );

  it("Should word a refused saved layout as the saved layout, not as a file", async () => {
    const { WindowManagerLayoutsApiError } =
      await import("../../adapters/window-manager-layouts-api");
    apiMocks.convert.mockRejectedValue(
      new WindowManagerLayoutsApiError(
        "window_manager_invalid_command",
        422,
        "window_manager_invalid_command"
      )
    );
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.loadSource({ kind: "earlier", version: 5, wire: EARLIER_WIRE });
    });

    expect(result.current.importError).toBe("This saved layout could not be opened.");
    expect(result.current.draft.desktops[0]?.name).toBe("Build");
  });

  it("Should word a check that did not finish without the transport's text", async () => {
    apiMocks.convert.mockRejectedValue(new TypeError("Failed to fetch"));
    const { result } = renderEditor(stateAt(41, "Build"));
    const file = new File([JSON.stringify(EARLIER_WIRE)], "layout.json");

    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(result.current.importError).toBe("Couldn't check the layout file. Try again.");
    expect(result.current.draft.desktops[0]?.name).toBe("Build");
  });

  it.each([
    { name: "plain text", content: "this is a plain text file" },
    { name: "an empty file", content: "" },
    { name: "a JSON array", content: "[1, 2]" },
    { name: "JSON null", content: "null" },
  ])(
    "Should refuse $name as not a layout document without asking the daemon",
    async ({ content }) => {
      const { result } = renderEditor(stateAt(41, "Build"));

      await act(async () => {
        await result.current.importDocument(new File([content], "layout.json"));
      });

      expect(apiMocks.convert).not.toHaveBeenCalled();
      expect(result.current.importError).toBe("The selected file is not a layout document.");
      expect(result.current.draft.desktops[0]?.name).toBe("Build");
    }
  );

  it("Should say a file could not be read when the browser cannot read it", async () => {
    const { result } = renderEditor(stateAt(41, "Build"));
    const file = new File(["{}"], "layout.json");
    vi.spyOn(file, "text").mockRejectedValue(
      new DOMException("NotReadableError", "NotReadableError")
    );

    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(apiMocks.convert).not.toHaveBeenCalled();
    expect(result.current.importError).toBe("The selected file could not be read.");
  });

  it("Should open a profile saved before tabs and splits through the daemon's conversion", async () => {
    apiMocks.convert.mockResolvedValue(documentWith("Classic"));
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.loadSource({ kind: "earlier", version: 5, wire: EARLIER_WIRE });
    });

    expect(apiMocks.convert.mock.calls[0]?.[2]).toEqual(EARLIER_WIRE);
    expect(result.current.draft.version).toBe(6);
    expect(result.current.draft.desktops[0]?.name).toBe("Classic");
  });

  it("Should open a current profile directly, without a daemon round trip", async () => {
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.loadSource({ kind: "current", document: documentWith("Saved") });
    });

    expect(apiMocks.convert).not.toHaveBeenCalled();
    expect(result.current.draft.desktops[0]?.name).toBe("Saved");
  });
});
