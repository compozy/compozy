import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createTestQueryClient, installFetchStub, matchPath, withQuery } from "@/test/utils";

import { resetActiveWorkspaceStoreForTests } from "../stores/active-workspace-store";
import type { Workspace } from "../types";
import { useActiveWorkspace } from "./use-active-workspace";

const workspaceOne: Workspace = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  filesystem_state: "present",
  read_only: false,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workspaceTwo: Workspace = {
  id: "ws-2",
  name: "two",
  root_dir: "/tmp/two",
  filesystem_state: "present",
  read_only: false,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("useActiveWorkspace", () => {
  let restore: (() => void) | null = null;

  beforeEach(() => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
  });

  afterEach(() => {
    restore?.();
    restore = null;
    resetActiveWorkspaceStoreForTests();
  });

  it("Should expose an empty status when the daemon has no workspaces", async () => {
    const stub = installFetchStub([
      { matcher: matchPath("/api/workspaces"), status: 200, body: { workspaces: [] } },
    ]);
    restore = stub.restore;
    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("empty"));
    expect(result.current.activeWorkspace).toBeNull();
  });

  it("Should auto-select and resolve when exactly one workspace exists", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
    ]);
    restore = stub.restore;
    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("resolved"));
    expect(result.current.activeWorkspaceId).toBe("ws-1");
    expect(window.sessionStorage.getItem("compozy.web.active-workspace")).toBe("ws-1");
  });

  it("Should require explicit selection when many workspaces exist", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;
    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("many"));
    expect(result.current.activeWorkspace).toBeNull();

    act(() => {
      result.current.setActiveWorkspaceId("ws-2");
    });
    await waitFor(() => expect(result.current.status).toBe("resolved"));
    expect(result.current.activeWorkspaceId).toBe("ws-2");
  });

  it("Should clear a stale selection and fall back to the selection surface", async () => {
    window.sessionStorage.setItem("compozy.web.active-workspace", "ws-gone");
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.setItem("compozy.web.active-workspace", "ws-gone");
    const { useActiveWorkspaceStore } = await import("../stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-gone" });

    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("many"));
    expect(result.current.selectedWorkspaceId).toBeNull();
    expect(window.sessionStorage.getItem("compozy.web.active-workspace")).toBeNull();
  });

  it("Should keep a selected read-only workspace when its path is missing", async () => {
    const missingWorkspace: Workspace = {
      ...workspaceOne,
      filesystem_state: "missing",
      read_only: true,
      root_dir: "/tmp/missing",
    };
    window.sessionStorage.setItem("compozy.web.active-workspace", missingWorkspace.id);
    const { useActiveWorkspaceStore } = await import("../stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: missingWorkspace.id });

    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [missingWorkspace, workspaceTwo] },
      },
    ]);
    restore = stub.restore;

    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("resolved"));
    expect(result.current.isStaleSelection).toBe(false);
    expect(result.current.activeWorkspace?.read_only).toBe(true);
    expect(window.sessionStorage.getItem("compozy.web.active-workspace")).toBe(missingWorkspace.id);
  });

  it("Should expose an error status when the daemon request fails", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 500,
        body: { code: "server_error", message: "down", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    const { result } = renderHook(() => useActiveWorkspace(), {
      wrapper: withQuery(createTestQueryClient()),
    });
    await waitFor(() => expect(result.current.status).toBe("error"));
    expect(result.current.activeWorkspace).toBeNull();
  });
});
