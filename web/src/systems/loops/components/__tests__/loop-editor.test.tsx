import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { TopbarSlotValue } from "@compozy/ui";

import { createMswFetch } from "@/test/msw-fetch";
import { renderWithTopbar } from "@/test/render-with-topbar";
import { LoopEditor } from "../editor/loop-editor";
import { LoopEditorPublishRejectedStrip } from "../editor/loop-editor-publish-rejected-strip";
import type { LoopDetail } from "../../types";
import { handlers } from "../../mocks";
import {
  PUBLISH_REJECTED_ISSUES,
  fullLifecycleDetail,
  readOnlySourceDetail,
  waitWarningDetail,
} from "../../mocks/fixture-editor-lifecycle";
import { loopDetailByName } from "../../mocks/fixtures";

const deliveryDetail = loopDetailByName.get("software-delivery")!;

const WS = "ws_default";

/** Serves one detail fixture while keeping the real linter/publish handlers behind it. */
function detailHandler(detail: LoopDetail) {
  return http.get("/api/workspaces/:workspaceId/loops/:name", () =>
    HttpResponse.json({ loop: detail })
  );
}

type PatchedDefinition = {
  graph: { nodes: Record<string, unknown>[] };
  contract: Record<string, unknown>;
};

/** Captures the published definition so a round-trip can be asserted on the real PATCH body. */
function capturePublish(response?: LoopDetail) {
  const captured: { definition: PatchedDefinition | null; expectedVersion?: number | null } = {
    definition: null,
  };
  const handler = http.patch("/api/workspaces/:workspaceId/loops/:name", async ({ request }) => {
    const body = (await request.json()) as {
      definition: PatchedDefinition;
      expected_version?: number | null;
    };
    captured.definition = body.definition;
    captured.expectedVersion = body.expected_version;
    return HttpResponse.json({ loop: response ?? deliveryDetail });
  });
  return { captured, handler };
}

function publishedNode(captured: { definition: PatchedDefinition | null }, id: string) {
  const node = captured.definition?.graph.nodes.find(entry => entry.id === id);
  if (!node) throw new Error(`published node ${id} not found`);
  return node;
}

function renderEditor(
  name = "software-delivery",
  extraHandlers: ReturnType<typeof http.post>[] = [],
  topbarIdentity?: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack">
) {
  vi.stubGlobal(
    "fetch",
    createMswFetch(() => [...extraHandlers, ...handlers])
  );
  const onPublished = vi.fn<(loop: LoopDetail) => void>();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithTopbar(
    <QueryClientProvider client={queryClient}>
      <LoopEditor
        workspaceId={WS}
        name={name}
        onPublished={onPublished}
        topbarIdentity={topbarIdentity}
      />
    </QueryClientProvider>
  );
  return { onPublished };
}

function nodeCard(id: string): HTMLElement {
  const cards = screen.getAllByTestId("loop-editor-node");
  const card = cards.find(el => el.getAttribute("data-node-id") === id);
  if (!card) throw new Error(`node card ${id} not found`);
  return card;
}

function expandLinterDock() {
  const toggle = screen.getByTestId("loop-linter-toggle");
  if (toggle.getAttribute("aria-expanded") === "false") {
    fireEvent.click(toggle);
  }
}

