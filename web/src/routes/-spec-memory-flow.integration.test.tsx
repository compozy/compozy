import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createTestQueryClient, installFetchStub, matchPath } from "@/test/utils";

import { routeTree } from "../routeTree.gen";
import { resetActiveWorkspaceStoreForTests } from "../systems/app-shell";

const workspaceOne = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const specPayload = {
  spec: {
    workspace: workspaceOne,
    workflow,
    prd: {
      id: "prd",
      kind: "prd",
      title: "PRD: alpha",
      updated_at: "2026-01-01T00:00:00Z",
      markdown: "# PRD body",
    },
    techspec: {
      id: "techspec",
      kind: "techspec",
      title: "TechSpec: alpha",
      updated_at: "2026-01-01T00:00:00Z",
      markdown: "# TechSpec body",
    },
    adrs: [
      {
        id: "adr-001",
        kind: "adr",
        title: "ADR-001: scope",
        updated_at: "2026-01-01T00:00:00Z",
        markdown: "ADR body",
      },
    ],
  },
};

const memoryIndexPayload = {
  memory: {
    workspace: workspaceOne,
    workflow,
    entries: [
      {
        file_id: "file-shared",
        display_path: ".compozy/memory/alpha/MEMORY.md",
        kind: "shared",
        size_bytes: 2048,
        title: "MEMORY.md",
        updated_at: "2026-01-02T00:00:00Z",
      },
      {
        file_id: "file-task-01",
        display_path: ".compozy/memory/alpha/task_01.md",
        kind: "task",
        size_bytes: 512,
        title: "task_01.md",
        updated_at: "2026-01-02T00:01:00Z",
      },
    ],
  },
};

const sharedMemoryFilePayload = {
  document: {
    id: "memory-file-shared",
    kind: "shared",
    title: "MEMORY.md",
    updated_at: "2026-01-02T00:00:00Z",
    markdown: "## Shared memory body",
  },
};

const taskMemoryFilePayload = {
  document: {
    id: "memory-file-task-01",
    kind: "task",
    title: "task_01.md",
    updated_at: "2026-01-02T00:01:00Z",
    markdown: "## Task 01 memory body",
  },
};

function matchUrl(pattern: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const requestMethod =
      input instanceof Request ? input.method.toUpperCase() : (init?.method ?? "GET").toUpperCase();
    return url.includes(pattern) && requestMethod === method.toUpperCase();
  };
}

async function renderApp(initialPath: string) {
  const queryClient = createTestQueryClient();
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    defaultPreload: false,
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  await router.load();
  return { router, queryClient };
}

describe("spec + memory flow integration", () => {
  let restore: (() => void) | null = null;

  beforeEach(async () => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
    const { useActiveWorkspaceStore } =
      await import("../systems/app-shell/stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-1" });
  });

  afterEach(() => {
    restore?.();
    restore = null;
    resetActiveWorkspaceStoreForTests();
    vi.clearAllMocks();
  });

  it("Should render PRD/TechSpec/ADR tabs from the typed daemon contract", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/alpha/spec"),
        status: 200,
        body: specPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/alpha/spec");
    await screen.findByTestId("workflow-spec-view");
    expect(screen.getByTestId("workflow-spec-prd-body")).toHaveTextContent("PRD body");
    await userEvent.click(screen.getByTestId("workflow-spec-tab-techspec"));
    expect(screen.getByTestId("workflow-spec-techspec-body")).toHaveTextContent("TechSpec body");
    await userEvent.click(screen.getByTestId("workflow-spec-tab-adrs"));
    expect(screen.getByTestId("workflow-spec-adr-adr-001")).toBeInTheDocument();
    const specCall = stub.calls.find(call => call.url.includes("/api/tasks/alpha/spec"));
    expect(specCall?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface a missing-document alert at the spec route", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks/ghost/spec"),
        status: 404,
        body: { code: "document_missing", message: "spec missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/workflows/ghost/spec");
    const alert = await screen.findByTestId("workflow-spec-load-error");
    expect(alert).toHaveTextContent("spec missing");
  });

  it("Should render the memory index from the workflows list", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/tasks", "GET"),
        status: 200,
        body: { workflows: [workflow] },
      },
    ]);
    restore = stub.restore;
    await renderApp("/memory");
    await screen.findByTestId("memory-index-view");
    await screen.findByTestId("memory-index-card-alpha");
    const link = screen.getByTestId("memory-index-open-alpha") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/memory/alpha");
  });

  it("Should render the workflow memory detail and load files by opaque file_id", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory"),
        status: 200,
        body: memoryIndexPayload,
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory/files/file-shared"),
        status: 200,
        body: sharedMemoryFilePayload,
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory/files/file-task-01"),
        status: 200,
        body: taskMemoryFilePayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/memory/alpha");
    await screen.findByTestId("workflow-memory-view");
    await waitFor(() => {
      expect(screen.getByTestId("workflow-memory-document-body")).toHaveTextContent(
        "Shared memory body"
      );
    });
    const sharedCall = stub.calls.find(call =>
      call.url.includes("/api/tasks/alpha/memory/files/file-shared")
    );
    expect(sharedCall?.url).not.toContain("MEMORY.md");
    expect(sharedCall?.headers["x-compozy-workspace-id"]).toBe("ws-1");

    await userEvent.click(screen.getByTestId("workflow-memory-entry-file-task-01"));
    await waitFor(() => {
      expect(screen.getByTestId("workflow-memory-document-body")).toHaveTextContent(
        "Task 01 memory body"
      );
    });
  });

  it("Should surface a stale-workspace alert when the memory index is stale", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory"),
        status: 412,
        body: {
          code: "workspace_context_stale",
          message: "workspace stale",
          request_id: "r",
        },
      },
    ]);
    restore = stub.restore;
    await renderApp("/memory/alpha");
    const alert = await screen.findByTestId("workflow-memory-load-error");
    expect(alert).toHaveTextContent("workspace stale");
  });

  it("Should recover when the selected memory file fails but the index is present", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory"),
        status: 200,
        body: memoryIndexPayload,
      },
      {
        matcher: matchPath("/api/tasks/alpha/memory/files/file-shared"),
        status: 404,
        body: { code: "memory_file_not_found", message: "file missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/memory/alpha");
    await screen.findByTestId("workflow-memory-view");
    const alert = await screen.findByTestId("workflow-memory-document-error");
    expect(alert).toHaveTextContent("file missing");
  });
});
