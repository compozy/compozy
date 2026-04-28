import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import type { RenderResult } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createContext, useContext, type ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { WorkflowSpecView } from "./workflow-spec-view";
import type { WorkflowSpecDocument } from "../types";

const workspace = {
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
} as const;

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const fullSpec: WorkflowSpecDocument = {
  workspace,
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
};

interface SpecTestContextValue {
  isRefreshing: boolean;
  spec: WorkflowSpecDocument;
}

const SpecTestContext = createContext<SpecTestContextValue | null>(null);

async function renderSpec(spec: WorkflowSpecDocument, isRefreshing: boolean = false) {
  let currentSpec = spec;
  let currentRefreshing = isRefreshing;
  const rootRoute = createRootRoute();
  const specRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function SpecRoute(): ReactElement {
      const value = useContext(SpecTestContext);
      if (!value) {
        throw new Error("expected spec test context");
      }
      return <WorkflowSpecView isRefreshing={value.isRefreshing} spec={value.spec} />;
    },
  });
  const workflowsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows",
    component: function WorkflowsStub(): ReactElement {
      return <div data-testid="workflows-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([specRoute, workflowsRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  let renderResult: RenderResult | null = null;
  await act(async () => {
    renderResult = render(
      <SpecTestContext.Provider value={{ isRefreshing: currentRefreshing, spec: currentSpec }}>
        <RouterProvider router={router} />
      </SpecTestContext.Provider>
    );
    await Promise.resolve();
  });
  return {
    rerender(nextSpec: WorkflowSpecDocument, nextRefreshing: boolean = currentRefreshing) {
      if (renderResult === null) {
        throw new Error("expected render result to be available");
      }
      const mounted = renderResult;
      currentSpec = nextSpec;
      currentRefreshing = nextRefreshing;
      act(() => {
        mounted.rerender(
          <SpecTestContext.Provider value={{ isRefreshing: currentRefreshing, spec: currentSpec }}>
            <RouterProvider router={router} />
          </SpecTestContext.Provider>
        );
      });
    },
  };
}

describe("WorkflowSpecView", () => {
  it("Should render the PRD body by default", async () => {
    await renderSpec(fullSpec);
    expect(screen.getByTestId("workflow-spec-view")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-spec-prd-body")).toHaveTextContent("PRD body");
  });

  it("Should switch to the TechSpec tab", async () => {
    await renderSpec(fullSpec);
    await userEvent.click(screen.getByTestId("workflow-spec-tab-techspec"));
    expect(screen.getByTestId("workflow-spec-techspec-body")).toHaveTextContent("TechSpec body");
  });

  it("Should render the ADR list in the ADR tab", async () => {
    await renderSpec(fullSpec);
    await userEvent.click(screen.getByTestId("workflow-spec-tab-adrs"));
    expect(screen.getByTestId("workflow-spec-adrs-list")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-spec-adr-adr-001")).toBeInTheDocument();
  });

  it("Should render the empty ADR state when no ADRs exist", async () => {
    await renderSpec({ ...fullSpec, adrs: [] });
    await userEvent.click(screen.getByTestId("workflow-spec-tab-adrs"));
    expect(screen.getByTestId("workflow-spec-adrs-empty")).toBeInTheDocument();
  });

  it("Should disable tabs for missing documents and fall back to the first present tab", async () => {
    await renderSpec({ ...fullSpec, prd: undefined });
    const prdTab = screen.getByTestId("workflow-spec-tab-prd") as HTMLButtonElement;
    expect(prdTab.disabled).toBe(true);
    expect(screen.getByTestId("workflow-spec-techspec-body")).toHaveTextContent("TechSpec body");
  });

  it("Should render the PRD missing state when only ADRs are present", async () => {
    await renderSpec({ ...fullSpec, prd: undefined, techspec: undefined });
    expect(screen.getByTestId("workflow-spec-adrs-list")).toBeInTheDocument();
    expect(screen.queryByTestId("workflow-spec-prd-missing")).not.toBeInTheDocument();
  });

  it("Should render the refreshing indicator", async () => {
    await renderSpec(fullSpec, true);
    expect(screen.getByTestId("workflow-spec-refreshing")).toBeInTheDocument();
  });

  it("Should reset the active tab when the workflow changes", async () => {
    const view = await renderSpec(fullSpec);
    await userEvent.click(screen.getByTestId("workflow-spec-tab-adrs"));
    expect(screen.getByTestId("workflow-spec-adrs-list")).toBeInTheDocument();

    view.rerender({
      ...fullSpec,
      workflow: { ...workflow, id: "wf-2", slug: "beta" },
      prd: {
        id: "prd-beta",
        kind: "prd",
        title: "PRD: beta",
        updated_at: "2026-01-01T00:00:00Z",
        markdown: "# Beta PRD body",
      },
      techspec: undefined,
      adrs: [],
    });

    expect(screen.getByTestId("workflow-spec-prd-body")).toHaveTextContent("Beta PRD body");
  });

  it("Should fall back to the first present tab when the active document disappears", async () => {
    const view = await renderSpec(fullSpec);
    await userEvent.click(screen.getByTestId("workflow-spec-tab-techspec"));
    expect(screen.getByTestId("workflow-spec-techspec-body")).toHaveTextContent("TechSpec body");

    view.rerender({
      ...fullSpec,
      techspec: undefined,
    });

    expect(screen.getByTestId("workflow-spec-prd-body")).toHaveTextContent("PRD body");
  });
});
