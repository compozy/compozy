import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { skillKeys } from "@/systems/skill";

import {
  decideAgentSkillReview,
  isSkillReviewBaseStale,
  isSkillReviewInvalid,
  isSkillReviewRevertConflict,
  isSkillReviewStale,
  skillInstallConflictOp,
  AgentSkillReviewApiError,
  type AgentSkillReviewDecision,
  type AgentSkillReviewRef,
} from "../adapters/agent-skill-review-api";
import {
  skillAutomaticItems,
  skillAutomaticSummary,
  skillReviewFiles,
  skillReviewState,
  skillReviewWhy,
  skillRevertRefusedMessage,
} from "../lib/agent-skill-review";
import { agentKeys } from "../lib/query-keys";
import { agentSkillReviewOptions } from "../lib/query-options";
import type { AgentSkillReviewPayload } from "../types";
import { useUnavailableEvidenceSessions } from "./use-agent-skill-evidence";

const NO_EVIDENCE_SESSIONS: readonly string[] = [];

/**
 * One proposed skill and the operator's decision on it (S9). `ref` is `null` until the caller
 * knows the instance's profile, so neither the read nor a decision goes out under the wrong one.
 * A decision names the digest that was read; when the agent changed the contents since, the daemon
 * refuses with `skill_review_stale` and the review rereads. The decision waits for the reread, and
 * only a different digest is called the latest version. A draft CompozyOS can't install
 * (`skill_review_invalid`), or an edit whose base snapshot can't be read
 * (`skill_review_snapshot_invalid`), keeps Approve off and Reject available. A refused revert reads
 * in product copy by its reason and rereads the review. A skill left with two versions
 * (`skill_install_conflict`) names the operation to resolve.
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
      if (decision === "reject") return;
      // A new approval or a revert changes which review of the skill can be reverted, and the catalog.
      void queryClient.invalidateQueries({ queryKey: agentKeys.skillReviews() });
      void queryClient.invalidateQueries({ queryKey: skillKeys.all });
    },
    onError: (error, { digest }) => {
      // A refused revert or a stale digest rereads the review; only a stale one remembers the digest.
      const stale = isSkillReviewStale(error);
      if (!ref || !(stale || isSkillReviewRevertConflict(error))) return;
      if (stale) setStaleDigest(digest);
      void queryClient.invalidateQueries({ queryKey: agentKeys.skillReview(ref.id, ref.profile) });
    },
  });
  const unavailableEvidenceSessionIds = useUnavailableEvidenceSessions(
    review?.evidence_session_ids ?? NO_EVIDENCE_SESSIONS
  );
  const files = review ? skillReviewFiles(review) : [];
  const { unreadable, snapshotInvalid, invalid } = reviewProblems(review, mutation.error);
  const installConflictOp = skillInstallConflictOp(mutation.error);
  return {
    review,
    loading: ref === null || (query.isPending && Boolean(ref.id)),
    loadError: query.error,
    files,
    why: review ? skillReviewWhy(review) : undefined,
    automatic: review ? skillAutomaticItems(review, files) : [],
    /** Edit reviews: proposed and installed "Runs automatically" counts. */
    automaticComparison: automaticComparison(review),
    /** Evidence sessions the daemon no longer knows: shown as unavailable, not as links. */
    unavailableEvidenceSessionIds,
    state: review ? skillReviewState(review) : "pending",
    /** The detail says this approval can be reverted now; the daemon stays authoritative. */
    revertAllowed: review?.revert?.allowed === true,
    /** A stale refusal brought a different digest: this is the newer version. The decision waits for the reread. */
    stale: staleDigest !== null && review?.digest !== staleDigest,
    deciding: mutation.isPending || query.isFetching,
    /** This version can't be installed: Approve is off, Reject stays available. */
    invalid,
    /** CompozyOS can't read the draft at all: there is nothing to show, only Reject. */
    unreadable,
    /** The skill changed while it was being replaced; the operator resolves this op from the CLI. */
    installConflictOp,
    decideError: decisionError(mutation.error, installConflictOp, snapshotInvalid),
    decide: (decision: AgentSkillReviewDecision) => {
      if (review && !query.isFetching && !(invalid && decision === "approve")) {
        mutation.mutate({ decision, digest: review.digest });
      }
    },
  };
}

/** Why this version can only be rejected, from the review and the last refused decision. */
function reviewProblems(review: AgentSkillReviewPayload | undefined, error: Error | null) {
  // The daemon marks a review whose stored draft no longer verifies; it can only be rejected.
  const unreadable = review?.problem === "skill_review_invalid";
  // A pending edit whose base snapshot can't be read can't be reviewed, so it can only be rejected.
  const snapshotInvalid =
    review?.problem === "skill_review_snapshot_invalid" && skillReviewState(review) === "pending";
  // An edit whose installed skill changed since the proposal can only be rejected.
  const invalid =
    unreadable || snapshotInvalid || isSkillReviewBaseStale(error) || isSkillReviewInvalid(error);
  return { unreadable, snapshotInvalid, invalid };
}

/** Edit reviews: the proposed and installed "Runs automatically" counts. */
function automaticComparison(review: AgentSkillReviewPayload | undefined) {
  const installed = review?.kind === "edit" ? review.base_runs_automatically : undefined;
  if (!review || !installed) return undefined;
  return {
    proposed: skillAutomaticSummary(review.runs_automatically),
    installed: skillAutomaticSummary(installed),
  };
}

/** What the card or viewer says about a refused decision, in product copy; null when nothing applies. */
function decisionError(
  error: Error | null,
  installConflictOp: string | null,
  snapshotInvalid: boolean
) {
  if (isSkillReviewBaseStale(error)) {
    return new Error("This skill changed after the proposal. Reject it and ask for a new one.");
  }
  if (isSkillReviewInvalid(error)) {
    return new Error(
      "CompozyOS can't install this version. Reject it, or wait for the agent to propose it again."
    );
  }
  if (isSkillReviewRevertConflict(error)) {
    return new Error(
      skillRevertRefusedMessage(error instanceof AgentSkillReviewApiError ? error.reason : "")
    );
  }
  if (error && !isSkillReviewStale(error) && installConflictOp === null) return error;
  if (snapshotInvalid) {
    return new Error(
      "The installed contents this change was based on can't be read. Reject it and ask for a new proposal."
    );
  }
  return null;
}

export type AgentSkillReviewView = ReturnType<typeof useAgentSkillReview>;
