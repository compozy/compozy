import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import type { OperationResponse } from "@/lib/api-contract";

type CoordinationPayload = OperationResponse<"getNetworkCoordination", 200>["coordination"];
type CoordinationMutationResponse = OperationResponse<"putNetworkCoordination", 200>;
type CoordinationConflictResponse = OperationResponse<"putNetworkCoordination", 409>;

interface CoordinationRef {
  scope: "workspace" | "task";
  taskId?: string;
  runId?: string;
}

interface CoordinationState {
  enabled: boolean;
  revision: number;
  dismissed: boolean;
  updatedAt?: string;
}

interface CoordinationMutationBody {
  scope?: string;
  task_id?: string;
  run_id?: string;
  enabled?: boolean;
  dismissed?: boolean;
  expected_revision?: number;
}

const coordinationState = new Map<string, CoordinationState>();

function refFromRequest(request: Request): CoordinationRef {
  const query = new URL(request.url).searchParams;
  return {
    scope: query.get("scope") === "task" ? "task" : "workspace",
    taskId: query.get("task_id") ?? undefined,
    runId: query.get("run_id") ?? undefined,
  };
}

function refFromBody(body: CoordinationMutationBody): CoordinationRef {
  return {
    scope: body.scope === "task" ? "task" : "workspace",
    taskId: body.task_id,
    runId: body.run_id,
  };
}

function stateKey(workspaceId: string, ref: CoordinationRef): string {
  return [workspaceId, ref.scope, ref.taskId ?? ""].join(":");
}

function currentState(workspaceId: string, ref: CoordinationRef): CoordinationState {
  return (
    coordinationState.get(stateKey(workspaceId, ref)) ?? {
      enabled: false,
      revision: 0,
      dismissed: false,
    }
  );
}

function coordinationPayload(
  workspaceId: string,
  ref: CoordinationRef,
  state: CoordinationState
): CoordinationPayload {
  const eligible =
    ref.scope === "task" &&
    Boolean(ref.taskId) &&
    Boolean(ref.runId) &&
    !state.enabled &&
    !state.dismissed;
  return {
    workspace_id: workspaceId,
    scope: ref.scope,
    ...(ref.taskId ? { task_id: ref.taskId } : {}),
    enabled: state.enabled,
    revision: state.revision,
    ...(state.updatedAt ? { updated_at: state.updatedAt, updated_by: "local-user" } : {}),
    invitation: {
      scope: ref.scope,
      ...(ref.taskId ? { task_id: ref.taskId } : {}),
      dismissed: state.dismissed,
    },
    eligibility: {
      eligible,
      reason: eligible
        ? "eligible"
        : state.enabled
          ? "already_enabled"
          : state.dismissed
            ? "dismissed"
            : "run_required",
      ...(ref.runId ? { run_id: ref.runId } : {}),
      coordinator: eligible,
      worker_count: eligible ? 2 : 0,
    },
  };
}

function mutateCoordination(
  workspaceId: string,
  ref: CoordinationRef,
  body: CoordinationMutationBody
): HttpResponse<CoordinationMutationResponse> | HttpResponse<CoordinationConflictResponse> {
  const current = currentState(workspaceId, ref);
  if (body.expected_revision !== current.revision) {
    return HttpResponse.json<CoordinationConflictResponse>(
      { error: "network coordination revision conflict" },
      { status: 409 }
    );
  }
  const next = {
    ...current,
    ...(typeof body.enabled === "boolean" ? { enabled: body.enabled } : {}),
    ...(typeof body.dismissed === "boolean" ? { dismissed: body.dismissed } : {}),
    revision: current.revision + 1,
    updatedAt: "2026-04-17T10:05:00Z",
  };
  coordinationState.set(stateKey(workspaceId, ref), next);
  return HttpResponse.json<CoordinationMutationResponse>({
    coordination: coordinationPayload(workspaceId, ref, next),
  });
}

export const coordinationHandlers: HttpHandler[] = [
  aghApiMock.get("/api/workspaces/{workspace_id}/network-coordination", ({ params, request }) => {
    const workspaceId = String(params.workspace_id);
    const ref = refFromRequest(request);
    return HttpResponse.json({
      coordination: coordinationPayload(workspaceId, ref, currentState(workspaceId, ref)),
    });
  }),
  aghApiMock.put(
    "/api/workspaces/{workspace_id}/network-coordination",
    async ({ params, request }) => {
      const body = (await request.json()) as CoordinationMutationBody;
      return mutateCoordination(String(params.workspace_id), refFromBody(body), body);
    }
  ),
  aghApiMock.put(
    "/api/workspaces/{workspace_id}/network-coordination/invitation",
    async ({ params, request }) => {
      const body = (await request.json()) as CoordinationMutationBody;
      return mutateCoordination(String(params.workspace_id), refFromBody(body), body);
    }
  ),
];
