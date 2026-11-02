import { PROFILE_IDENTITY_SWATCHES, useProfileReadScope } from "@/systems/profiles";
import type { RuntimeProviderOption } from "@/systems/runtime";

import type {
  AgentCreateDialogStore,
  AgentCreateFlow,
  AgentCreateOutcome,
} from "./agent-create-dialog-logic";
import { errorMessage } from "./agent-create-dialog-logic";
import { useCreateAgent, useDuplicateAgent } from "./use-agents";
import { useCreateAgentInstance } from "./use-agent-instances";
import {
  AGENT_PLACE_LATER,
  buildCreateAgentParams,
  buildDuplicateAgentParams,
  validateAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "../lib/agent-create-draft";
import type { AgentInstanceCapacity } from "../lib/agent-instance";
import type { AgentPayload } from "../types";

interface AgentCreateSubmitArgs {
  activeWorkspaceId: string | undefined;
  dialogStore: AgentCreateDialogStore;
  draft: AgentCreateDialogDraft;
  flow: AgentCreateFlow;
  instanceCapacity: AgentInstanceCapacity;
  providerOptions: RuntimeProviderOption[];
  providersError: string | null;
  providersLoading: boolean;
}

/** Sends the draft: create or duplicate, then the first instance when a Stays on agent can start. */
export function useAgentCreateSubmit({
  activeWorkspaceId,
  dialogStore,
  draft,
  flow,
  instanceCapacity,
  providerOptions,
  providersError,
  providersLoading,
}: AgentCreateSubmitArgs) {
  const { destination } = useProfileReadScope();
  const { mutateAsync: createAgent } = useCreateAgent();
  const { mutateAsync: duplicateAgent } = useDuplicateAgent();
  const { mutateAsync: createInstance } = useCreateAgentInstance();

  const validationContext = {
    hasActiveWorkspace: Boolean(activeWorkspaceId),
    providerOptions,
    providersError,
    providersLoading,
  };

  /** Starts the first instance, unless Decide later or a shown block says it can't start. */
  const startFirstInstance = async (agent: AgentPayload): Promise<AgentCreateOutcome> => {
    const provider = providerOptions.find(option => option.id === draft.provider);
    const skipped =
      draft.kind !== "stays_on" ||
      draft.place === AGENT_PLACE_LATER ||
      instanceCapacity.reached ||
      provider?.needs_auth === true;
    if (skipped) return { agent, landing: "agent" };
    try {
      await createInstance({ agent: agent.name, workspace: draft.place });
      return { agent, landing: "sessions" };
    } catch (cause) {
      return {
        agent,
        landing: "agent",
        startError: errorMessage(cause, "This agent couldn't start."),
      };
    }
  };

  const reject = (fallback: string) => {
    const { fields } = validateAgentCreateDraft(draft, validationContext);
    dialogStore.trigger.submissionRejected({
      error: Object.values(fields).find(field => field && field.length > 0) ?? fallback,
    });
  };

  const submitDuplicate = (source: AgentPayload) => {
    const params = buildDuplicateAgentParams(source, draft, activeWorkspaceId, validationContext);
    if (!params) return reject("Fix the highlighted fields before duplicating an agent.");
    dialogStore.trigger.submissionRequested({
      execute: async () =>
        startFirstInstance(
          await duplicateAgent({ sourceName: source.name, params, profile: destination })
        ),
      failureMessage: "Failed to duplicate agent.",
    });
  };

  const submitCreate = () => {
    const params = buildCreateAgentParams(
      draft,
      activeWorkspaceId,
      validationContext,
      PROFILE_IDENTITY_SWATCHES.map(swatch => swatch.value)
    );
    if (!params) return reject("Fix the highlighted fields before creating an agent.");
    dialogStore.trigger.submissionRequested({
      execute: async () => startFirstInstance(await createAgent({ params, profile: destination })),
      failureMessage: "Failed to create agent.",
    });
  };

  return () => {
    if (flow.status !== "editing" && flow.status !== "failed") return;
    if (flow.kind === "duplicate") submitDuplicate(flow.source);
    else submitCreate();
  };
}
