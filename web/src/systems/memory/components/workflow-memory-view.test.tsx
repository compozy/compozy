import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { WorkflowMemoryView } from "./workflow-memory-view";
import type { MarkdownDocument, WorkflowMemoryIndex } from "../types";

const workspace = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const populatedIndex: WorkflowMemoryIndex = {
  workspace,
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
};

const document: MarkdownDocument = {
  id: "memory-file-shared",
  kind: "shared",
  title: "MEMORY.md",
  updated_at: "2026-01-02T00:00:00Z",
  markdown: "## Shared memory body",
};

interface RenderProps {
  index?: WorkflowMemoryIndex;
  selectedFileId?: string | null;
  selectedDocument?: MarkdownDocument | undefined;
  onSelectFileId?: (fileId: string) => void;
  isDocumentLoading?: boolean;
  isDocumentRefreshing?: boolean;
  documentError?: string | null;
}

async function renderView(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const viewRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function ViewRoute(): ReactElement {
      return (
        <WorkflowMemoryView
          documentError={props.documentError ?? null}
          index={props.index ?? populatedIndex}
          isDocumentLoading={props.isDocumentLoading ?? false}
          isDocumentRefreshing={props.isDocumentRefreshing ?? false}
          onSelectFileId={props.onSelectFileId ?? (() => {})}
          selectedDocument={props.selectedDocument}
          selectedFileId={props.selectedFileId ?? null}
        />
      );
    },
  });
  const memoryRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/memory",
    component: function MemoryStub(): ReactElement {
      return <div data-testid="memory-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([viewRoute, memoryRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("WorkflowMemoryView", () => {
  it("Should render entries keyed by opaque file_id and selection callback uses the opaque id", async () => {
    const onSelectFileId = vi.fn();
    await renderView({
      selectedFileId: "file-shared",
      selectedDocument: document,
      onSelectFileId,
    });
    expect(screen.getByTestId("workflow-memory-entry-file-shared")).toBeInTheDocument();
    const taskEntry = screen.getByTestId("workflow-memory-entry-file-task-01");
    await userEvent.click(taskEntry);
    expect(onSelectFileId).toHaveBeenCalledWith("file-task-01");
  });

  it("Should render the selected document body", async () => {
    await renderView({ selectedFileId: "file-shared", selectedDocument: document });
    expect(screen.getByTestId("workflow-memory-document-body")).toHaveTextContent(
      "Shared memory body"
    );
  });

  it("Should render a placeholder when no file is selected", async () => {
    await renderView({ selectedFileId: null });
    expect(screen.getByTestId("workflow-memory-document-placeholder")).toBeInTheDocument();
  });

  it("Should render the document loading state", async () => {
    await renderView({
      selectedFileId: "file-shared",
      selectedDocument: undefined,
      isDocumentLoading: true,
    });
    expect(screen.getByTestId("workflow-memory-document-loading")).toBeInTheDocument();
  });

  it("Should render the document error state", async () => {
    await renderView({
      selectedFileId: "file-shared",
      selectedDocument: undefined,
      documentError: "stale workspace",
    });
    expect(screen.getByTestId("workflow-memory-document-error")).toHaveTextContent(
      "stale workspace"
    );
  });

  it("Should render the empty-index card when no entries exist", async () => {
    await renderView({
      index: { workspace, workflow, entries: [] },
      selectedFileId: null,
    });
    expect(screen.getByTestId("workflow-memory-empty")).toBeInTheDocument();
  });

  it("Should render the shared and task groups in the sidebar", async () => {
    await renderView({ selectedFileId: "file-shared", selectedDocument: document });
    expect(screen.getByTestId("workflow-memory-group-shared")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-memory-group-task")).toBeInTheDocument();
  });
});
