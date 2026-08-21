import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
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
  LoopDryRunNode,
  LoopValidationIssue,
} from "@/systems/loops";

import { loopCategory, loopKind } from "../lib/loop-catalog";
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
import { graphEngHandlers } from "./handlers-graph-eng";
import { graphEngRunDetailByRunId, graphEngRunFixtures } from "./fixture-graph-eng-runs";
import {
  resolveLoopRunBriefing,
  resolveLoopRunRoster,
  resolveLoopRunTimeline,
} from "./fixture-run-reads";
import { RELEASE_TRAIN_LOOP_NAME, releaseTrainDetail } from "./fixture-release-train";
import { lintDefinition } from "./lint-definition";
import { materializeContractFixture } from "./materialize-contract-fixture";

const catalogByName = new Map(loopCatalogFixtures.map(entry => [entry.name, entry]));

const runDetailByRunId = new Map([...loopRunDetailByRunId, ...graphEngRunDetailByRunId]);
const runListFixtures = [...loopRunFixtures, ...graphEngRunFixtures];
const detailByName = new Map([
  ...loopDetailByName,
  [RELEASE_TRAIN_LOOP_NAME, releaseTrainDetail] as const,
]);

const definitionNames = new Set(detailByName.keys());

function resolveRunInputs(
  entry: (typeof loopCatalogFixtures)[number],
  requested: Record<string, unknown>
) {
  const defaults = Object.fromEntries(
    Object.entries(entry.inputs ?? {})
      .filter(([, schema]) => schema.default !== undefined)
      .map(([name, schema]) => [name, schema.default])
  );
  const resolved = { ...defaults, ...requested };
  const origins = Object.fromEntries(
    Object.keys(resolved).map(name => [
      name,
      Object.prototype.hasOwnProperty.call(requested, name) ? "run" : "definition",
    ])
  );
  return { origins, resolved };
}

function previewNodes(definition: LoopDefinition | undefined): LoopDryRunNode[] {
  if (!definition) return [];
  const edges = definition.graph.edges as unknown as Array<{ from: string; to: string }>;
  return (
    definition.graph.nodes as unknown as Array<{
      id: string;
      class: LoopDryRunNode["class"];
      kind: string;
    }>
  ).map(node => {
    const dependsOn = edges.filter(edge => edge.to === node.id).map(edge => edge.from);
    return {
      id: node.id,
      class: node.class,
      kind: node.kind,
      ...(dependsOn.length > 0 ? { depends_on: dependsOn } : {}),
    };
  });
}

/** Server-owned catalog read: the daemon filters, counts, and reports facets, not the client. */
function readCatalogPage(url: URL) {
  const q = (url.searchParams.get("q") ?? "").trim().toLowerCase();
  const kind = url.searchParams.get("kind");
  const category = url.searchParams.get("category");
  const status = url.searchParams.get("status");
  const loops = loopCatalogFixtures.filter(entry => {
    if (kind && loopKind(entry).replace("-", "_") !== kind) return false;
    if (category && loopCategory(entry) !== category) return false;
    if (status && entry.last_run?.status !== status) return false;
    if (q === "") return true;
    const haystack = [entry.name, entry.contract.goal, loopCategory(entry) ?? ""]
      .join(" ")
      .toLowerCase();
    return haystack.includes(q);
  });
  return {
    facets: {
      categories: tally(loopCatalogFixtures.map(entry => loopCategory(entry))),
      kinds: tally(loopCatalogFixtures.map(entry => loopKind(entry).replace("-", "_"))),
      statuses: tally(loopCatalogFixtures.map(entry => entry.last_run?.status ?? null)),
    },
    loops,
    page: { has_more: false, limit: 50, total: loops.length },
  };
}

function tally(values: readonly (string | null | undefined)[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const value of values) {
    if (!value) continue;
    counts[value] = (counts[value] ?? 0) + 1;
  }
  return counts;
}

const loopRunEventStreamEncoder = new TextEncoder();

