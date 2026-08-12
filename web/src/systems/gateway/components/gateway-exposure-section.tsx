import { useState } from "react";
import { ShieldCheck } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@compozy/ui";

import type { GatewayExposureModel, GatewayExposureRow } from "../lib/gateway-exposure-model";
import { GatewayExposureRowItem } from "./gateway-exposure-row";
import { GatewayPublicConsentDialog } from "./gateway-public-consent-dialog";
import { SettingsGroup } from "@/systems/settings";

export interface GatewayExposureSectionProps {
  exposure: GatewayExposureModel;
  isSubmitting: boolean;
  error?: string | null;
  onSetSurface: (row: GatewayExposureRow, desired: boolean, consent: boolean) => void;
}

/**
 * The exposure ladder. Turning a rung off is immediate and unconditional;
 * turning public operator access on always routes through consent first,
 * because the daemon requires fresh consent on every enable.
 */
export function GatewayExposureSection({
  exposure,
  isSubmitting,
  error,
  onSetSurface,
}: GatewayExposureSectionProps) {
  const [consentRow, setConsentRow] = useState<GatewayExposureRow | null>(null);

  const handleToggle = (row: GatewayExposureRow, desired: boolean) => {
    if (desired && row.requiresConsent) {
      setConsentRow(row);
      return;
    }
    onSetSurface(row, desired, false);
  };

  const handleConsent = () => {
    if (!consentRow) return;
    onSetSurface(consentRow, true, true);
    setConsentRow(null);
  };

  return (
    <SettingsGroup data-testid="gateway-exposure-section" title="Reachability">
      {exposure.ceilingEnabled ? null : (
        <div className="border-t border-line-soft px-4 py-3 first:border-t-0">
          <Alert data-testid="gateway-exposure-ceiling" variant="warning">
            <ShieldCheck />
            <AlertTitle>Remote reachability is turned off in configuration</AlertTitle>
            <AlertDescription>
              Set <code>gateway.enabled</code> to true. The change applies immediately.
            </AlertDescription>
          </Alert>
        </div>
      )}
      {exposure.refusal ? (
        <div className="border-t border-line-soft px-4 py-3 first:border-t-0">
          <Alert data-testid="gateway-exposure-refusal" variant="danger">
            <AlertTitle>{exposure.refusal.cause}</AlertTitle>
            <AlertDescription>{exposure.refusal.fix}</AlertDescription>
          </Alert>
        </div>
      ) : null}
      {error && !consentRow ? (
        <div className="border-t border-line-soft px-4 py-3 first:border-t-0">
          <Alert data-testid="gateway-exposure-error" variant="danger">
            <AlertTitle>That change was not applied</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </div>
      ) : null}
      {exposure.rows.map(row => (
        <GatewayExposureRowItem
          disabled={isSubmitting}
          key={row.id}
          onToggle={handleToggle}
          row={row}
        />
      ))}
      <GatewayPublicConsentDialog
        error={null}
        isSubmitting={isSubmitting}
        onConfirm={handleConsent}
        onOpenChange={open => setConsentRow(open ? consentRow : null)}
        open={consentRow !== null}
      />
    </SettingsGroup>
  );
}