describe("LoopEditor", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("Should preserve route identity and editor actions in one topbar publication", async () => {
    const onBack = vi.fn();
    const onOpenLoops = vi.fn();
    const onOpenLoop = vi.fn();
    renderEditor("software-delivery", [], {
      onBack,
      crumbs: [
        { id: "loops", label: "Loops", onSelect: onOpenLoops },
        { id: "loop", label: "software-delivery", onSelect: onOpenLoop },
      ],
      crumb: "Editor",
    });

    await screen.findByTestId("loop-editor");
    expect(screen.getByRole("button", { name: "Back one level" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Loops" })).toBeInTheDocument();
    expect(screen.getByText("Editor")).toBeInTheDocument();
    expect(screen.getByTestId("loop-editor-validate")).toBeInTheDocument();
    expect(screen.getByTestId("loop-editor-publish")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Back one level" }));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("Should open the Contract lane by default and switch to Node when a canvas node is selected", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));

    expect(screen.getByTestId("loop-editor-tab-contract")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("loop-editor-contract")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-editor-inspector")).not.toBeInTheDocument();

    fireEvent.click(nodeCard("implement"));
    await waitFor(() =>
      expect(screen.getByTestId("loop-editor-tab-node")).toHaveAttribute("aria-selected", "true")
    );
    expect(
      within(screen.getByTestId("loop-editor-inspector")).getByTestId("loop-inspector-name")
    ).toHaveTextContent("implement");
  });

  it("E2E-web-12: edits a workspace Loop draft — palette add, inspector swap", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    // The canonical body renders on the canvas.
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));
    expect(screen.getByTestId("loop-editor-save")).toBeInTheDocument();
    // No unsaved-changes chip until the definition is edited.
    expect(screen.queryByTestId("loop-editor-dirty-chip")).not.toBeInTheDocument();
    // Adding a node from the palette extends the body, selects it, opens Node lane, and marks dirty.
    fireEvent.click(screen.getByTestId("loop-palette-item-run-agent"));
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(9));
    expect(screen.getByTestId("loop-editor-dirty-chip")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        within(screen.getByTestId("loop-editor-inspector")).getByTestId("loop-inspector-name")
      ).toHaveTextContent("run_agent")
    );
  });

  it("Should preserve both nodes when palette additions are dispatched in one event turn", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));

    fireEvent.click(screen.getByTestId("loop-palette-item-run-agent"));
    fireEvent.click(screen.getByTestId("loop-palette-item-run-agent"));

    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(10));
    expect(nodeCard("run_agent")).toBeInTheDocument();
    expect(nodeCard("run_agent_2")).toBeInTheDocument();
  });

  it("E2E-web-12: swaps the inspector field set when a different node is selected", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));
    fireEvent.click(nodeCard("implement"));
    await waitFor(() =>
      expect(
        within(screen.getByTestId("loop-editor-inspector")).getByTestId("loop-inspector-name")
      ).toHaveTextContent("implement")
    );
    // The fan-out node exposes the max_fan_out knob; a gate node does not.
    expect(screen.getByTestId("loop-field-max_fan_out")).toBeInTheDocument();
    fireEvent.click(nodeCard("review"));
    await waitFor(() =>
      expect(screen.queryByTestId("loop-field-max_fan_out")).not.toBeInTheDocument()
    );
    expect(screen.getByTestId("loop-editor-criteria")).toBeInTheDocument();
  });

  it("E2E-web-13: raising fan-out past the ceiling surfaces a 422 per-node error and disables Publish", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));
    // Publish is enabled on a clean workspace-sourced Loop.
    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());
    fireEvent.click(nodeCard("implement"));
    const fanOut = await screen.findByTestId("loop-field-max_fan_out");
    fireEvent.change(fanOut, { target: { value: "80" } });
    // The daemon linter (mock) returns fan_out_ceiling_exceeded → issue + node badge + gate.
    await waitFor(() =>
      expect(screen.getByTestId("loop-linter-error-count")).toHaveTextContent("1 error")
    );
    expandLinterDock();
    expect(screen.getByTestId("loop-linter-issue")).toHaveTextContent(/ceiling of 64/i);
    expect(nodeCard("implement")).toHaveAttribute("data-node-error", "true");
    expect(screen.getByTestId("loop-editor-publish")).toBeDisabled();
    // Lowering it back under the ceiling clears the issue and re-enables Publish. A clean
    // verdict renders NO counter at all (US-028 AC-4) — not "0 issues".
    fireEvent.change(fanOut, { target: { value: "32" } });
    await waitFor(() =>
      expect(screen.queryByTestId("loop-linter-error-count")).not.toBeInTheDocument()
    );
    expect(screen.queryByTestId("loop-linter-warning-count")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-linter-issue")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled();
    expect(nodeCard("implement")).toHaveAttribute("data-node-error", "false");
  });

  it("E2E-web-13: maps a cycle 422 onto the offending node and reveals it from the dock", async () => {
    const cycleHandler = http.post("/api/workspaces/:workspaceId/loops/:name/validate", () =>
      HttpResponse.json(
        {
          valid: false,
          errors: [
            {
              node_id: "review",
              code: "cycle_detected",
              message: "review is part of a cycle.",
              severity: "error",
            },
          ],
        },
        { status: 422 }
      )
    );
    renderEditor("software-delivery", [cycleHandler]);
    await screen.findByTestId("loop-editor");
    fireEvent.click(screen.getByTestId("loop-editor-validate"));
    await waitFor(() => expect(nodeCard("review")).toHaveAttribute("data-node-error", "true"));
    expect(screen.getByTestId("loop-editor-publish")).toBeDisabled();
    // Reveal node selects it in the inspector.
    expandLinterDock();
    fireEvent.click(screen.getByTestId("loop-linter-reveal"));
    await waitFor(() =>
      expect(
        within(screen.getByTestId("loop-editor-inspector")).getByTestId("loop-inspector-name")
      ).toHaveTextContent("review")
    );
  });

  it("E2E-web-13: maps a publish (PATCH) 422 per-node error back onto nodes, not just a banner", async () => {
    // A publish 422 must badge its own offending nodes (task-22 MUST), like validate does.
    const reject = http.patch("/api/workspaces/:workspaceId/loops/:name", () =>
      HttpResponse.json(
        {
          valid: false,
          errors: [
            {
              node_id: "verify",
              code: "verdict_policy_requires_judge",
              message: "verify: fixed_passes gate needs a judge or human criterion.",
              severity: "error",
            },
          ],
        },
        { status: 422 }
      )
    );
    renderEditor("software-delivery", [reject]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("loop-editor-publish"));
    // The rejected node is badged from the publish response itself.
    await waitFor(() => expect(nodeCard("verify")).toHaveAttribute("data-node-error", "true"));
    expect(screen.getByTestId("loop-editor-publish-error")).toHaveTextContent(
      /1 issue to resolve/i
    );
    expect(screen.getByTestId("loop-editor-publish-error")).toHaveAttribute("role", "alert");
    expandLinterDock();
    expect(screen.getByTestId("loop-linter-issue")).toHaveTextContent(/needs a judge/i);
    // The publish verdict now gates further publishes (an unattributed code fails safe).
    expect(screen.getByTestId("loop-editor-publish")).toBeDisabled();
  });

  it("Should keep the id input focused while renaming a node character-by-character", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));
    fireEvent.click(nodeCard("collect"));
    const idInput = (await screen.findByTestId("loop-field-id")) as HTMLInputElement;
    idInput.focus();
    fireEvent.change(idInput, { target: { value: "collect2" } });
    // The node is renamed everywhere...
    await waitFor(() =>
      expect(
        screen
          .getAllByTestId("loop-editor-node")
          .some(el => el.getAttribute("data-node-id") === "collect2")
      ).toBe(true)
    );
    // ...and the id input did not remount (focus survives), so multi-char renames are usable.
    expect(screen.getByTestId("loop-field-id")).toHaveFocus();
  });

  it("Should show an 'unavailable' dock state (not a stale pass) when validate can't reach the daemon", async () => {
    const downValidate = http.post("/api/workspaces/:workspaceId/loops/:name/validate", () =>
      HttpResponse.json({ error: "linter offline" }, { status: 500 })
    );
    renderEditor("software-delivery", [downValidate]);
    await screen.findByTestId("loop-editor");
    // The mount auto-validate fails → the dock reports unavailable instead of "all pass".
    await waitFor(() =>
      expect(screen.getByTestId("loop-linter-count")).toHaveTextContent("unavailable")
    );
    expandLinterDock();
    expect(screen.getByTestId("loop-linter-unavailable")).toBeInTheDocument();
    // Header stays truthful — unavailable, never a claimed pass.
    expect(screen.getByTestId("loop-linter-count")).toHaveTextContent("unavailable");
  });

  it("E2E-web-14: Graph/DSL toggle renders compozy.loop/v1 and highlights the offending field", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));
    // Raise the fan-out to produce an offending field, then flip to the DSL view.
    fireEvent.click(nodeCard("implement"));
    fireEvent.change(await screen.findByTestId("loop-field-max_fan_out"), {
      target: { value: "80" },
    });
    await waitFor(() =>
      expect(screen.getByTestId("loop-linter-error-count")).toHaveTextContent("1 error")
    );
    fireEvent.click(screen.getByTestId("loop-editor-view-dsl"));
    expect(screen.getByTestId("loop-editor-view-dsl")).toHaveAttribute("aria-pressed", "true");
    const dsl = await screen.findByTestId("loop-editor-dsl");
    expect(dsl).toHaveTextContent("compozy.loop/v1");
    const offending = dsl.querySelector('[data-offending="true"]');
    expect(offending?.textContent).toMatch(/max_fan_out/);
  });

  it("E2E-web-14: renders the read-only Start summary chips from the definition start[]", async () => {
    renderEditor();
    await screen.findByTestId("loop-editor");
    const summary = await screen.findByTestId("loop-editor-start-summary");
    expect(summary).toHaveTextContent("manual");
    expect(summary).toHaveTextContent("schedule");
  });

  it("E2E-web-15/16: publishes the fork (expected_version), bumps the version, and continues to run", async () => {
    let patchBody: { expected_version?: number | null } | null = null;
    const capture = http.patch("/api/workspaces/:workspaceId/loops/:name", async ({ request }) => {
      patchBody = (await request.json()) as { expected_version?: number | null };
      return HttpResponse.json({
        loop: {
          ...deliveryDetail,
          version: 5,
          definition: {
            ...deliveryDetail.definition,
            meta: { name: "software-delivery", version: 5, catalog: {} },
          },
        },
      });
    });
    const { onPublished } = renderEditor("software-delivery", [capture]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v4"));
    fireEvent.click(screen.getByTestId("loop-palette-item-collect"));
    fireEvent.click(screen.getByTestId("loop-editor-publish"));
    await waitFor(() => expect(onPublished).toHaveBeenCalledTimes(1));
    expect(patchBody).toEqual(expect.objectContaining({ expected_version: 4 }));
    expect(onPublished.mock.calls[0][0].version).toBe(5);
    // The published version replaces the draft version and clears the dirty chip.
    await waitFor(() => expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v5"));
    expect(screen.queryByTestId("loop-editor-dirty-chip")).not.toBeInTheDocument();
  });

  it("Should publish contract edits including an intentionally cleared goal with CAS", async () => {
    let patchBody: {
      definition?: { contract?: { goal?: string; definition_of_done?: string } };
      expected_version?: number | null;
    } | null = null;
    const capture = http.patch("/api/workspaces/:workspaceId/loops/:name", async ({ request }) => {
      patchBody = (await request.json()) as typeof patchBody;
      return HttpResponse.json({ loop: deliveryDetail });
    });
    renderEditor("software-delivery", [capture]);

    await screen.findByTestId("loop-editor");
    fireEvent.change(screen.getByRole("textbox", { name: "Goal (optional)" }), {
      target: { value: "" },
    });
    fireEvent.change(screen.getByRole("textbox", { name: "Definition of done" }), {
      target: { value: "The release evidence is recorded." },
    });
    expect(screen.getByTestId("loop-editor-dirty-chip")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-editor-publish"));

    await waitFor(() => expect(patchBody).not.toBeNull());
    expect(patchBody).toEqual(
      expect.objectContaining({
        expected_version: 4,
        definition: expect.objectContaining({
          contract: expect.objectContaining({
            goal: "",
            definition_of_done: "The release evidence is recorded.",
          }),
        }),
      })
    );
  });

  it("WT-005: carries the whole Spec 1 grammar through edit → PATCH → reopen unchanged", async () => {
    const { captured, handler } = capturePublish();
    renderEditor("software-delivery", [handler, detailHandler(fullLifecycleDetail)]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(10));

    // Reopen fidelity: the persisted envelope is what the inspector shows.
    fireEvent.click(nodeCard("execute_task"));
    const deadline = (await screen.findByTestId("loop-field-deadline")) as HTMLInputElement;
    expect(deadline.value).toBe("2h");
    expect((screen.getByTestId("loop-field-max_attempts") as HTMLInputElement).value).toBe("3");
    expect((screen.getByTestId("loop-field-backoff_base") as HTMLInputElement).value).toBe("30s");
    expect((screen.getByTestId("loop-field-result_failure_field") as HTMLInputElement).value).toBe(
      "err"
    );
    expect(screen.getByTestId("loop-field-on_error_allow_fail")).toBeChecked();

    // One real edit through the UI, then publish.
    fireEvent.change(deadline, { target: { value: "4h" } });
    await waitFor(() => expect(screen.getByTestId("loop-editor-dirty-chip")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("loop-editor-publish"));
    await waitFor(() => expect(captured.definition).not.toBeNull());

    const execute = publishedNode(captured, "execute_task");
    expect(execute).toMatchObject({
      deadline: "4h",
      retry: {
        max_attempts: 3,
        backoff: { base: "30s", max: "5m" },
        non_retryable: ["tool_invalid_input"],
      },
      result_contract: { failure_field: "err", message_field: "err.message" },
      on_error: {
        allow_fail: true,
        effects: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
      },
      on_retry: [{ emit: { kind: "task_retrying" } }],
      on_quarantine: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
    });
    // The retired `retry.max` key is never authored.
    expect(execute.retry).not.toHaveProperty("max");

    expect(publishedNode(captured, "await_deploy_ack").params).toMatchObject({
      event: { kind: "task_status_changed" },
      expect: { type: "object", required: ["deploy_id"] },
      ahead_arrival: "consume_on_entry",
      expires: { after: "24h", route: "release_notes" },
    });
    expect(publishedNode(captured, "release_notes")).toMatchObject({
      on_parent_close: "cancel",
    });
    expect(captured.definition?.contract).toMatchObject({
      on_failed: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
      on_canceled: [{ emit: { kind: "delivery_canceled" } }],
    });

    expect(execute).not.toHaveProperty("on_success");
    expect(execute).not.toHaveProperty("on_pause");
    expect(execute).not.toHaveProperty("on_timeout");
    expect(execute).not.toHaveProperty("on_cancel");
    expect(captured.definition?.contract).not.toHaveProperty("on_done");
    expect(captured.definition?.contract).not.toHaveProperty("on_noop");
  });

  it("WT-006: lets the daemon's named diagnostic gate Publish for on_error", async () => {
    renderEditor("software-delivery", [detailHandler(fullLifecycleDetail)]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());

    // Declaring a route ALONGSIDE the already-authored allow_fail is the EC-1 contradiction.
    fireEvent.click(nodeCard("execute_task"));
    fireEvent.change(await screen.findByTestId("loop-field-on_error_route"), {
      target: { value: "collect" },
    });

    // The gate is the daemon's verdict, surfaced by its own code — not a client-side rule.
    await waitFor(() => expect(screen.getByTestId("loop-linter-error-count")).toBeInTheDocument());
    expandLinterDock();
    expect(screen.getByTestId("loop-linter-dock")).toHaveTextContent("error_route_conflict");
    expect(screen.getByTestId("loop-editor-publish")).toBeDisabled();

    // Resolving it to a single absorption mode clears the gate.
    fireEvent.click(screen.getByTestId("loop-field-on_error_allow_fail"));
    await waitFor(() =>
      expect(screen.queryByTestId("loop-linter-error-count")).not.toBeInTheDocument()
    );
    expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled();
  });

  it("WT-006: builds each effect entry as emit XOR tool instead of validating it after the fact", async () => {
    const { captured, handler } = capturePublish();
    renderEditor("software-delivery", [handler, detailHandler(fullLifecycleDetail)]);
    await screen.findByTestId("loop-editor");
    fireEvent.click(nodeCard("execute_task"));

    // The authored on_retry row is an emit; switching its shape must REPLACE the variant.
    const shape = await screen.findByTestId("loop-field-on_retry-shape-0");
    fireEvent.change(screen.getByTestId("loop-field-on_retry-payload-0"), {
      target: { value: '{"ticket":128}' },
    });
    fireEvent.change(shape, { target: { value: "tool" } });
    await waitFor(() =>
      expect(screen.getByTestId("loop-field-on_retry-row")).toHaveAttribute("data-shape", "tool")
    );
    expect(screen.getByTestId("loop-field-on_retry-with-0")).toHaveValue("");

    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("loop-editor-publish"));
    await waitFor(() => expect(captured.definition).not.toBeNull());

    const onRetry = publishedNode(captured, "execute_task").on_retry as Record<string, unknown>[];
    // Exactly one variant survives — never `{emit, tool}`, which the linter rejects.
    expect(onRetry[0]).toHaveProperty("tool");
    expect(onRetry[0]).not.toHaveProperty("emit");
  });

  it("WT-007: keeps Publish enabled on a warning-only verdict and shows the warning", async () => {
    renderEditor("software-delivery", [detailHandler(waitWarningDetail)]);
    await screen.findByTestId("loop-editor");

    // `wait_expiry_without_path` is a real daemon warning: visible, never a gate.
    await waitFor(() =>
      expect(screen.getByTestId("loop-linter-warning-count")).toHaveTextContent("1 warning")
    );
    expect(screen.queryByTestId("loop-linter-error-count")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled();
    expandLinterDock();
    const row = screen.getByTestId("loop-linter-issue");
    expect(row).toHaveAttribute("data-severity", "warning");
    expect(row).toHaveTextContent("wait_expiry_without_path");
  });

  it("WT-008: keeps a read-only definition immutable through every authoring path", async () => {
    let forkBody: { fork_from_name?: string } | null = null;
    let forked = false;
    const fork = http.post("/api/workspaces/:workspaceId/loops", async ({ request }) => {
      forkBody = (await request.json()) as typeof forkBody;
      forked = true;
      return HttpResponse.json({ loop: deliveryDetail }, { status: 201 });
    });
    const detail = http.get("/api/workspaces/:workspaceId/loops/:name", () =>
      HttpResponse.json({ loop: forked ? deliveryDetail : readOnlySourceDetail })
    );
    renderEditor("software-delivery", [fork, detail]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8));

    expect(screen.getByTestId("loop-editor-readonly-strip")).toHaveTextContent(
      /read-only marketplace source/i
    );

    // Every mutation path is CLOSED, not merely decorated with a notice.
    fireEvent.click(screen.getByTestId("loop-palette-item-run-agent"));
    const goal = screen.getByRole("textbox", { name: "Goal (optional)" }) as HTMLTextAreaElement;
    const goalBefore = goal.value;
    fireEvent.change(goal, { target: { value: "rewritten" } });
    fireEvent.click(nodeCard("implement"));
    const fanOut = (await screen.findByTestId("loop-field-max_fan_out")) as HTMLInputElement;
    fireEvent.change(fanOut, { target: { value: "80" } });

    expect(screen.getAllByTestId("loop-editor-node")).toHaveLength(8);
    expect(goal.value).toBe(goalBefore);
    expect(fanOut.value).toBe("64");
    expect(screen.queryByTestId("loop-editor-dirty-chip")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v1");

    // Publish is illegal; Validate never writes, so it stays available; Fork is the one verb.
    expect(screen.getByTestId("loop-editor-publish")).toBeDisabled();
    expect(screen.getByTestId("loop-editor-validate")).not.toBeDisabled();
    fireEvent.click(screen.getByTestId("loop-editor-fork"));
    await waitFor(() => expect(forkBody).not.toBeNull());
    expect(forkBody).toEqual({ fork_from_name: "software-delivery" });
    await waitFor(() =>
      expect(screen.queryByTestId("loop-editor-readonly-strip")).not.toBeInTheDocument()
    );
    expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled();
  });

  it("WT-008: shows every returned issue on a publish 422 and never advances the version", async () => {
    const reject = http.patch("/api/workspaces/:workspaceId/loops/:name", () =>
      HttpResponse.json({ valid: false, errors: PUBLISH_REJECTED_ISSUES }, { status: 422 })
    );
    renderEditor("software-delivery", [reject]);
    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v4"));
    await waitFor(() => expect(screen.getByTestId("loop-editor-publish")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("loop-editor-publish"));

    const strip = await screen.findByTestId("loop-editor-publish-error");
    expect(strip).toHaveAttribute("role", "alert");
    expect(strip).toHaveTextContent(/2 issues to resolve/i);
    expect(strip).toHaveTextContent(/expected_version v4 kept/i);
    // The strip lists the daemon's own verdict, so the fix path needs no dock expansion.
    const issues = within(strip).getAllByTestId("loop-editor-publish-error-issue");
    expect(issues).toHaveLength(2);
    expect(issues[0]).toHaveTextContent("fan_out_ceiling_exceeded");
    expect(issues[1]).toHaveTextContent("error_route_conflict");
    // Nothing was saved: the compare-and-swap version pill is unchanged.
    expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v4");
  });

  it("Should show that the editor version was kept when a rejection has no issue list", () => {
    render(<LoopEditorPublishRejectedStrip message="Publish rejected" issues={[]} version={4} />);
    const strip = screen.getByTestId("loop-editor-publish-error");
    expect(strip).toHaveTextContent("expected_version v4 kept");
    expect(strip).not.toHaveTextContent("422");
  });

  it("Should publish the first edit of a version-zero workspace Loop with explicit CAS", async () => {
    let patchBody: {
      definition?: { meta?: { version?: number } };
      expected_version?: number | null;
    } | null = null;
    const versionZeroDetail: LoopDetail = {
      ...deliveryDetail,
      version: 0,
      definition: {
        ...deliveryDetail.definition,
        meta: { ...deliveryDetail.definition.meta, version: undefined },
      },
    };
    const loadFork = http.get("/api/workspaces/:workspaceId/loops/:name", () =>
      HttpResponse.json({ loop: versionZeroDetail })
    );
    const publish = http.patch("/api/workspaces/:workspaceId/loops/:name", async ({ request }) => {
      patchBody = (await request.json()) as typeof patchBody;
      return HttpResponse.json({
        loop: {
          ...versionZeroDetail,
          version: 1,
          definition: {
            ...versionZeroDetail.definition,
            meta: { ...versionZeroDetail.definition.meta, version: 1 },
          },
        },
      });
    });
    const { onPublished } = renderEditor("software-delivery", [loadFork, publish]);

    await screen.findByTestId("loop-editor");
    await waitFor(() => expect(screen.getByTestId("loop-editor-version")).toHaveTextContent("v0"));
    fireEvent.click(screen.getByTestId("loop-palette-item-collect"));
    fireEvent.click(screen.getByTestId("loop-editor-publish"));

    await waitFor(() => expect(onPublished).toHaveBeenCalledTimes(1));
    expect(patchBody).toEqual(
      expect.objectContaining({
        expected_version: 0,
        definition: expect.objectContaining({
          meta: expect.objectContaining({ version: 0 }),
        }),
      })
    );
    expect(onPublished.mock.calls[0][0].version).toBe(1);
  });
});
