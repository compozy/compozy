/** S9: the container that reads a proposed skill's review and hands it to the card view. */
import { Skeleton } from "@compozy/ui";

import { useAgentInstancesState, useAgentSkillReview } from "@/systems/agent";

import type { ReportAgent } from "../../../lib/session-report-identity";
import { Notice } from "./report-entry";
import { SkillReviewCardView } from "./skill-review-card-view";

export function SkillReviewCard({
  agent,
  when,
  reviewId,
  instanceId,
}: {
  agent: ReportAgent;
  when: string;
  reviewId: string;
  /** The instance that proposed it: the review route answers only under its profile. */
  instanceId: string;
}) {
  const { instances, settled } = useAgentInstancesState();
  const instance = instances.find(candidate => candidate.id === instanceId);
  // Wait for the instance read, so nothing goes out under the default profile by mistake.
  const view = useAgentSkillReview(
    settled ? { id: reviewId, profile: instance?.profile_name } : null
  );
  if (view.loading) return <Skeleton className="h-28 w-full rounded-lg" />;
  if (view.loadError || !view.review) {
    return (
      <Notice
        agent={agent}
        body="I proposed a skill, but it couldn't be loaded here. Try again in a moment."
        title="A skill is waiting for review"
        when={when}
      />
    );
  }
  return <SkillReviewCardView agent={agent} view={view} when={when} />;
}
