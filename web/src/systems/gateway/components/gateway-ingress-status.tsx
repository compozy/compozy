import { MonoId } from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import { DELIVERY_DURABILITY_NOTE, ingressReachabilityCopy } from "../lib/gateway-copy";
import type { GatewayIngressBinding } from "../types";
import { GatewayStatusChip } from "./gateway-status-chip";

export interface GatewayIngressStatusProps {
  ingress: GatewayIngressBinding | null | undefined;
  /** What the URL would deliver to, e.g. "this trigger" or "this bridge". */
  subject: string;
  "data-testid"?: string;
}

/**
 * Honest delivery reachability for one webhook trigger or bridge instance.
 *
 * The URL is shown only when the daemon actually published one, and it is never
 * shown without the reachability state beside it: a URL that exists is not a
 * URL that answers. When public ingress is off there is no URL at all, only the
 * path to turning it on.
 */
export function GatewayIngressStatus({
  ingress,
  subject,
  "data-testid": testId,
}: GatewayIngressStatusProps) {
  if (!ingress) {
    return (
      <p className="text-form-label text-muted" data-testid={testId}>
        Public delivery ingress is off, so {subject} has no public delivery URL.{" "}
        <GatewaySettingsLink />
      </p>
    );
  }

  const copy = ingressReachabilityCopy(ingress.reachability);
  const url = ingress.url?.trim();

  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      <div className="flex flex-wrap items-center gap-2">
        {/* The detail is already visible beside the chip, so it is not repeated
            as screen-reader-only text. */}
        <GatewayStatusChip label={copy.label} tone={copy.tone} />
        <span className="text-form-label text-muted">{copy.detail}</span>
      </div>
      {url ? (
        <MonoId copy copyLabel="Copy delivery URL" preserveCase value={url} />
      ) : (
        <p className="text-form-label text-muted">
          {ingress.enable_path?.trim()
            ? "No verified public address is published yet. "
            : "No verified public address is published yet, so there is no delivery URL to hand out."}
          {ingress.enable_path?.trim() ? <GatewaySettingsLink /> : null}
        </p>
      )}
      {url ? <p className="text-form-label text-muted">{DELIVERY_DURABILITY_NOTE}</p> : null}
    </div>
  );
}

function GatewaySettingsLink() {
  return (
    <Link className="text-accent underline-offset-4 hover:underline" to="/settings/gateway">
      Open Gateway settings to publish one.
    </Link>
  );
}
