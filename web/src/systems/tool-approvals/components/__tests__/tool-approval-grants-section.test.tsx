import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { delay, http, HttpResponse } from "msw";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch, createStatefulMswStore } from "@/test/msw-fetch";
import { toolApprovalGrantFixtures } from "@/systems/tool-approvals/mocks/fixtures";
import { ToolApprovalGrantsSection } from "@/systems/tool-approvals";

const WS = "ws_default";
const TEST_ID = "settings-page-general-tool-approvals";
const allowGrant = toolApprovalGrantFixtures[0]!;
const rejectGrant = toolApprovalGrantFixtures[1]!;

const workspaceMock = vi.hoisted(() => ({
  value: { activeWorkspaceId: "ws_default" as string | null, hasHydrated: true, isLoading: false },
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => workspaceMock.value,
}));

function listHandler(grants: ReadonlyArray<(typeof toolApprovalGrantFixtures)[number]>) {
  return http.get("/api/tool-approval-grants", () =>
    HttpResponse.json({ grants, total: grants.length })
  );
}

function stubFetch(handlers: ReturnType<typeof http.get>[]) {
  vi.stubGlobal(
    "fetch",
    createMswFetch(() => handlers)
  );
}

function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return render(<ToolApprovalGrantsSection />, { wrapper });
}

