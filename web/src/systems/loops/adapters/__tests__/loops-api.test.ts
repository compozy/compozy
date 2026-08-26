import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, fetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import { loopEffectiveConfigFixture } from "@/systems/loops/mocks/fixtures";
import { SPEC_CYCLE_IMPORT_TASKS_KIND } from "@/systems/loops/mocks/fixture-action-kinds";
import {
  LoopInputValidationError,
  LoopReadError,
  LoopRequestError,
  LoopsApiError,
  LoopTimetravelError,
  amendLoopNode,
  approveLoopRun,
  buildLoopStreamUrl,
  createLoop,
  deleteLoop,
  diffLoopRun,
  forkLoopRun,
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRequest,
  getLoopRun,
  getLoopRunBriefing,
  getLoopRunRoster,
  getLoopRunTimeline,
  listLoopRequests,
  listLoopRuns,
  listLoops,
  patchLoop,
  pauseLoopRun,
  putLoopAnnotations,
  putLoopConfig,
  rerunLoopRun,
  respondLoopRequest,
  resumeLoopRun,
  runLoop,
  cancelLoopRun,
  validateLoop,
} from "@/systems/loops";
import { GRAPH_ENG_RUN_ID } from "@/systems/loops/mocks/fixture-graph-eng-requests";
import type { CreateLoopRequest, PatchLoopRequest } from "@/systems/loops";

const WS = "ws_1";

describe("buildLoopStreamUrl", () => {
  it("Should build a workspace-scoped stream URL with after_sequence when a seed is provided", () => {
    expect(buildLoopStreamUrl(WS, "run_1", { after_sequence: "14" })).toBe(
      "/api/workspaces/ws_1/loop-runs/run_1/events?after_sequence=14"
    );
  });

  it("Should keep after_sequence=0 for deterministic Last-Event-ID:0 precedence", () => {
    expect(buildLoopStreamUrl(WS, "run_1", { after_sequence: "0" })).toBe(
      "/api/workspaces/ws_1/loop-runs/run_1/events?after_sequence=0"
    );
  });

  it("Should omit the query string when no seed is provided", () => {
    expect(buildLoopStreamUrl(WS, "run_1")).toBe("/api/workspaces/ws_1/loop-runs/run_1/events");
  });

  it("Should encode unsafe characters in both the workspace and run segments", () => {
    expect(buildLoopStreamUrl("ws a", "run/1", { after_sequence: "1" })).toBe(
      "/api/workspaces/ws%20a/loop-runs/run%2F1/events?after_sequence=1"
    );
  });

  it("Should reject empty workspace and run ids", () => {
    expect(() => buildLoopStreamUrl("", "run_1")).toThrow(/workspace id is required/);
    expect(() => buildLoopStreamUrl(WS, "  ")).toThrow(/loop run id is required/);
  });
});

