import { Link2 } from "lucide-react";

import { SkillExposePanel, useSkillExposures, type SkillPayload } from "@/systems/skill";
import { useActiveWorkspace } from "@/systems/workspace";

import { MarketplaceDetailRailCard } from "./marketplace-detail-shell";

/**
 * Where this skill is exposed for other tools to read.
 *
 * The card is absent — not disabled — for skills with no folder of their own,
 * because there is nothing an operator could do about it here.
 */
function MarketplaceSkillExposuresCard({ skill }: { skill: SkillPayload }) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const model = useSkillExposures(skill, activeWorkspaceId ?? "");
  if (!model.eligible) return null;
  return (
    <MarketplaceDetailRailCard
      data-testid="skill-exposures-card"
      icon={Link2}
      summary={
        model.expose.isPending
          ? "updating"
          : model.exposures.length > 0
            ? String(model.exposures.length)
            : "not exposed"
      }
      title="Exposures"
    >
      <SkillExposePanel
        exposures={model.exposures}
        labelForTarget={model.labelForTarget}
        model={model.expose}
        onRetryTargets={model.retryTargets}
        targets={model.targets}
        targetsError={model.targetsError}
        targetsLoading={model.targetsLoading}
      />
    </MarketplaceDetailRailCard>
  );
}

export { MarketplaceSkillExposuresCard };
