// Suite: saved-layout selection and destruction gates
// Invariant: neither loading nor deleting a saved layout destroys work without
// asking, and a delete leaves no field of the removed record behind to seed the
// next one.
// Boundary IN: the visible profile records + whether the draft has unapplied edits.
// Boundary OUT: the card grid's markup, transport, the layout draft itself.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMocks = vi.hoisted(() => ({
  deleteProfile: vi.fn(),
  putProfile: vi.fn(),
}));

vi.mock("../../adapters/window-manager-layouts-api", () => ({
  deleteWindowManagerLayoutProfile: apiMocks.deleteProfile,
  putWindowManagerLayoutProfile: apiMocks.putProfile,
}));

import { useWindowManagerLayoutProfiles } from "../use-window-manager-layout-profiles";
import { settingsKeys } from "../../lib/query-keys";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutResourceRecord,
} from "../../lib/window-manager-layout-types";

const DOCUMENT: WindowManagerLayoutDocument = {
  version: 2,
  workspaceId: "workspace-a",
  desktops: [],
  windows: {},
  overrides: {},
};

const RECORD: WindowManagerLayoutResourceRecord = {
  kind: "window_layout",
  id: "two-up-review",
  version: 4,
  scope: { kind: "global", id: "" },
  spec: {
    version: 1,
    id: "two-up-review",
    displayName: "Two-up review",
    aspectVariant: "landscape",
    participantSlots: [],
    overflowPolicy: "reject",
    document: { ...DOCUMENT, workspaceId: "" },
  },
  createdAt: "2026-07-22T00:00:00Z",
  updatedAt: "2026-07-22T00:00:00Z",
};

function renderProfiles(draftDirty: boolean) {
  const loaded: WindowManagerLayoutDocument[] = [];
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const view = renderHook(
    () =>
      useWindowManagerLayoutProfiles({
        workspaceId: "workspace-a",
        document: DOCUMENT,
        profiles: [RECORD],
        draftDirty,
        onLoad: next => loaded.push(next),
      }),
    { wrapper }
  );
  return { ...view, loaded, queryClient };
}

describe("useWindowManagerLayoutProfiles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMocks.deleteProfile.mockResolvedValue(undefined);
  });

  it("Should load straight away when there is nothing unapplied to lose", () => {
    const { result, loaded } = renderProfiles(false);

    act(() => {
      result.current.requestLoad(RECORD);
    });

    expect(result.current.pendingLoad).toBe(null);
    expect(loaded).toHaveLength(1);
    expect(loaded[0]?.workspaceId).toBe("workspace-a");
  });

  it("Should ask before replacing a dirty draft", () => {
    const { result, loaded } = renderProfiles(true);

    act(() => {
      result.current.requestLoad(RECORD);
    });

    expect(result.current.pendingLoad).toEqual(RECORD);
    expect(loaded).toHaveLength(0);
  });

  it("Should leave the draft untouched when the person keeps editing", () => {
    const { result, loaded } = renderProfiles(true);
    act(() => {
      result.current.requestLoad(RECORD);
    });

    act(() => {
      result.current.cancelLoad();
    });

    expect(result.current.pendingLoad).toBe(null);
    expect(loaded).toHaveLength(0);
  });

  it("Should replace the draft once the person confirms", () => {
    const { result, loaded } = renderProfiles(true);
    act(() => {
      result.current.requestLoad(RECORD);
    });

    act(() => {
      result.current.confirmLoad();
    });

    expect(result.current.pendingLoad).toBe(null);
    expect(loaded).toHaveLength(1);
  });

  it("Should not delete until the person confirms", () => {
    const { result } = renderProfiles(false);
    act(() => {
      result.current.selectProfile(RECORD);
    });

    act(() => {
      result.current.requestDelete();
    });

    expect(result.current.pendingDelete).toEqual(RECORD);
    expect(apiMocks.deleteProfile).not.toHaveBeenCalled();

    act(() => {
      result.current.cancelDelete();
    });

    expect(result.current.pendingDelete).toBe(null);
    expect(apiMocks.deleteProfile).not.toHaveBeenCalled();
  });

  it("Should clear every field of the deleted record, not only its name and id", async () => {
    const { result } = renderProfiles(false);
    act(() => {
      result.current.selectProfile(RECORD);
    });
    expect(result.current.scope).toBe("global");
    expect(result.current.aspect).toBe("landscape");
    expect(result.current.overflow).toBe("reject");

    await act(async () => {
      result.current.requestDelete();
      await result.current.remove.mutateAsync();
    });

    expect(apiMocks.deleteProfile).toHaveBeenCalledWith("workspace-a", "two-up-review", 4);
    expect(result.current.id).toBe("");
    expect(result.current.displayName).toBe("");
    expect(result.current.scope).toBe("workspace");
    expect(result.current.aspect).toBe("any");
    expect(result.current.overflow).toBe("stack");
    expect(result.current.pendingDelete).toBe(null);
  });

  it("Should preserve the current version and replace the previous scoped cache identity on save", async () => {
    const workspaceRecord = {
      ...RECORD,
      scope: { kind: "workspace" as const, id: "workspace-a" },
      version: 5,
    };
    apiMocks.putProfile.mockResolvedValue(workspaceRecord);
    const { result, queryClient } = renderProfiles(false);
    queryClient.setQueryData(settingsKeys.windowManagerLayoutProfiles("workspace-a"), [RECORD]);

    act(() => {
      result.current.selectProfile(RECORD);
      result.current.setScope("workspace");
    });

    await act(async () => {
      await result.current.save.mutateAsync();
    });

    expect(apiMocks.putProfile).toHaveBeenCalledWith(
      expect.objectContaining({ id: "two-up-review" }),
      "workspace",
      "workspace-a",
      4
    );
    expect(
      queryClient.getQueryData(settingsKeys.windowManagerLayoutProfiles("workspace-a"))
    ).toEqual([workspaceRecord]);
  });
});
