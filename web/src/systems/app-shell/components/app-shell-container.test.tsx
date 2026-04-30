import { render, screen, waitFor } from "@testing-library/react";
import { useQuery } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createTestQueryClient, installFetchStub, matchPath, withQuery } from "@/test/utils";
import { WORKSPACE_STORAGE_KEY } from "@/lib/session-storage";

import { resetActiveWorkspaceStoreForTests } from "../stores/active-workspace-store";
import { useActiveWorkspaceStore } from "../stores/active-workspace-store";
import type { Workspace } from "../types";
import { AppShellContainer } from "./app-shell-container";
import { useActiveWorkspaceContext } from "../lib/active-workspace-context";

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

function ContextProbe(): ReactElement {
  const context = useActiveWorkspaceContext();
  return <p data-testid="probe-active">{context.activeWorkspace.id}</p>;
}

function StaleWorkspaceQueryProbe({
  workspaceId = "ws-1",
}: {
  workspaceId?: string;
}): ReactElement {
  useQuery({
    queryKey: ["stale-workspace-probe", workspaceId],
    queryFn: async (): Promise<string> => {
      throw { code: "workspace_context_stale", message: "stale" };
    },
  });
  return <ContextProbe />;
}

describe("AppShellContainer", () => {
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

  it("Should render the onboarding surface for zero workspaces", async () => {
    const stub = installFetchStub([
      { matcher: matchPath("/api/workspaces"), status: 200, body: { workspaces: [] } },
    ]);
    restore = stub.restore;
    render(
      <AppShellContainer>
        <ContextProbe />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );
    expect(await screen.findByTestId("workspace-onboarding")).toBeInTheDocument();
  });

  it("Should render children when exactly one workspace resolves", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
    ]);
    restore = stub.restore;
    render(
      <AppShellContainer>
        <ContextProbe />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );
    expect(await screen.findByTestId("probe-active")).toHaveTextContent("ws-1");
  });

  it("Should render the workspace picker when many workspaces exist", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;
    render(
      <AppShellContainer>
        <ContextProbe />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );
    expect(await screen.findByTestId("workspace-picker-list")).toBeInTheDocument();
    await userEvent.click(await screen.findByTestId("workspace-picker-select-ws-2"));
    await waitFor(() => expect(screen.getByTestId("probe-active")).toHaveTextContent("ws-2"));
  });

  it("Should display the error boundary when the workspaces request fails", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 500,
        body: { code: "server_error", message: "down", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    render(
      <AppShellContainer>
        <ContextProbe />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );
    expect(await screen.findByTestId("app-shell-error")).toBeInTheDocument();
  });

  it("Should clear the active workspace when a child query reports stale workspace context", async () => {
    window.sessionStorage.setItem(WORKSPACE_STORAGE_KEY, "ws-1");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-1" });
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;

    render(
      <AppShellContainer>
        <StaleWorkspaceQueryProbe />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );

    expect(await screen.findByTestId("workspace-picker-stale")).toBeInTheDocument();
    expect(screen.queryByTestId("probe-active")).not.toBeInTheDocument();
    expect(window.sessionStorage.getItem(WORKSPACE_STORAGE_KEY)).toBeNull();
  });

  it("Should ignore stale workspace errors from an inactive workspace query", async () => {
    window.sessionStorage.setItem(WORKSPACE_STORAGE_KEY, "ws-2");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-2" });
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne, workspaceTwo] },
      },
    ]);
    restore = stub.restore;

    render(
      <AppShellContainer>
        <StaleWorkspaceQueryProbe workspaceId="ws-1" />
      </AppShellContainer>,
      { wrapper: withQuery(createTestQueryClient()) }
    );

    expect(await screen.findByTestId("probe-active")).toHaveTextContent("ws-2");
    expect(screen.queryByTestId("workspace-picker-stale")).not.toBeInTheDocument();
    expect(window.sessionStorage.getItem(WORKSPACE_STORAGE_KEY)).toBe("ws-2");
  });
});
