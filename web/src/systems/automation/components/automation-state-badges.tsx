import { Lock } from "lucide-react";

import { Pill } from "@compozy/ui";

import { automationSourceLabel } from "../lib/automation-formatters";
import type { AutomationSource } from "../types";

interface AutomationStateBadgesProps {
  enabled: boolean;
  source: AutomationSource;
}

/**
 * Exception-only badges for a job/trigger row: nothing for the common enabled,
 * user-created case; "Disabled" when off; a lock pill for managed sources.
 */
export function AutomationStateBadges({ enabled, source }: AutomationStateBadgesProps) {
  return (
    <>
      {enabled ? null : (
        <Pill data-testid="automation-disabled-badge" form="plain">
          <Pill.Dot tone="neutral" />
          Disabled
        </Pill>
      )}
      {source === "dynamic" ? null : (
        <Pill data-testid="automation-source-badge" size="xs">
          <Lock aria-hidden="true" className="size-3" />
          {automationSourceLabel(source)}
        </Pill>
      )}
    </>
  );
}
