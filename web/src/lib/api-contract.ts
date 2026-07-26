import type { operations as aghOperations } from "@/generated/agh-openapi";
import type { operations as daemonOperations } from "@/generated/compozy-openapi";

type OperationResponses<Operation> = Operation extends { responses: infer Responses }
  ? Responses
  : never;

type OperationRequestBodyContent<RequestBody> = RequestBody extends {
  content: { "application/json": infer Body };
}
  ? Body
  : never;

type OperationRequest<Operation> = Operation extends { requestBody: infer RequestBody }
  ? OperationRequestBodyContent<RequestBody>
  : Operation extends { requestBody?: infer RequestBody }
    ? OperationRequestBodyContent<RequestBody>
    : never;

type OperationQueryParams<Operation> = Operation extends {
  parameters?: { query?: infer Query };
}
  ? Query
  : never;

type OperationPathParams<Operation> = Operation extends {
  parameters?: { path?: infer Path };
}
  ? Path
  : never;

type ContentBody<T> = T extends { content: { "application/json": infer Body } }
  ? Body
  : T extends { content: { "text/plain": infer Body } }
    ? Body
    : T extends { content: { "text/event-stream": infer Body } }
      ? Body
      : never;

type OperationResponseFor<
  Operations,
  Id extends keyof Operations,
  Status extends keyof OperationResponses<Operations[Id]>,
> = ContentBody<OperationResponses<Operations[Id]>[Status]>;

type OperationRequestBodyFor<Operations, Id extends keyof Operations> = OperationRequest<
  Operations[Id]
>;

type OperationQueryFor<Operations, Id extends keyof Operations> = OperationQueryParams<
  Operations[Id]
>;

type OperationPathFor<Operations, Id extends keyof Operations> = OperationPathParams<
  Operations[Id]
>;

export type OperationId = keyof aghOperations;

export type OperationResponse<
  Id extends OperationId,
  Status extends keyof OperationResponses<aghOperations[Id]>,
> = OperationResponseFor<aghOperations, Id, Status>;

export type OperationRequestBody<Id extends OperationId> = OperationRequestBodyFor<
  aghOperations,
  Id
>;

export type ReasoningEffort = NonNullable<
  OperationRequestBody<"createSession">["reasoning_effort"]
>;

const reasoningEffortMembership = {
  none: true,
  minimal: true,
  low: true,
  medium: true,
  high: true,
  xhigh: true,
  max: true,
} satisfies Record<ReasoningEffort, true>;

export function isReasoningEffort(value: string): value is ReasoningEffort {
  return Object.hasOwn(reasoningEffortMembership, value);
}

export type OperationQuery<Id extends OperationId> = OperationQueryFor<aghOperations, Id>;

export type OperationPath<Id extends OperationId> = OperationPathFor<aghOperations, Id>;

export type DaemonOperationId = keyof daemonOperations;

export type DaemonOperationResponse<
  Id extends DaemonOperationId,
  Status extends keyof OperationResponses<daemonOperations[Id]>,
> = OperationResponseFor<daemonOperations, Id, Status>;

export type DaemonOperationRequestBody<Id extends DaemonOperationId> = OperationRequestBodyFor<
  daemonOperations,
  Id
>;

export type DaemonOperationQuery<Id extends DaemonOperationId> = OperationQueryFor<
  daemonOperations,
  Id
>;

export type DaemonOperationPath<Id extends DaemonOperationId> = OperationPathFor<
  daemonOperations,
  Id
>;