describe("ToolApprovalGrantsSection", () => {
  beforeEach(() => {
    workspaceMock.value = { activeWorkspaceId: WS, hasHydrated: true, isLoading: false };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should render each remembered decision with its truthful scope and last-used time", async () => {
    stubFetch([listHandler(toolApprovalGrantFixtures)]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-list`)).toBeInTheDocument());

    expect(screen.getByText(allowGrant.tool_id)).toBeInTheDocument();
    expect(screen.getByText(rejectGrant.tool_id)).toBeInTheDocument();
    expect(screen.getByTestId(`tool-approval-grant-decision-${allowGrant.id}`)).toHaveTextContent(
      "allow"
    );
    expect(screen.getByTestId(`tool-approval-grant-decision-${rejectGrant.id}`)).toHaveTextContent(
      "reject"
    );
    // The daemon reports each distinct matching scope truthfully.
    expect(screen.getByText("claude-code")).toBeInTheDocument();
    expect(screen.getByText("openclaw")).toBeInTheDocument();
    expect(screen.getByText("agent-wide")).toBeInTheDocument();
    expect(screen.getByText("tool-wide")).toBeInTheDocument();
    expect(screen.getByText("exact input")).toBeInTheDocument();
    expect(
      screen.getByTestId(`tool-approval-grant-last-used-${allowGrant.id}`)
    ).toBeInTheDocument();
  });

  it("Should show a workspace-scoped empty state when there are no decisions", async () => {
    stubFetch([listHandler([])]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-empty`)).toBeInTheDocument());
    expect(screen.getByTestId(`${TEST_ID}-empty`)).toHaveTextContent(
      /no remembered decisions yet/i
    );
  });

  it("Should surface an error with a retry affordance when the list fails", async () => {
    stubFetch([
      http.get("/api/tool-approval-grants", () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      ),
    ]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-error`)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("Should show the loading state while the list resolves", async () => {
    stubFetch([
      http.get("/api/tool-approval-grants", async () => {
        await delay("infinite");
        return HttpResponse.json({ grants: [], total: 0 });
      }),
    ]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-loading`)).toBeInTheDocument());
  });

  it("Should render the empty state without fetching when there is no active workspace", async () => {
    workspaceMock.value = { activeWorkspaceId: null, hasHydrated: true, isLoading: false };
    stubFetch([listHandler(toolApprovalGrantFixtures)]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-empty`)).toBeInTheDocument());
    expect(screen.getByTestId(`${TEST_ID}-set-open`)).toBeDisabled();
  });

  it("Should set an explicit agent-wide decision and render daemon truth", async () => {
    const store = createStatefulMswStore(toolApprovalGrantFixtures.slice(0, 0));
    const widerGrant = {
      ...allowGrant,
      id: "set-agent-wide",
      workspace_id: WS,
      agent_name: "claude-code",
      tool_id: "agh__workspace_list",
      decision: "allow" as const,
      input_digest: undefined,
    };
    let requestBody: unknown;
    stubFetch([
      http.get("/api/tool-approval-grants", () =>
        HttpResponse.json({ grants: store.all(), total: store.all().length })
      ),
      http.put("/api/tool-approval-grants", async ({ request }) => {
        requestBody = await request.json();
        store.prepend(widerGrant);
        return HttpResponse.json({ grant: widerGrant });
      }),
    ]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-empty`)).toBeInTheDocument());
    fireEvent.click(screen.getByTestId(`${TEST_ID}-set-open`));
    expect(await screen.findByTestId("tool-approval-grant-set-confirm")).toBeDisabled();

    fireEvent.click(screen.getByTestId("tool-approval-grant-scope-agent"));
    fireEvent.change(screen.getByTestId("tool-approval-grant-tool-id"), {
      target: { value: widerGrant.tool_id },
    });
    fireEvent.change(screen.getByTestId("tool-approval-grant-agent-name"), {
      target: { value: widerGrant.agent_name },
    });
    fireEvent.click(screen.getByTestId("tool-approval-grant-decision-allow"));
    fireEvent.click(screen.getByTestId("tool-approval-grant-set-confirm"));

    await waitFor(() =>
      expect(screen.getByTestId(`tool-approval-grant-scope-${widerGrant.id}`)).toHaveTextContent(
        "agent-wide"
      )
    );
    expect(requestBody).toEqual({
      tool_id: widerGrant.tool_id,
      decision: "allow",
      scope: "agent",
      agent_name: widerGrant.agent_name,
    });
    expect(screen.queryByTestId("tool-approval-grant-set-dialog")).not.toBeInTheDocument();
  });

  it("Should keep the wider-decision dialog open when set fails", async () => {
    stubFetch([
      listHandler([]),
      http.put("/api/tool-approval-grants", () =>
        HttpResponse.json({ error: "set exploded" }, { status: 500 })
      ),
    ]);
    renderSection();

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-empty`)).toBeInTheDocument());
    fireEvent.click(screen.getByTestId(`${TEST_ID}-set-open`));
    fireEvent.click(screen.getByTestId("tool-approval-grant-scope-tool"));
    fireEvent.change(screen.getByTestId("tool-approval-grant-tool-id"), {
      target: { value: "agh__task_create" },
    });
    fireEvent.click(screen.getByTestId("tool-approval-grant-decision-reject"));
    fireEvent.click(screen.getByTestId("tool-approval-grant-set-confirm"));

    await waitFor(() =>
      expect(screen.getByTestId("tool-approval-grant-set-error")).toHaveTextContent("set exploded")
    );
    expect(screen.getByTestId("tool-approval-grant-set-dialog")).toBeInTheDocument();
  });

  it("Should revoke a decision after confirmation and remove its row", async () => {
    const store = createStatefulMswStore(toolApprovalGrantFixtures);
    stubFetch([
      http.get("/api/tool-approval-grants", () =>
        HttpResponse.json({ grants: store.all(), total: store.all().length })
      ),
      http.delete("/api/tool-approval-grants/:id", ({ params }) =>
        store.delete(String(params.id))
          ? new HttpResponse(null, { status: 204 })
          : HttpResponse.json({ error: "not found" }, { status: 404 })
      ),
    ]);
    renderSection();

    await waitFor(() =>
      expect(
        screen.getByTestId(`tool-approval-grant-decision-${rejectGrant.id}`)
      ).toBeInTheDocument()
    );

    fireEvent.click(screen.getByTestId(`tool-approval-grant-revoke-${rejectGrant.id}`));
    fireEvent.click(await screen.findByTestId(`${TEST_ID}-revoke-confirm`));

    await waitFor(() =>
      expect(
        screen.queryByTestId(`tool-approval-grant-decision-${rejectGrant.id}`)
      ).not.toBeInTheDocument()
    );
    // The other decisions remain.
    expect(screen.getByTestId(`tool-approval-grant-decision-${allowGrant.id}`)).toBeInTheDocument();
  });

  it("Should keep the row and report failure when a revoke fails", async () => {
    stubFetch([
      listHandler(toolApprovalGrantFixtures),
      http.delete("/api/tool-approval-grants/:id", () =>
        HttpResponse.json({ error: "revoke exploded" }, { status: 500 })
      ),
    ]);
    renderSection();

    await waitFor(() =>
      expect(screen.getByTestId(`tool-approval-grant-revoke-${allowGrant.id}`)).toBeInTheDocument()
    );

    fireEvent.click(screen.getByTestId(`tool-approval-grant-revoke-${allowGrant.id}`));
    fireEvent.click(await screen.findByTestId(`${TEST_ID}-revoke-confirm`));

    await waitFor(() =>
      expect(screen.getByTestId(`${TEST_ID}-revoke-error`)).toHaveTextContent("revoke exploded")
    );
    expect(screen.getByTestId(`tool-approval-grant-decision-${allowGrant.id}`)).toBeInTheDocument();
  });

  it("Should disable confirm and cancel while a revoke is in flight", async () => {
    stubFetch([
      listHandler(toolApprovalGrantFixtures),
      http.delete("/api/tool-approval-grants/:id", async () => {
        await delay("infinite");
        return new HttpResponse(null, { status: 204 });
      }),
    ]);
    renderSection();

    await waitFor(() =>
      expect(screen.getByTestId(`tool-approval-grant-revoke-${allowGrant.id}`)).toBeInTheDocument()
    );

    fireEvent.click(screen.getByTestId(`tool-approval-grant-revoke-${allowGrant.id}`));
    fireEvent.click(await screen.findByTestId(`${TEST_ID}-revoke-confirm`));

    await waitFor(() => expect(screen.getByTestId(`${TEST_ID}-revoke-confirm`)).toBeDisabled());
    expect(screen.getByTestId(`${TEST_ID}-revoke-cancel`)).toBeDisabled();
  });
});
