import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import { createNetworkChannelDraft } from "../lib/network-formatters";
import type {
  CreateNetworkChannelRequest,
  CreateNetworkChannelResponse,
  NetworkFanoutPolicy,
} from "../types";

interface CreateChannelRequestFence {
  generation: number;
  requestId: number;
  workspaceId: string;
}

interface NetworkCreateChannelContext {
  dialogWorkspaceId: string | null;
  draft: ReturnType<typeof createNetworkChannelDraft>;
  generation: number;
  open: boolean;
  pendingRequest: CreateChannelRequestFence | null;
  requestCount: number;
}

interface NavigateToChannelInput {
  channel: string;
  workspaceId: string;
}

type NetworkCreateChannelEventPayloadMap = {
  agentSelectionChanged: { selectedAgentNames: string[] };
  channelCreationFailed: CreateChannelRequestFence & { error: string };
  channelCreationRequested: {
    create: (request: CreateNetworkChannelRequest) => Promise<CreateNetworkChannelResponse>;
    navigate: (input: NavigateToChannelInput) => Promise<void>;
    workspaceId: string;
  };
  channelCreationSucceeded: CreateChannelRequestFence & {
    channel: string;
    navigate: (input: NavigateToChannelInput) => Promise<void>;
  };
  channelNameEntered: { channelName: string };
  dialogClosed: {};
  dialogOpened: { workspaceId: string };
  fanoutPolicyChanged: { fanoutPolicy: NetworkFanoutPolicy };
  lifecycleDisposed: {};
  navigationFailed: CreateChannelRequestFence & { error: string };
  purposeEntered: { purpose: string };
};

type NetworkCreateChannelEnqueue = EnqueueObject<
  NetworkCreateChannelContext,
  never,
  NetworkCreateChannelEventPayloadMap
>;

function hasRequestFence(
  request: CreateChannelRequestFence | null,
  event: CreateChannelRequestFence
): request is CreateChannelRequestFence {
  return (
    request?.generation === event.generation &&
    request.requestId === event.requestId &&
    request.workspaceId === event.workspaceId
  );
}

export const networkCreateChannelLogic = createStoreLogic<
  NetworkCreateChannelContext,
  NetworkCreateChannelEventPayloadMap
>({
  context: (): NetworkCreateChannelContext => ({
    dialogWorkspaceId: null,
    draft: createNetworkChannelDraft(),
    generation: 0,
    open: false,
    pendingRequest: null,
    requestCount: 0,
  }),
  on: {
    agentSelectionChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, selectedAgentNames: event.selectedAgentNames },
    }),
    channelCreationFailed: (context, event, enqueue) => {
      if (!hasRequestFence(context.pendingRequest, event)) return;
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return { ...context, pendingRequest: null };
    },
    channelCreationRequested: (context, event, enqueue) => {
      if (
        !context.open ||
        context.dialogWorkspaceId !== event.workspaceId ||
        context.pendingRequest !== null ||
        context.draft.channelName.trim() === "" ||
        context.draft.purpose.trim() === "" ||
        context.draft.selectedAgentNames.length === 0
      ) {
        return;
      }
      const requestId = context.requestCount + 1;
      const request = {
        generation: context.generation,
        requestId,
        workspaceId: event.workspaceId,
      };
      enqueueCreateChannel(context, event, enqueue, request);
      return { ...context, pendingRequest: request, requestCount: requestId };
    },
    channelCreationSucceeded: (context, event, enqueue) => {
      if (!hasRequestFence(context.pendingRequest, event)) return;
      enqueueNavigation(event, enqueue);
      return resetContext(context, context.generation + 1);
    },
    channelNameEntered: (context, event) => ({
      ...context,
      draft: { ...context.draft, channelName: event.channelName },
    }),
    dialogClosed: context => ({ ...context, open: false }),
    dialogOpened: (context, event) => {
      if (context.dialogWorkspaceId === event.workspaceId) {
        return { ...context, open: true };
      }
      return {
        ...resetContext(context, context.generation + 1),
        dialogWorkspaceId: event.workspaceId,
        open: true,
      };
    },
    fanoutPolicyChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, fanoutPolicy: event.fanoutPolicy },
    }),
    lifecycleDisposed: context => resetContext(context, context.generation + 1),
    navigationFailed: (context, event, enqueue) => {
      if (context.generation !== event.generation + 1) return;
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
    },
    purposeEntered: (context, event) => ({
      ...context,
      draft: { ...context.draft, purpose: event.purpose },
    }),
  },
});

function enqueueCreateChannel(
  context: NetworkCreateChannelContext,
  event: NetworkCreateChannelEventPayloadMap["channelCreationRequested"],
  enqueue: NetworkCreateChannelEnqueue,
  request: CreateChannelRequestFence
) {
  const input: CreateNetworkChannelRequest = {
    agent_names: context.draft.selectedAgentNames,
    channel: context.draft.channelName.trim(),
    fanout_policy: context.draft.fanoutPolicy,
    purpose: context.draft.purpose.trim(),
    workspace_id: event.workspaceId,
  };
  enqueue.effect(async ({ trigger }) => {
    try {
      const response = await event.create(input);
      trigger.channelCreationSucceeded({
        ...request,
        channel: response.channel.channel,
        navigate: event.navigate,
      });
    } catch (error) {
      trigger.channelCreationFailed({ ...request, error: describeCreateError(error) });
    }
  });
}

function enqueueNavigation(
  event: NetworkCreateChannelEventPayloadMap["channelCreationSucceeded"],
  enqueue: NetworkCreateChannelEnqueue
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await event.navigate({ channel: event.channel, workspaceId: event.workspaceId });
    } catch (error) {
      trigger.navigationFailed({ ...event, error: describeNavigationError(error) });
    }
  });
}

function resetContext(
  context: NetworkCreateChannelContext,
  generation: number
): NetworkCreateChannelContext {
  return {
    ...context,
    dialogWorkspaceId: null,
    draft: createNetworkChannelDraft(),
    generation,
    open: false,
    pendingRequest: null,
  };
}

function describeCreateError(error: unknown): string {
  return error instanceof Error ? error.message : "Failed to create network channel";
}

function describeNavigationError(error: unknown): string {
  return error instanceof Error ? error.message : "Failed to open the created network channel";
}
