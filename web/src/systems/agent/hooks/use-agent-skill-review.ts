import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { skillKeys } from "@/systems/skill";

import {
  decideAgentSkillReview,
  isSkillReviewInvalid,
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
 * refuses with `skill_review_stale` and the review rereads. The decision waits for the reread, and
 * only a different digest is called the latest version. A draft CompozyOS can't install
 * (`skill_review_invalid`) keeps Approve off and Reject available.
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
  const invalid = isSkillReviewInvalid(mutation.error);
  return {
    review,
    loading: ref === null || (query.isPending && Boolean(ref.id)),
    loadError: query.error,
    files,
    why: review ? skillReviewWhy(review) : undefined,
    automatic: review ? skillAutomaticItems(review, files) : [],
    state: review ? skillReviewState(review) : "pending",
    /** A stale refusal brought a different digest: this is the newer version. The decision waits for the reread. */
    stale: staleDigest !== null && review?.digest !== staleDigest,
    deciding: mutation.isPending || query.isFetching,
    /** This version can't be installed: Approve is off, Reject stays available. */
    invalid,
    decideError: invalid
      ? new Error(
          "CompozyOS can't install this version. Reject it, or wait for the agent to propose it again."
        )
      : mutation.error && !isSkillReviewStale(mutation.error)
        ? mutation.error
        : null,
    decide: (decision: AgentSkillReviewDecision) => {
      if (review && !query.isFetching && !(invalid && decision === "approve")) {
        mutation.mutate({ decision, digest: review.digest });
      }
    },
  };
}

export type AgentSkillReviewView = ReturnType<typeof useAgentSkillReview>;