describe("loops-api (request construction + error mapping)", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should GET the workspace-scoped catalog", async () => {
    mockJsonResponse({
      facets: {
        categories: { delivery: 0 },
        kinds: { read_only: 0, workspace: 0 },
        statuses: {},
      },
      loops: [],
      page: { has_more: false, limit: 25, total: 0 },
    });
    const result = await listLoops(WS, {
      category: " delivery ",
      kind: "read_only",
      limit: 25,
      q: " release ",
      sort: "name",
      status: "running",
      cursor: " cursor-2 ",
    });
    expect(result.page).toEqual({ has_more: false, limit: 25, total: 0 });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops?q=release&kind=read_only&category=delivery&status=running&sort=name&cursor=cursor-2&limit=25",
      method: "GET",
    });
  });

  it("Should map a 404 loop read onto a typed LoopsApiError", async () => {
    mockJsonResponse({ error: "nope" }, { status: 404 });
    await expect(getLoop(WS, "missing")).rejects.toBeInstanceOf(LoopsApiError);
    mockJsonResponse({ error: "nope" }, { status: 404 });
    await expect(getLoop(WS, "missing")).rejects.toMatchObject({
      name: "LoopsApiError",
      status: 404,
      message: "Loop not found: missing",
    });
  });

  it("Should POST a new loop and surface a 409 conflict as a typed error", async () => {
    const body: CreateLoopRequest = { fork_from_name: "implement-tasks" };
    mockJsonResponse({ loop: { name: "implement-tasks" } }, { status: 201 });
    await createLoop(WS, body);
    await expectFetchRequest({ path: "/api/workspaces/ws_1/loops", method: "POST", body });

    mockJsonResponse({ error: "exists" }, { status: 409 });
    await expect(createLoop(WS, body)).rejects.toMatchObject({ status: 409 });
  });

  it("Should PATCH with expected_version and surface a 409 stale-editor conflict", async () => {
    const body = { expected_version: 3 } as PatchLoopRequest;
    // No server-provided error message -> the adapter's stale-editor fallback is used.
    mockJsonResponse({}, { status: 409 });
    await expect(patchLoop(WS, "implement-tasks", body)).rejects.toMatchObject({
      status: 409,
      message: expect.stringContaining("modified by another editor"),
    });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops/implement-tasks",
      method: "PATCH",
      body,
    });
  });

  it("Should DELETE a loop without throwing on 204", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));
    await expect(deleteLoop(WS, "implement-tasks")).resolves.toBeUndefined();
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops/implement-tasks",
      method: "DELETE",
    });
  });

  it("Should treat a 404 config read as a missing Loop", async () => {
    mockJsonResponse({ error: "no config" }, { status: 404 });
    await expect(getLoopConfig(WS, "implement-tasks")).rejects.toMatchObject({ status: 404 });
  });

  it("Should PUT the config override", async () => {
    mockJsonResponse({
      config: { iteration_cap: 8 },
      effective_config: loopEffectiveConfigFixture,
    });
    await putLoopConfig(WS, "implement-tasks", { config: { iteration_cap: 8 } });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops/implement-tasks/config",
      method: "PUT",
      body: { config: { iteration_cap: 8 } },
    });
  });

  it("Should pass the dry flag through on a dry run and omit it on a real run", async () => {
    mockJsonResponse({ dry_run: { loop_name: "implement-tasks" } });
    await runLoop(WS, "implement-tasks", { inputs: {} }, { dry: true });
    expect(new URL(fetchRequest().url).search).toBe("?dry=true");

    mockJsonResponse({ run: { id: "run_1" } }, { status: 201 });
    await runLoop(WS, "implement-tasks", { inputs: {} });
    expect(new URL(fetchRequest(1).url).search).toBe("");

    mockJsonResponse({ run: { id: "run_2" } }, { status: 201 });
    await runLoop(WS, "implement-tasks", { inputs: {} }, { profile: " marketing " });
    expect(new URL(fetchRequest(2).url).search).toBe("?profile=marketing");
  });

  it("Should return the 422 lint verdict from validate instead of throwing", async () => {
    mockJsonResponse(
      { valid: false, errors: [{ code: "unknown_reference", message: "x", severity: "error" }] },
      { status: 422 }
    );
    const result = await validateLoop(WS, "implement-tasks", {
      definition: { meta: { name: "implement-tasks" } },
    } as never);
    expect(result.valid).toBe(false);
    expect(result.errors).toHaveLength(1);
  });

  it("Should GET workspace-wide runs with the normalized filter query", async () => {
    mockJsonResponse({
      runs: [],
      aggregates: { total: 0, live: 0, terminal: 0, succeeded: 0, failed: 0 },
    });
    await listLoopRuns(WS, {
      loop: "implement-tasks",
      status: "running",
      origin: "session",
      origin_session: "session_1",
      live: true,
      limit: 10,
      profile: " marketing ",
      all_profiles: false,
    });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs?loop=implement-tasks&status=running&origin=session&origin_session=session_1&live=true&limit=10&profile=marketing&all_profiles=false",
      method: "GET",
    });
  });

  it("Should GET a single run and POST run controls to the scoped endpoints", async () => {
    mockJsonResponse({ run: { id: "run_1" } });
    await getLoopRun(WS, "run_1");
    await expectFetchRequest({ path: "/api/workspaces/ws_1/loop-runs/run_1", method: "GET" });

    mockJsonResponse({ ok: true });
    await pauseLoopRun(WS, "run_1");
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/pause",
      method: "POST",
      body: {},
      callIndex: 1,
    });

    mockJsonResponse({ ok: true });
    await resumeLoopRun(WS, "run_1");
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/resume",
      method: "POST",
      callIndex: 2,
    });

    mockJsonResponse({ ok: true, run_id: "run_1" });
    await cancelLoopRun(WS, "run_1");
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/cancel",
      method: "POST",
      callIndex: 3,
    });

    mockJsonResponse({ ok: true });
    await approveLoopRun(WS, "run_1", { decision: "approve", gate_id: "gate_1" });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/approve",
      method: "POST",
      body: { decision: "approve", gate_id: "gate_1" },
      callIndex: 4,
    });
  });

  it("Should GET + PUT annotations at the scoped editor endpoint", async () => {
    mockJsonResponse({ annotations: [{ node_id: "execute_task", x: 1, y: 2 }] });
    await getLoopAnnotations(WS, "implement-tasks");
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops/implement-tasks/annotations",
      method: "GET",
    });

    const body = { annotations: [{ node_id: "execute_task", x: 3, y: 4 }] };
    mockJsonResponse(body);
    await putLoopAnnotations(WS, "implement-tasks", body);
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loops/implement-tasks/annotations",
      method: "PUT",
      body,
      callIndex: 1,
    });
  });

  it("Should map every remaining failure status onto a typed LoopsApiError", async () => {
    mockJsonResponse({ error: "x" }, { status: 500 });
    await expect(listLoops(WS)).rejects.toMatchObject({ status: 500 });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(getLoopRun(WS, "missing")).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(getLoopAnnotations(WS, "missing")).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(putLoopAnnotations(WS, "missing", { annotations: [] })).rejects.toMatchObject({
      status: 404,
    });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(putLoopConfig(WS, "missing", { config: {} })).rejects.toMatchObject({
      status: 404,
    });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(deleteLoop(WS, "missing")).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(validateLoop(WS, "missing", { definition: {} } as never)).rejects.toMatchObject({
      status: 404,
    });

    mockJsonResponse({ error: "x" }, { status: 409 });
    await expect(runLoop(WS, "implement-tasks", { inputs: {} })).rejects.toMatchObject({
      status: 409,
    });

    mockJsonResponse({ error: "x" }, { status: 409 });
    await expect(resumeLoopRun(WS, "run_1")).rejects.toMatchObject({ status: 409 });

    mockJsonResponse({ error: "x" }, { status: 404 });
    await expect(cancelLoopRun(WS, "run_1")).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "x" }, { status: 422 });
    await expect(
      approveLoopRun(WS, "run_1", { decision: "approve", gate_id: "g" })
    ).rejects.toMatchObject({ status: 422 });
  });

  it("Should preserve a run input rejection as a field-addressed error", async () => {
    mockJsonResponse(
      {
        valid: false,
        input_validation: {
          loop: "implement-tasks",
          field: "agent",
          kind: "agent",
          value: "missing-agent",
          origin: "run",
          reason: "unknown_reference",
        },
      },
      { status: 422 }
    );

    const rejection = runLoop(WS, "implement-tasks", {
      inputs: { agent: "missing-agent" },
    });

    await expect(rejection).rejects.toEqual(
      expect.objectContaining<Partial<LoopInputValidationError>>({
        name: "LoopInputValidationError",
        status: 422,
        fieldErrors: { agent: "agent references an unavailable agent." },
        validation: expect.objectContaining({
          field: "agent",
          origin: "run",
          reason: "unknown_reference",
        }),
      })
    );
  });

  // The three run reads (ADR-005). Their filters are normalised the same way the
  // rest of the adapters normalise theirs, and their refusals carry structure —
  // an allowed-state list, a stale-cursor code — that the UI acts on rather than
  // prints. Both halves are locked here, in the adapters' canonical suite.
  it("Should GET the run briefing and keep a 404's reason envelope typed", async () => {
    mockJsonResponse({ run_id: "r-1", status: "running" });
    await getLoopRunBriefing(WS, "r-1");
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/r-1/briefing",
      method: "GET",
    });

    mockJsonResponse(
      { error: "loop_run_not_found", code: "loop_run_not_found", details: { run_id: "r-1" } },
      { status: 404 }
    );
    // A 404 that drops `code` and `details` cannot say which run was missed.
    await expect(getLoopRunBriefing(WS, "r-1")).rejects.toMatchObject({
      name: "LoopReadError",
      status: 404,
      code: "loop_run_not_found",
      message: "Loop run not found",
      details: { run_id: "r-1" },
    });
  });

  it("Should GET the roster with trimmed filters and expose the allowed states on a refusal", async () => {
    mockJsonResponse({ run_id: "r-1", nodes: [], fanout_rollups: [] });
    await getLoopRunRoster(WS, "r-1", {
      state: " running ",
      generation: 2,
      cursor: "  ",
      limit: 200,
    });
    // A blank cursor is an absent cursor, exactly as elsewhere in the adapters.
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/r-1/nodes?state=running&generation=2&limit=200",
      method: "GET",
    });

    mockJsonResponse(
      {
        error: "invalid_node_state",
        code: "invalid_node_state",
        details: { allowed: "all,running,failed" },
      },
      { status: 400 }
    );
    // A state the client allows, refused by the daemon anyway: this asserts the
    // envelope mapping, not the local allowlist (which the next case owns).
    const refusal = await getLoopRunRoster(WS, "r-1", { state: "running" }).catch(
      (error: unknown) => error
    );
    expect(refusal).toBeInstanceOf(LoopReadError);
    expect((refusal as InstanceType<typeof LoopReadError>).allowedStates).toEqual([
      "all",
      "running",
      "failed",
    ]);
  });

  it("Should refuse a state outside the allowlist without spending a request", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockClear();
    // `pending` is an output state, never a filter value. The daemon would
    // answer 400; knowing that already, the adapter does not ask.
    const refusal = await getLoopRunRoster(WS, "r-1", { state: "pending" }).catch(
      (error: unknown) => error
    );
    expect(refusal).toBeInstanceOf(LoopReadError);
    expect((refusal as InstanceType<typeof LoopReadError>).code).toBe("invalid_node_state");
    expect((refusal as InstanceType<typeof LoopReadError>).allowedStates).toContain("all");
    expect((refusal as InstanceType<typeof LoopReadError>).allowedStates).not.toContain("pending");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("Should GET the timeline and mark a branch-changed cursor as stale", async () => {
    mockJsonResponse({ run_id: "r-1", head_seq: 12, entries: [] });
    await getLoopRunTimeline(WS, "r-1", { view: " all ", cursor: " tok-1 ", after_sequence: 0 });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/r-1/timeline?view=all&cursor=tok-1&after_sequence=0",
      method: "GET",
    });

    mockJsonResponse(
      { error: "timeline_branch_changed", code: "timeline_branch_changed" },
      { status: 409 }
    );
    const refusal = await getLoopRunTimeline(WS, "r-1", { cursor: "tok-1" }).catch(
      (error: unknown) => error
    );
    expect(refusal).toBeInstanceOf(LoopReadError);
    // The story restarts from the newest window; it never splices two histories.
    expect((refusal as InstanceType<typeof LoopReadError>).isStaleCursor).toBe(true);
  });
});

