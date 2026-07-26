import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import {
  buildLiveNetworkParticipationFixture,
  buildLocalNetworkParticipationFixture,
  type ResolvedNetworkParticipationFixture,
} from "@/test/network-participation-fixtures";
import type {
  LoopAnnotationsUpdateRequest,
  LoopConfigUpdateRequest,
  LoopDefinition,
  LoopDefinitionMeta,
  LoopValidationIssue,
} from "@/systems/loops";

import {
  loopAnnotationsFixture,
  loopCatalogFixtures,
  loopConfigFixture,
  loopEffectiveConfigFixture,
  loopDetailByName,
  loopRunAggregatesFixture,
  loopRunDetailByRunId,
  loopRunFixtures,
} from "./fixtures";

const catalogByName = new Map(loopCatalogFixtures.map(entry => [entry.name, entry]));

const FAN_OUT_CEILING = 64;
const loopRunEventStreamEncoder = new TextEncoder();

interface MockLintIssue {
  node_id?: string;
  code: string;
  message: string;
  severity: "error" | "warning";
}

/** Detects the first node caught in an edge cycle, mirroring the daemon acyclicity check. */
function firstCycleNode(edges: { from: string; to: string }[]): string | null {
  const adjacency = new Map<string, string[]>();
  for (const edge of edges) {
    const list = adjacency.get(edge.from) ?? [];
    list.push(edge.to);
    adjacency.set(edge.from, list);
  }
  const visiting = new Set<string>();
  const done = new Set<string>();
  let found: string | null = null;
  const walk = (node: string) => {
    if (found || done.has(node)) return;
    visiting.add(node);
    for (const next of adjacency.get(node) ?? []) {
      if (visiting.has(next)) {
        found = next;
        return;
      }
      walk(next);
    }
    visiting.delete(node);
    done.add(node);
  };
  for (const node of adjacency.keys()) walk(node);
  return found;
}

/**
 * A faithful stand-in for the shared Go linter over the posted definition: it flags any
 * fan-out node whose `max_fan_out` exceeds the daemon ceiling and any acyclicity break,
 * returning the same `{ node_id, code, message, severity }` per-node shape the daemon
 * emits (ADR-023). This makes the editor's validate → 422 → Publish gate real in tests.
 */
function lintDefinition(graph?: { nodes?: unknown[]; edges?: unknown[] }): MockLintIssue[] {
  const issues: MockLintIssue[] = [];
  const nodes = Array.isArray(graph?.nodes) ? (graph!.nodes as Record<string, unknown>[]) : [];
  for (const node of nodes) {
    const fanOut = node.max_fan_out;
    if (typeof fanOut === "number" && fanOut > FAN_OUT_CEILING) {
      issues.push({
        node_id: String(node.id),
        code: "fan_out_ceiling_exceeded",
        message: `max_fan_out (${fanOut}) exceeds the daemon ceiling of ${FAN_OUT_CEILING}.`,
        severity: "error",
      });
    }
  }
  const edges = Array.isArray(graph?.edges)
    ? (graph!.edges as { from: string; to: string }[]).filter(edge => edge?.from && edge?.to)
    : [];
  const cycleNode = firstCycleNode(edges);
  if (cycleNode) {
    issues.push({
      node_id: cycleNode,
      code: "cycle_detected",
      message: `The graph is not acyclic: ${cycleNode} is part of a cycle.`,
      severity: "error",
    });
  }
  return issues;
}

