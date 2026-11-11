import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  AgentInstanceApiError,
  pickAgentInstanceProposal,
  type AgentInstanceRef,
} from "../adapters/agent-instance-api";
import { invalidateAgentInstanceReads } from "./use-agent-instances";

export interface ProposalPickVariables {
  instance: AgentInstanceRef;
  reportId: string;
  proposal: number;
}

/** Why a pick failed, in words, with the daemon's code. */
const PICK_FAILURE: Record<string, string> = {
  proposal_already_picked: "This was already picked.",
  instance_not_active: "The agent is paused, so it can't take this now.",
  instance_no_main_session: "The agent has no main chat to take this.",
  session_input_queue_full: "The agent is busy; pick it when ready.",
  report_not_found: "This report is no longer kept.",
};

export function proposalPickFailure(error: unknown) {
  const code = error instanceof AgentInstanceApiError ? error.code : "";
  const words = PICK_FAILURE[code] ?? "Couldn't pick this. Try again.";
  return { code, words };
}

/** One operator pick through the daemon; the report rereads to show it. */
export function useProposalPick() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ instance, reportId, proposal }: ProposalPickVariables) =>
      pickAgentInstanceProposal(instance, reportId, proposal),
    onSettled: (_data, _error, { instance }) =>
      invalidateAgentInstanceReads(queryClient, instance.id),
  });
}
