/** S9: the container that reads a proposed skill's review and hands it to the card view. */
import { Skeleton } from "@compozy/ui";

import { useAgentSkillReview } from "@/systems/agent";

import type { ReportAgent } from "../../../lib/session-report-identity";
import { Notice } from "./report-entry";
import { SkillReviewCardView } from "./skill-review-card-view";

export function SkillReviewCard({
  agent,
  when,
  reviewId,
  profile,
}: {
  agent: ReportAgent;
  when: string;
  reviewId: string;
  profile?: string;
}) {
  const view = useAgentSkillReview({ id: reviewId, profile });
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