function createLoopRunEventsStreamResponse(workspaceId: string, runId: string): Response {
  const generation = runDetailByRunId.get(runId)?.run.generation ?? 1;
  const frame = {
    id: `${runId}:storybook:1`,
    seq: 1,
    at: "2026-04-17T18:10:00Z",
    workspace_id: workspaceId,
    loop_run_id: runId,
    kind: "node_running",
    payload: { node_id: "execute_task", generation },
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
  compozyApiMock.get("/api/workspaces/{workspace_id}/loops", ({ request }) =>
    HttpResponse.json(readCatalogPage(new URL(request.url)))
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/loops", () =>
    HttpResponse.json({ loop: loopDetailByName.get("implement-tasks")! }, { status: 201 })
  ),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}", ({ params }) => {
    const detail = detailByName.get(String(params.name));
    if (!detail) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    return HttpResponse.json({ loop: detail });
  }),
  compozyApiMock.patch(
    "/api/workspaces/{workspace_id}/loops/{name}",
    async ({ params, request }) => {
      const detail = detailByName.get(String(params.name));
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
    }
  ),
  compozyApiMock.delete("/api/workspaces/{workspace_id}/loops/{name}", ({ params }) => {
    if (!definitionNames.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Loop not found: ${String(params.name)}` },
        { status: 404 }
      );
    }
    return new HttpResponse(null, { status: 204 });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/config", ({ params }) => {
    if (!definitionNames.has(String(params.name))) {
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
  compozyApiMock.put("/api/workspaces/{workspace_id}/loops/{name}/config", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Partial<LoopConfigUpdateRequest>;
    return HttpResponse.json({
      config: body.config ?? loopConfigFixture,
      effective_config: loopEffectiveConfigFixture,
    });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/annotations", () =>
    HttpResponse.json({ annotations: loopAnnotationsFixture })
  ),
  compozyApiMock.put(
    "/api/workspaces/{workspace_id}/loops/{name}/annotations",
    async ({ request }) => {
      const body = (await request
        .json()
        .catch(() => ({}))) as Partial<LoopAnnotationsUpdateRequest>;
      return HttpResponse.json({ annotations: body.annotations ?? loopAnnotationsFixture });
    }
  ),
  compozyApiMock.post(
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
        inputs?: Record<string, unknown>;
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
        const definition = loopDetailByName.get(name)?.definition;
        const inputs = resolveRunInputs(entry, body.inputs ?? {});
        return HttpResponse.json({
          dry_run: {
            loop_name: name,
            generation: 1,
            resolved_inputs: inputs.resolved,
            input_origins: inputs.origins,
            resolved_network_participation: resolvedParticipation,
            contract: entry.contract,
            materialized_contract: materializeContractFixture(entry.contract, inputs.resolved),
            nodes: previewNodes(definition),
            effective_config: {
              iteration_cap: 12,
              budget_tokens: 500_000,
              budget_wall_sec: 3_600,
              budget_on_exceeded: "halt",
              environment: { mode: "root" },
              fan_out_width: 4,
              gate_max_revisions: 3,
              human_gate_enabled: true,
              runtime_defaults: {
                worker: { provider: "openai", model: "gpt-5.4" },
                judge: { provider: "anthropic", model: "claude-sonnet-4" },
              },
              runtime_rules: [
                {
                  match: { type: "implementation" },
                  runtime: { reasoning: "high" },
                },
              ],
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
            inputs: resolveRunInputs(entry, body.inputs ?? {}).resolved,
            resolved_network_participation: resolvedParticipation,
          },
        },
        { status: 201 }
      );
    }
  ),
  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/loops/{name}/validate",
    async ({ request }) => {
      const body = (await request.json().catch(() => ({}))) as {
        definition?: { graph?: { nodes?: unknown[]; edges?: unknown[] }; contract?: unknown };
      };
      const errors = lintDefinition(body.definition);
      return HttpResponse.json({
        valid: errors.length === 0,
        errors: errors satisfies LoopValidationIssue[],
      });
    }
  ),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs", ({ request }) => {
    const url = new URL(request.url);
    const loop = url.searchParams.get("loop");
    const status = url.searchParams.get("status");
    const runs = runListFixtures.filter(run => {
      if (loop && run.loop_name !== loop) return false;
      if (status && run.status !== status) return false;
      return true;
    });
    return HttpResponse.json({ runs, aggregates: loopRunAggregatesFixture });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", ({ params }) => {
    const detail = runDetailByRunId.get(String(params.run_id));
    if (!detail) {
      return HttpResponse.json(
        { error: `Loop run not found: ${String(params.run_id)}` },
        { status: 404 }
      );
    }
    return HttpResponse.json(detail);
  }),
  // The run read layer: three computed projections over one source (ADR-005).
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing", ({ params }) =>
    HttpResponse.json(resolveLoopRunBriefing(String(params.run_id)))
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes",
    ({ params, request }) =>
      HttpResponse.json(
        resolveLoopRunRoster(String(params.run_id), new URL(request.url).searchParams)
      )
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/timeline",
    ({ params, request }) =>
      HttpResponse.json(
        resolveLoopRunTimeline(String(params.run_id), new URL(request.url).searchParams)
      )
  ),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns", ({ params }) => {
    const runId = String(params.run_id);
    if (!runDetailByRunId.has(runId)) {
      return HttpResponse.json({ error: `Loop run not found: ${runId}` }, { status: 404 });
    }
    return HttpResponse.json({ turns: [], next_after_seq: null });
  }),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/events",
    ({ params, response }) => {
      const workspaceId = String(params.workspace_id);
      const runId = String(params.run_id);
      if (!runDetailByRunId.has(runId)) {
        return HttpResponse.json({ error: `Loop run not found: ${runId}` }, { status: 404 });
      }
      return response.untyped(createLoopRunEventsStreamResponse(workspaceId, runId));
    }
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/approve", () =>
    HttpResponse.json({ ok: true })
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/pause", () =>
    HttpResponse.json({ ok: true })
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/resume", () =>
    HttpResponse.json({ ok: true })
  ),
  compozyApiMock.post("/api/workspaces/{workspace_id}/loop-runs/{run_id}/cancel", ({ params }) =>
    HttpResponse.json({ ok: true, run_id: String(params.run_id) })
  ),
  ...graphEngHandlers,
];