describe("loops-api (against MSW mock handlers)", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should resolve the catalog, a definition, runs and a run detail from the fixtures", async () => {
    // Invariant: the adapter fixture catalog mirrors every bundled spec-cycle Loop and exposes
    // each real graph. This adapter suite owns the public mock HTTP boundary.
    const loops = await listLoops(WS);
    expect(loops.loops.map(loop => loop.name)).toEqual(["implement-tasks", "review-and-fix"]);

    const detail = await getLoop(WS, "implement-tasks");
    expect(detail.definition.meta.name).toBe("implement-tasks");
    expect(detail.definition.meta.version).toBe(0);
    expect(detail.definition.concurrency).toBe("forbid");
    expect(detail.definition.graph.nodes.map(node => node.id)).toEqual([
      "slug_input",
      "load_tasks",
      "implement",
      "select_mode",
      "select_category",
      "stage_orchestrated",
      "execute_backend",
      "execute_frontend",
      "execute_default",
      "collect",
      "select_delivery",
      "per_task_done",
      "orchestrate",
    ]);

    const runs = await listLoopRuns(WS, { loop: "implement-tasks" });
    expect(runs.runs.every(run => run.loop_name === "implement-tasks")).toBe(true);
    expect(runs.runs.every(run => run.definition_version === 0)).toBe(true);
    expect(runs.aggregates.total).toBeGreaterThan(0);

    const runDetail = await getLoopRun(WS, "looprun_running");
    expect(runDetail.run.status).toBe("running");
    expect(runDetail.generations?.[0]?.outputs.map(output => output.node_id)).toEqual([
      "slug_input",
      "select_mode",
      "load_tasks",
      "implement",
      "select_category",
      "execute_backend",
      "execute_frontend",
      "execute_default",
      "collect",
      "orchestrate",
    ]);

    const stalledDetail = await getLoopRun(WS, "looprun_stalled");
    expect(stalledDetail.generations?.map(generation => generation.generation)).toEqual([
      1, 2, 3, 4,
    ]);
    expect(stalledDetail.generations?.map(generation => generation.parent_generation)).toEqual([
      0, 1, 2, 3,
    ]);
  });

  it("Should surface a 404 for an unknown loop name through the handler", async () => {
    await expect(getLoop(WS, "ghost")).rejects.toMatchObject({ status: 404 });
  });

  it("Should resolve config, annotations, validate, run controls and a dry run from the fixtures", async () => {
    expect(await getLoopConfig(WS, "implement-tasks")).toMatchObject({
      config: { iteration_cap: 16 },
      effectiveConfig: { fan_out_width: 4 },
    });
    expect(
      await putLoopConfig(WS, "implement-tasks", { config: { iteration_cap: 9 } })
    ).toMatchObject({
      config: { iteration_cap: 9 },
      effectiveConfig: { fan_out_width: 4 },
    });
    expect(await getLoopAnnotations(WS, "implement-tasks")).toHaveLength(2);
    expect(
      await putLoopAnnotations(WS, "implement-tasks", {
        annotations: [{ node_id: "n", x: 1, y: 2 }],
      })
    ).toEqual([{ node_id: "n", x: 1, y: 2 }]);
    expect(await validateLoop(WS, "implement-tasks", { definition: {} } as never)).toMatchObject({
      valid: true,
    });
    expect(await resumeLoopRun(WS, "looprun_running")).toEqual({ ok: true });
    expect(await cancelLoopRun(WS, "looprun_running")).toEqual({
      ok: true,
      run_id: "looprun_running",
    });
    const dryRun = await runLoop(
      WS,
      "implement-tasks",
      { inputs: { slug: "billing-webhooks" } },
      { dry: true }
    );
    expect(dryRun.dry_run).toMatchObject({
      resolved_inputs: {
        slug: "billing-webhooks",
        mode: "per-task",
        implementer: "code_implementer",
        orchestrator: "orchestrator",
        auto_commit: false,
      },
      input_origins: {
        slug: "run",
        mode: "definition",
        implementer: "definition",
        orchestrator: "definition",
        auto_commit: "definition",
      },
    });
    expect(dryRun.dry_run?.nodes).toEqual([
      { id: "slug_input", class: "source", kind: "input" },
      {
        id: "load_tasks",
        class: "action",
        kind: SPEC_CYCLE_IMPORT_TASKS_KIND,
        depends_on: ["slug_input"],
      },
      {
        id: "implement",
        class: "control",
        kind: "fan-out",
        depends_on: ["load_tasks"],
      },
      {
        id: "select_mode",
        class: "control",
        kind: "route",
        depends_on: ["implement"],
      },
      {
        id: "select_category",
        class: "control",
        kind: "route",
        depends_on: ["select_mode"],
      },
      {
        id: "stage_orchestrated",
        class: "action",
        kind: "transform",
        depends_on: ["select_mode"],
      },
      {
        id: "execute_backend",
        class: "action",
        kind: "run-agent",
        depends_on: ["select_category"],
      },
      {
        id: "execute_frontend",
        class: "action",
        kind: "run-agent",
        depends_on: ["select_category"],
      },
      {
        id: "execute_default",
        class: "action",
        kind: "run-agent",
        depends_on: ["select_category"],
      },
      {
        id: "collect",
        class: "control",
        kind: "collect",
        depends_on: [
          "stage_orchestrated",
          "execute_backend",
          "execute_frontend",
          "execute_default",
        ],
      },
      {
        id: "select_delivery",
        class: "control",
        kind: "route",
        depends_on: ["collect"],
      },
      {
        id: "per_task_done",
        class: "action",
        kind: "transform",
        depends_on: ["select_delivery"],
      },
      {
        id: "orchestrate",
        class: "action",
        kind: "goal",
        depends_on: ["select_delivery"],
      },
    ]);
    const started = await runLoop(WS, "implement-tasks", {
      inputs: { slug: "billing-webhooks" },
    });
    expect(started.run?.inputs).toEqual({
      slug: "billing-webhooks",
      mode: "per-task",
      implementer: "code_implementer",
      orchestrator: "orchestrator",
      auto_commit: false,
    });
    await expect(deleteLoop(WS, "implement-tasks")).resolves.toBeUndefined();
  });
});

