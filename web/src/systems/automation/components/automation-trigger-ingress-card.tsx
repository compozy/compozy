import { Section } from "@compozy/ui";

import type { AutomationTrigger } from "../types";
import { GatewayIngressStatus } from "@/systems/gateway";

/**
 * The public delivery URL a sender should post to, with the truth about whether
 * it currently answers.
 *
 * The local webhook path is shown elsewhere; this section is only about public
 * reachability, so it stays honest when public ingress is off instead of
 * showing a URL that would go nowhere.
 */
export function AutomationTriggerIngressCard({ trigger }: { trigger: AutomationTrigger }) {
  if (trigger.event !== "webhook") return null;
  return (
    <Section label="Public delivery">
      <GatewayIngressStatus
        data-testid="automation-trigger-ingress"
        ingress={trigger.ingress}
        subject="this trigger"
      />
    </Section>
  );
}
