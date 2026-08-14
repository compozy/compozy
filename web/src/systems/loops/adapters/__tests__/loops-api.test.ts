import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, fetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import { loopEffectiveConfigFixture } from "@/systems/loops/mocks/fixtures";
import {
  LoopsApiError,
  approveLoopRun,
  buildLoopStreamUrl,
  createLoop,
  deleteLoop,
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRun,
  listGoalTurns,
  listLoopRuns,
  listLoops,
  patchLoop,
  pauseLoopRun,
  putLoopAnnotations,
  putLoopConfig,
  resumeLoopRun,
  runLoop,
  cancelLoopRun,
  validateLoop,
} from "@/systems/loops";
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
    });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs?loop=implement-tasks&status=running&origin=session&origin_session=session_1&live=true&limit=10",
      method: "GET",
    });
  });

  it("Should paginate Goal turns with run-scoped filters and preserve after_seq zero", async () => {
    mockJsonResponse({ turns: [], next_after_seq: null });
    await listGoalTurns(WS, "run_1", {
      node: "goal",
      item: 0,
      after_seq: 0,
      limit: 25,
    });
    await expectFetchRequest({
      path: "/api/workspaces/ws_1/loop-runs/run_1/turns?node=goal&item=0&after_seq=0&limit=25",
      method: "GET",
    });

    mockJsonResponse({ error: "missing" }, { status: 404 });
    await expect(listGoalTurns(WS, "missing")).rejects.toMatchObject({
      status: 404,
      message: expect.stringContaining("missing"),
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
      "execute_task",
      "collect",
    ]);

    const runs = await listLoopRuns(WS, { loop: "implement-tasks" });
    expect(runs.runs.every(run => run.loop_name === "implement-tasks")).toBe(true);
    expect(runs.runs.every(run => run.definition_version === 0)).toBe(true);
    expect(runs.aggregates.total).toBeGreaterThan(0);

    const runDetail = await getLoopRun(WS, "looprun_running");
    expect(runDetail.run.status).toBe("running");
    expect(runDetail.generations?.[0]?.outputs.map(output => output.node_id)).toEqual([
      "slug_input",
      "load_tasks",
      "implement",
      "execute_task",
      "collect",
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
        implementer: "code_implementer",
        auto_commit: false,
      },
      input_origins: {
        slug: "run",
        implementer: "definition",
        auto_commit: "definition",
      },
    });
    expect(dryRun.dry_run?.nodes).toEqual([
      { id: "slug_input", class: "source", kind: "input" },
      {
        id: "load_tasks",
        class: "action",
        kind: "ext__spec_cycle__import_tasks",
        depends_on: ["slug_input"],
      },
      {
        id: "implement",
        class: "control",
        kind: "fan-out",
        depends_on: ["load_tasks"],
      },
      {
        id: "execute_task",
        class: "action",
        kind: "run-agent",
        depends_on: ["implement"],
      },
      {
        id: "collect",
        class: "control",
        kind: "collect",
        depends_on: ["execute_task"],
      },
    ]);
    const started = await runLoop(WS, "implement-tasks", {
      inputs: { slug: "billing-webhooks" },
    });
    expect(started.run?.inputs).toEqual({
      slug: "billing-webhooks",
      implementer: "code_implementer",
      auto_commit: false,
    });
    await expect(deleteLoop(WS, "implement-tasks")).resolves.toBeUndefined();
  });
});
