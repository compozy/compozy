import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { resolveDiffFixture } from "./fixture-graph-eng-diff";
import {
  GRAPH_ENG_FORK_RUN_ID,
  GRAPH_ENG_RUN_ID,
  graphEngPendingRequests,
  graphEngRequestsByNode,
  graphEngResolvedRequests,
} from "./fixture-graph-eng-requests";
import { releaseTrainForkRun } from "./fixture-graph-eng-runs";

function reason(code: string, message: string, details?: Record<string, string>) {
  return { error: message, code, ...(details ? { details } : {}) };
}

function requestKey(runId: string, nodeId: string, itemIndex: number): string {
  return `${runId}:${nodeId}:${itemIndex}`;
}

export const graphEngHandlers: HttpHandler[] = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-requests", ({ request }) => {
    const url = new URL(request.url);
    const state = url.searchParams.get("state") ?? "pending";
    const runId = url.searchParams.get("run_id");
    const source = state === "resolved" ? graphEngResolvedRequests : graphEngPendingRequests;
    const items = runId ? source.filter(entry => entry.loop_run_id === runId) : source;
    return HttpResponse.json({
      items,
      aggregates: { pending: graphEngPendingRequests.length },
      next_cursor: "",
    });
  }),

  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/request",
    ({ params, request }) => {
      const url = new URL(request.url);
      const itemIndex = Number(url.searchParams.get("item_index") ?? "0");
      const found = graphEngRequestsByNode.get(
        requestKey(String(params.run_id), String(params.node_id), itemIndex)
      );
      if (!found) {
        return HttpResponse.json(
          { error: `Loop request not found: ${String(params.node_id)}` },
          { status: 404 }
        );
      }
      return HttpResponse.json({
        ...found,
        context: {
          ...(typeof found.context === "object" && found.context !== null ? found.context : {}),
          plan: "The complete redacted context, not the bounded preview.",
        },
      });
    }
  ),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/respond",
    async ({ params, request }) => {
      const nodeId = String(params.node_id);
      const body = (await request.json().catch(() => ({}))) as {
        decision?: string;
        payload?: unknown;
        item_index?: number;
      };
      const payload =
        typeof body.payload === "object" && body.payload !== null
          ? (body.payload as Record<string, unknown>)
          : {};
      if (nodeId === "confirm-rollout") {
        const regions = payload.regions;
        if (!Array.isArray(regions) || regions.length === 0) {
          return HttpResponse.json(
            reason("request_validation_failed", "answer does not match the expected shape", {
              regions: "minItems: array must have at least 1 items",
            }),
            { status: 422 }
          );
        }
      }
      if (nodeId === "apply-migration" && body.decision === "approve" && body.item_index === 9) {
        return HttpResponse.json(
          reason("request_already_answered", "request already answered", {
            decision: "reject",
            actor_id: "release-bot",
          }),
          { status: 409 }
        );
      }
      return HttpResponse.json({
        ok: true,
        run_id: String(params.run_id),
        node_id: nodeId,
        decision: body.decision ?? "respond",
        state: "answered",
        provenance: {
          actor_kind: "operator",
          actor_id: "pedro",
          answered_at: "2026-08-17T09:12:00Z",
        },
      });
    }
  ),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/amend",
    async ({ params, request }) => {
      const nodeId = String(params.node_id);
      const body = (await request.json().catch(() => ({}))) as {
        payload?: unknown;
        reason?: string;
        item_index?: number;
      };
      const payload =
        typeof body.payload === "object" && body.payload !== null
          ? (body.payload as Record<string, unknown>)
          : {};
      if (typeof payload.risk !== "string") {
        return HttpResponse.json(
          reason("request_validation_failed", "payload does not match the node's output shape", {
            risk: "required: missing property 'risk'",
          }),
          { status: 422 }
        );
      }
      return HttpResponse.json({
        ok: true,
        amendment: {
          loop_run_id: String(params.run_id),
          generation: 3,
          node_id: nodeId,
          item_index: body.item_index ?? 0,
          amendment_seq: 1,
          actor_kind: "operator",
          actor_id: "pedro",
          reason: body.reason ?? "",
          created_at: "2026-08-17T09:22:00Z",
          original: { risk: "high" },
          amended: payload,
        },
      });
    }
  ),

  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/diff",
    ({ params, request }) => {
      const url = new URL(request.url);
      const diff = resolveDiffFixture(String(params.run_id), url.searchParams);
      if (!diff) {
        return HttpResponse.json(
          reason("diff_cross_loop", "pick a generation or a run to compare against"),
          { status: 422 }
        );
      }
      return HttpResponse.json(diff);
    }
  ),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/rerun",
    async ({ params, request }) => {
      const body = (await request.json().catch(() => ({}))) as {
        from_node?: string;
        reason?: string;
      };
      const fromNode = body.from_node ?? "";
      if (fromNode === "confirm-rollout") {
        return HttpResponse.json(
          reason("rerun_node_unsettled", "node is still parked on a human request", {
            actual_state: "parked",
          }),
          { status: 422 }
        );
      }
      if (String(params.run_id) === GRAPH_ENG_FORK_RUN_ID) {
        return HttpResponse.json(
          reason("rerun_busy", "a generation is already in flight", { actual_state: "running" }),
          { status: 409 }
        );
      }
      return HttpResponse.json({
        run_id: String(params.run_id),
        generation: 4,
        parent_generation: 3,
        rerun_nodes: [fromNode, "collect-rollout", "render-notes"],
        carried: 5,
      });
    }
  ),

  compozyApiMock.post(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/fork",
    async ({ params, request }) => {
      const body = (await request.json().catch(() => ({}))) as {
        generation?: number;
        inputs?: Record<string, unknown>;
      };
      if (body.generation === undefined || body.generation > 3) {
        return HttpResponse.json(
          reason("fork_generation_unknown", "that generation is not available on this run"),
          { status: 404 }
        );
      }
      const services = body.inputs?.services;
      if (Array.isArray(services) && services.length === 0) {
        return HttpResponse.json(
          reason("input_validation_failed", "inputs do not satisfy the loop's declared shape", {
            services: "minItems: array must have at least 1 items",
          }),
          { status: 422 }
        );
      }
      return HttpResponse.json(
        {
          run: {
            ...releaseTrainForkRun,
            inputs: body.inputs ?? releaseTrainForkRun.inputs,
            forked_from: {
              run_id: String(params.run_id) || GRAPH_ENG_RUN_ID,
              generation: body.generation,
            },
          },
        },
        { status: 201 }
      );
    }
  ),
];
