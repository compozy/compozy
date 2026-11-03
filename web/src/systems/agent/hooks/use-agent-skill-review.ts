import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  decideAgentSkillReview,
  isSkillReviewStale,
  type AgentSkillReviewDecision,
  type AgentSkillReviewRef,
} from "../adapters/agent-skill-review-api";
import {
  skillAutomaticItems,
  skillReviewFiles,
  skillReviewState,
  skillReviewWhy,
} from "../lib/agent-skill-review";
import { agentKeys } from "../lib/query-keys";
import { agentSkillReviewOptions } from "../lib/query-options";

/**
 * One proposed skill and the operator's decision on it (S9). A decision names the digest that was
 * read; when the agent changed the contents since, the daemon refuses with `skill_review_stale`,
 * the review rereads, and `stale` stays on until the operator decides on the latest.
 */
export function useAgentSkillReview(ref: AgentSkillReviewRef | null) {
  const queryClient = useQueryClient();
  const [stale, setStale] = useState(false);
  const query = useQuery({
    ...agentSkillReviewOptions(ref ?? { id: "" }),
    enabled: Boolean(ref?.id),
  });
  const review = query.data;
  const mutation = useMutation({
    mutationFn: ({ decision, digest }: { decision: AgentSkillReviewDecision; digest: string }) =>
      decideAgentSkillReview(ref!, decision, digest),
    onSuccess: decided => {
      setStale(false);
      queryClient.setQueryData(agentKeys.skillReview(decided.id), decided);
    },
    onError: error => {
      if (!isSkillReviewStale(error) || !ref) return;
      setStale(true);
      void queryClient.invalidateQueries({ queryKey: agentKeys.skillReview(ref.id) });
    },
  });
  const files = review ? skillReviewFiles(review) : [];
  return {
    review,
    loading: query.isPending && Boolean(ref?.id),
    loadError: query.error,
    files,
    why: review ? skillReviewWhy(review) : undefined,
    automatic: review ? skillAutomaticItems(review, files) : [],
    state: review ? skillReviewState(review) : "pending",
    stale,
    deciding: mutation.isPending,
    decideError: mutation.error && !isSkillReviewStale(mutation.error) ? mutation.error : null,
    decide: (decision: AgentSkillReviewDecision) => {
      if (review) mutation.mutate({ decision, digest: review.digest });
    },
  };
}

export type AgentSkillReviewView = ReturnType<typeof useAgentSkillReview>;