async function expectRejection<E extends Error>(
  promise: Promise<unknown>,
  ctor: new (...args: never[]) => E
): Promise<E> {
  const error = await promise.then(
    () => null,
    (reason: unknown) => reason
  );
  expect(error).toBeInstanceOf(ctor);
  return error as E;
}

describe("loop requests + time travel (request construction + error mapping)", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should address the request inventory, one request, and the respond verb", async () => {
    mockJsonResponse({ items: [], aggregates: { pending: 0 }, next_cursor: "" });
    await listLoopRequests(WS, { state: "pending", run_id: "run_1", limit: 25 });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-requests?state=pending&run_id=run_1&limit=25",
      method: "GET",
    });

    mockJsonResponse({ node_id: "pick_envs" });
    await getLoopRequest({
      workspaceId: WS,
      runId: "run_1",
      generation: 3,
      nodeId: "pick_envs",
      itemIndex: 2,
    });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/nodes/pick_envs/request?generation=3&item_index=2",
      method: "GET",
      callIndex: 1,
    });

    const body = {
      generation: 3,
      decision: "edit",
      payload: { tag: "v2" },
      note: "fixed tag",
      item_index: 0,
    };
    mockJsonResponse({ ok: true });
    await respondLoopRequest({ workspaceId: WS, runId: "run_1", nodeId: "publish" }, body);
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/nodes/publish/respond",
      method: "POST",
      body,
      callIndex: 2,
    });
  });

  it("Should address diff, rerun, fork, and amend at their scoped endpoints", async () => {
    mockJsonResponse({ kind: "generation" });
    await diffLoopRun(
      { workspaceId: WS, runId: "run_1" },
      { generation: 1, against_generation: 2 }
    );
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/diff?generation=1&against_generation=2",
      method: "GET",
    });

    const rerunBody = { from_node: "shard_verify", reason: "flaky infra", request_id: "r-1" };
    mockJsonResponse({ run_id: "run_1", generation: 3 });
    await rerunLoopRun({ workspaceId: WS, runId: "run_1" }, rerunBody);
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/rerun",
      method: "POST",
      body: rerunBody,
      callIndex: 1,
    });

    const forkBody = { generation: 2, inputs: { service: "payments" } };
    mockJsonResponse({ run: { id: "run_2" } }, { status: 201 });
    await forkLoopRun({ workspaceId: WS, runId: "run_1" }, forkBody);
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/fork",
      method: "POST",
      body: forkBody,
      callIndex: 2,
    });

    const amendBody = { payload: { risk: "medium" }, reason: "over-rated" };
    mockJsonResponse({ ok: true, amendment: {} });
    await amendLoopNode({ workspaceId: WS, runId: "run_1", nodeId: "classify" }, amendBody);
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/nodes/classify/amend",
      method: "POST",
      body: amendBody,
      callIndex: 3,
    });
  });

  it("Should keep the daemon's reason envelope on every deterministic respond refusal", async () => {
    mockJsonResponse(
      {
        error: "answer does not match",
        code: "request_validation_failed",
        details: { regions: "minItems: array must have at least 1 items" },
      },
      { status: 422 }
    );
    const validation = await expectRejection(
      respondLoopRequest(
        { workspaceId: WS, runId: "run_1", nodeId: "pick_envs" },
        { generation: 3, payload: { regions: [] } }
      ),
      LoopRequestError
    );
    expect(validation.status).toBe(422);
    expect(validation.code).toBe("request_validation_failed");
    expect(validation.isAnswerable).toBe(true);
    expect(validation.fieldErrors).toEqual({
      regions: "minItems: array must have at least 1 items",
    });

    mockJsonResponse(
      {
        error: "already answered",
        code: "request_already_answered",
        details: { decision: "reject" },
      },
      { status: 409 }
    );
    const answered = await expectRejection(
      respondLoopRequest(
        { workspaceId: WS, runId: "run_1", nodeId: "publish" },
        { generation: 3, payload: {} }
      ),
      LoopRequestError
    );
    expect(answered.status).toBe(409);
    expect(answered.recordedDecision).toBe("reject");
    expect(answered.isAnswerable).toBe(false);
    expect(answered.fieldErrors).toEqual({});

    for (const [status, code] of [
      [410, "request_expired"],
      [410, "request_canceled"],
      [403, "respond_not_permitted"],
      [403, "respond_self_denied"],
    ] as const) {
      mockJsonResponse({ error: code, code }, { status });
      const refusal = await expectRejection(
        respondLoopRequest(
          { workspaceId: WS, runId: "run_1", nodeId: "publish" },
          { generation: 3, payload: {} }
        ),
        LoopRequestError
      );
      expect(refusal.code).toBe(code);
      expect(refusal.isAnswerable).toBe(false);
    }

    mockJsonResponse({ error: "gone" }, { status: 404 });
    await expect(
      getLoopRequest({ workspaceId: WS, runId: "run_1", generation: 3, nodeId: "ghost" })
    ).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "boom" }, { status: 500 });
    await expect(listLoopRequests(WS)).rejects.toMatchObject({ status: 500 });
  });

  it("Should keep the reason envelope on every time-travel refusal", async () => {
    const cases = [
      [409, "rerun_busy"],
      [422, "rerun_node_unsettled"],
      [409, "timetravel_key_reuse"],
      [403, "timetravel_self_denied"],
    ] as const;
    for (const [status, code] of cases) {
      mockJsonResponse({ error: code, code, details: { actual_state: "running" } }, { status });
      const refusal = await expectRejection(
        rerunLoopRun({ workspaceId: WS, runId: "run_1" }, { from_node: "shard_verify" }),
        LoopTimetravelError
      );
      expect(refusal.status).toBe(status);
      expect(refusal.code).toBe(code);
      expect(refusal.details.actual_state).toBe("running");
    }

    mockJsonResponse(
      { error: "unknown generation", code: "fork_generation_unknown" },
      { status: 404 }
    );
    const fork = await expectRejection(
      forkLoopRun({ workspaceId: WS, runId: "run_1" }, { generation: 99 }),
      LoopTimetravelError
    );
    expect(fork.code).toBe("fork_generation_unknown");

    mockJsonResponse({ error: "cross loop", code: "diff_cross_loop" }, { status: 422 });
    await expect(diffLoopRun({ workspaceId: WS, runId: "run_1" })).rejects.toMatchObject({
      code: "diff_cross_loop",
    });

    mockJsonResponse(
      { error: "bad shape", code: "request_validation_failed", details: { risk: "required" } },
      { status: 422 }
    );
    const amend = await expectRejection(
      amendLoopNode({ workspaceId: WS, runId: "run_1", nodeId: "classify" }, { payload: {} }),
      LoopRequestError
    );
    expect(amend.fieldErrors).toEqual({ risk: "required" });

    mockJsonResponse({ error: "gone" }, { status: 404 });
    await expect(
      amendLoopNode({ workspaceId: WS, runId: "run_1", nodeId: "ghost" }, { payload: {} })
    ).rejects.toMatchObject({ status: 404 });

    mockJsonResponse({ error: "boom" }, { status: 500 });
    await expect(
      forkLoopRun({ workspaceId: WS, runId: "run_1" }, { generation: 1 })
    ).rejects.toMatchObject({ status: 500 });
  });
});

