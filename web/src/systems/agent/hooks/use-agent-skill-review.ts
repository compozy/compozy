import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { skillKeys } from "@/systems/skill";

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
 * One proposed skill and the operator's decision on it (S9). `ref` is `null` until the caller
 * knows the instance's profile, so neither the read nor a decision goes out under the wrong one.
 * A decision names the digest that was read; when the agent changed the contents since, the daemon
 * refuses with `skill_review_stale` and the review rereads. Until the reread brings a different
 * digest the decision stays off, and only then does the view call it the latest version.
 */
export function useAgentSkillReview(ref: AgentSkillReviewRef | null) {
  const queryClient = useQueryClient();
  const [staleDigest, setStaleDigest] = useState<string | null>(null);
  const query = useQuery({
    ...agentSkillReviewOptions(ref ?? { id: "" }),
    enabled: Boolean(ref?.id),
  });
  const review = query.data;
  const mutation = useMutation({
    mutationFn: ({ decision, digest }: { decision: AgentSkillReviewDecision; digest: string }) =>
      decideAgentSkillReview(ref!, decision, digest),
    onSuccess: (decided, { decision }) => {
      setStaleDigest(null);
      queryClient.setQueryData(agentKeys.skillReview(decided.id, ref?.profile), decided);
      void queryClient.invalidateQueries({
        queryKey: agentKeys.instanceReports(decided.instance_id),
      });
      if (decision === "approve") void queryClient.invalidateQueries({ queryKey: skillKeys.all });
    },
    onError: (error, { digest }) => {
      if (!isSkillReviewStale(error) || !ref) return;
      setStaleDigest(digest);
      void queryClient.invalidateQueries({ queryKey: agentKeys.skillReview(ref.id, ref.profile) });
    },
  });
  const files = review ? skillReviewFiles(review) : [];
  const awaitingLatest = staleDigest !== null && review?.digest === staleDigest;
  return {
    review,
    loading: ref === null || (query.isPending && Boolean(ref.id)),
    loadError: query.error,
    files,
    why: review ? skillReviewWhy(review) : undefined,
    automatic: review ? skillAutomaticItems(review, files) : [],
    state: review ? skillReviewState(review) : "pending",
    /** The contents changed after the operator read them, and this is the newer version. */
    stale: staleDigest !== null && !awaitingLatest,
    deciding: mutation.isPending || query.isFetching || awaitingLatest,
    decideError: mutation.error && !isSkillReviewStale(mutation.error) ? mutation.error : null,
    decide: (decision: AgentSkillReviewDecision) => {
      if (review && !query.isFetching && !awaitingLatest) {
        mutation.mutate({ decision, digest: review.digest });
      }
    },
  };
}

export type AgentSkillReviewView = ReturnType<typeof useAgentSkillReview>;