function createLoopRunEventsStreamResponse(workspaceId: string, runId: string): Response {
  const frame = {
    id: `${runId}:storybook:1`,
    seq: 1,
    at: "2026-04-17T18:10:00Z",
    workspace_id: workspaceId,
    loop_run_id: runId,
    kind: "node_running",
    payload: { node_id: "execute_task", generation: 2 },
  };
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        loopRunEventStreamEncoder.encode(`event: node_running\ndata: ${JSON.stringify(frame)}\n\n`)
      );
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
  });
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/workspaces/{workspace_id}/loops", () =>
    HttpResponse.json({
      facets: {
        categories: { delivery: 1, watch: 1 },
        kinds: { read_only: 1, workspace: 1 },
        statuses: { running: 1, watching: 1 },
      },
      loops: loopCatalogFixtures,
      page: {
        has_more: false,
        limit: 50,
        total: loopCatalogFixtures.length,
      },
    })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loops", () =>
    HttpResponse.json({ loop: loopDetailByName.get("software-delivery")! }, { status: 201 })
  ),
  aghApiMock.get("/api/workspaces/{workspace_id}/loops/{name}", ({ params }) => {
    const detail = loopDetailByName.get(String(params.name));
    if (!detail) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    return HttpResponse.json({ loop: detail });
  }),
  aghApiMock.patch("/api/workspaces/{workspace_id}/loops/{name}", async ({ params, request }) => {
    const detail = loopDetailByName.get(String(params.name));
    if (!detail) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    const body = (await request.json().catch(() => ({}))) as {
      definition?: Record<string, unknown>;
      expected_version?: number | null;
    };
    // Echo the published definition with a bumped monotonic meta.version (§9.13);
    // the store is not mutated so parallel tests stay isolated.
    const published = (body.definition ?? detail.definition) as Partial<LoopDefinition>;
    const publishedMeta =
      typeof published.meta === "object" && published.meta !== null
        ? (published.meta as Partial<LoopDefinitionMeta>)
        : {};
    const nextVersion = (detail.version ?? 0) + 1;
    return HttpResponse.json({
      loop: {
        ...detail,
        version: nextVersion,
        definition: {
          ...detail.definition,
          ...published,
          meta: { ...detail.definition.meta, ...publishedMeta, version: nextVersion },
        },
      },
    });
  }),
  aghApiMock.delete("/api/workspaces/{workspace_id}/loops/{name}", ({ params }) => {
    if (!catalogByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    return new HttpResponse(null, { status: 204 });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/config", ({ params }) => {
    if (!catalogByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    return HttpResponse.json({
      config: loopConfigFixture,
      effective_config: loopEffectiveConfigFixture,
    });
  }),
  aghApiMock.put("/api/workspaces/{workspace_id}/loops/{name}/config", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Partial<LoopConfigUpdateRequest>;
    return HttpResponse.json({
      config: body.config ?? loopConfigFixture,
      effective_config: loopEffectiveConfigFixture,
    });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/annotations", () =>
    HttpResponse.json({ annotations: loopAnnotationsFixture })
  ),
  aghApiMock.put("/api/workspaces/{workspace_id}/loops/{name}/annotations", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Partial<LoopAnnotationsUpdateRequest>;
    return HttpResponse.json({ annotations: body.annotations ?? loopAnnotationsFixture });
  }),
  aghApiMock.post(
    "/api/workspaces/{workspace_id}/loops/{name}/run",
    async ({ request, params }) => {
      const url = new URL(request.url);
      const name = String(params.name);
      const workspaceId = String(params.workspace_id);
      const entry = catalogByName.get(name);
      if (!entry) {
        return HttpResponse.json({ error: `Loop not found: ${name}` }, { status: 404 });
      }
      const detail = entry.last_run ? loopRunDetailByRunId.get(entry.last_run.id) : undefined;
      const body = (await request.json().catch(() => ({}))) as {
        network_participation?: {
          mode?: string | null;
          channel_id?: string | null;
          channel_strategy?: string | null;
        } | null;
      };
      const requested = body.network_participation;
      let resolvedParticipation: ResolvedNetworkParticipationFixture =
        buildLocalNetworkParticipationFixture();
      if (requested?.mode === "live") {
        const strategy = requested.channel_strategy;
        const channelId = requested.channel_id?.trim() ?? "";
        if (strategy === "named" && channelId) {
          resolvedParticipation = buildLiveNetworkParticipationFixture({ workspaceId, channelId });
        } else if (strategy === "loop_run") {
          resolvedParticipation = buildLiveNetworkParticipationFixture({
            workspaceId,
            channelId: `loop-${detail?.run.id ?? name}`,
            channelStrategy: "loop_run",
          });
        } else {
          return HttpResponse.json(
            { error: "Loop Live participation requires named or loop_run strategy." },
            { status: 422 }
          );
        }
      }
      if (url.searchParams.get("dry") === "true") {
        return HttpResponse.json({
          dry_run: {
            loop_name: name,
            generation: 1,
            resolved_inputs: {},
            resolved_network_participation: resolvedParticipation,
            contract: entry.contract,
            nodes: [{ id: "plan", kind: "run-agent", class: "action" }],
            effective_config: {
              iteration_cap: 12,
              budget_tokens: 500_000,
              budget_wall_sec: 3_600,
              budget_on_exceeded: "halt",
              fan_out_width: 4,
              gate_max_revisions: 3,
              human_gate_enabled: true,
              model_defaults: { judge: "claude", worker: "codex" },
              no_progress_window: 3,
              reattempt_strategy: "failed_only",
              enabled_checks_json: null,
            },
          },
        });
      }
      if (!detail) {
        return HttpResponse.json({ error: `Loop run not found for ${name}` }, { status: 404 });
      }
      return HttpResponse.json(
        {
          run: {
            ...detail.run,
            resolved_network_participation: resolvedParticipation,
          },
        },
        { status: 201 }
      );
    }
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loops/{name}/validate", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      definition?: { graph?: { nodes?: unknown[]; edges?: unknown[] } };
    };
    const errors = lintDefinition(body.definition?.graph);
    return HttpResponse.json({
      valid: errors.length === 0,
      errors: errors satisfies LoopValidationIssue[],
    });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/loop-runs", ({ request }) => {
    const url = new URL(request.url);
    const loop = url.searchParams.get("loop");
    const status = url.searchParams.get("status");
    const runs = loopRunFixtures.filter(run => {
      if (loop && run.loop_name !== loop) return false;
      if (status && run.status !== status) return false;
      return true;
    });
    return HttpResponse.json({ runs, aggregates: loopRunAggregatesFixture });
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", ({ params }) => {
    const detail = loopRunDetailByRunId.get(String(params.run_id));
    if (!detail) {
      return HttpResponse.json(
        { error: `Loop run not found: ${String(params.run_id)}` },
        { status: 404 }
      );
    }
    return HttpResponse.json(detail);
  }),
  aghApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns", ({ params }) => {
    const runId = String(params.run_id);
    if (!loopRunDetailByRunId.has(runId)) {
      return HttpResponse.json({ error: `Loop run not found: ${runId}` }, { status: 404 });
    }
    return HttpResponse.json({ turns: [], next_after_seq: null });
  }),
  aghApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/events",
    ({ params, response }) => {
      const workspaceId = String(params.workspace_id);
      const runId = String(params.run_id);
      if (!loopRunDetailByRunId.has(runId)) {
        return HttpResponse.json({ error: `Loop run not found: ${runId}` }, { status: 404 });
      }
      return response.untyped(createLoopRunEventsStreamResponse(workspaceId, runId));
    }
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/approve", () =>
    HttpResponse.json({ ok: true })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/pause", () =>
    HttpResponse.json({ ok: true })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/resume", () =>
    HttpResponse.json({ ok: true })
  ),
  aghApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/stop", () =>
    HttpResponse.json({ ok: true })
  ),
];