describe("loop requests + time travel (against MSW mock handlers)", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should read the daemon-computed pending aggregate rather than a page count", async () => {
    const pending = await listLoopRequests(WS, { state: "pending" });
    expect(pending.items.map(entry => entry.node_id)).toEqual([
      "confirm-rollout",
      "apply-migration",
    ]);
    expect(pending.aggregates.pending).toBe(2);

    const filtered = await listLoopRequests(WS, { state: "pending", run_id: "nope" });
    expect(filtered.items).toHaveLength(0);
    expect(filtered.aggregates.pending).toBe(2);

    const resolved = await listLoopRequests(WS, { state: "resolved" });
    expect(resolved.items.map(entry => entry.state)).toEqual(["answered", "expired", "canceled"]);
  });

  it("Should return the full redacted context only from the per-request detail read", async () => {
    const listed = await listLoopRequests(WS, { state: "pending" });
    const preview = listed.items[0]?.context as Record<string, unknown>;
    expect(preview.plan).toBeUndefined();

    const detail = await getLoopRequest({
      workspaceId: WS,
      runId: GRAPH_ENG_RUN_ID,
      generation: 3,
      nodeId: "confirm-rollout",
      itemIndex: 0,
    });
    expect((detail.context as Record<string, unknown>).plan).toContain("complete redacted context");

    await expect(
      getLoopRequest({
        workspaceId: WS,
        runId: GRAPH_ENG_RUN_ID,
        generation: 3,
        nodeId: "ghost",
      })
    ).rejects.toMatchObject({ status: 404 });
  });

  it("Should resolve respond, amend, diff, rerun and fork through the handlers", async () => {
    await expect(
      respondLoopRequest(
        { workspaceId: WS, runId: GRAPH_ENG_RUN_ID, nodeId: "confirm-rollout" },
        { generation: 3, payload: { regions: [] } }
      )
    ).rejects.toMatchObject({ code: "request_validation_failed", status: 422 });

    const answered = await respondLoopRequest(
      { workspaceId: WS, runId: GRAPH_ENG_RUN_ID, nodeId: "confirm-rollout" },
      { generation: 3, payload: { regions: ["eu"], canary: true } }
    );
    expect(answered).toMatchObject({ state: "answered", provenance: { actor_id: "pedro" } });

    const amended = await amendLoopNode(
      { workspaceId: WS, runId: GRAPH_ENG_RUN_ID, nodeId: "render-notes" },
      { payload: { risk: "medium" }, reason: "over-rated" }
    );
    expect(amended.amendment).toMatchObject({ original: { risk: "high" }, amendment_seq: 1 });

    const diff = await diffLoopRun(
      { workspaceId: WS, runId: GRAPH_ENG_RUN_ID },
      { generation: 2, against_generation: 3 }
    );
    expect(diff.kind).toBe("generation");
    expect(diff.nodes.map(node => node.change)).toEqual([
      "changed",
      "rerun",
      "skipped",
      "carried",
      "verdict",
    ]);

    const rerun = await rerunLoopRun(
      { workspaceId: WS, runId: GRAPH_ENG_RUN_ID },
      { from_node: "render-notes" }
    );
    expect(rerun.rerun_nodes).toContain("render-notes");
    await expect(
      rerunLoopRun({ workspaceId: WS, runId: GRAPH_ENG_RUN_ID }, { from_node: "confirm-rollout" })
    ).rejects.toMatchObject({ code: "rerun_node_unsettled" });

    const forked = await forkLoopRun(
      { workspaceId: WS, runId: GRAPH_ENG_RUN_ID },
      { generation: 2, inputs: { severity: "p0" } }
    );
    expect(forked.run.forked_from).toMatchObject({ generation: 2 });
    await expect(
      forkLoopRun({ workspaceId: WS, runId: GRAPH_ENG_RUN_ID }, { generation: 99 })
    ).rejects.toMatchObject({ code: "fork_generation_unknown", status: 404 });
  });
});
