import { AlertCircle, ArrowRightLeft } from "lucide-react";
import { type ReactNode, use } from "react";

import { Button, Marker, MarkerMeta } from "@compozy/ui";

import { SessionContinueContext } from "../contexts/session-continue-context-value";
import { formatMessageTimestamp } from "../lib/format-timestamp";
import type { ProviderErrorView } from "../lib/provider-error";
import { ClusterCount } from "./marker-cluster-count";

const PROVIDER_ERROR_CAUSE: Record<ProviderErrorView["code"], string> = {
  provider_auth_required:
    "This turn stopped because the provider rejected the request as not authenticated.",
  provider_rate_limited: "This turn stopped because the provider is limiting requests right now.",
};

// Public CLI invocation shown to the operator as the next step; it carries no credential.
const PROVIDER_STATUS_CLI_HINT = "compozy provider auth status <provider> --remote";

// One next step per daemon next_action; `inspect` doubles as the fallback for unknown values.
const PROVIDER_ERROR_NEXT_STEP: Record<ProviderErrorView["nextAction"], ReactNode> = {
  login: (
    <>
      Sign in with the provider CLI, run{" "}
      <code className="font-mono" data-testid="provider-error-command">
        {PROVIDER_STATUS_CLI_HINT}
      </code>{" "}
      to confirm through the daemon, then send your message again.
    </>
  ),
  bind_secret:
    "Update the provider's bound credential in Settings → Providers, then send your message again.",
  inspect:
    "Check the provider's configuration in Settings → Providers, then send your message again.",
  retry: "Wait for the provider to recover, then send your message again.",
  handoff: "Continue this session with another agent or route.",
};

function providerErrorSubject(view: ProviderErrorView): string {
  if (view.code === "provider_rate_limited") {
    return `${view.provider} is rate limited`;
  }
  switch (view.nextAction) {
    case "login":
      return `${view.provider} needs sign-in`;
    case "bind_secret":
      return `${view.provider} credential needs updating`;
    default:
      return `${view.provider} authentication failed`;
  }
}

function providerErrorOccurrence(view: ProviderErrorView): string | null {
  if (view.occurrenceCount <= 1) {
    return null;
  }
  const since = view.firstSeenAt ? formatMessageTimestamp(Date.parse(view.firstSeenAt)) : "";
  return since ? `${view.occurrenceCount} times since ${since}` : `${view.occurrenceCount} times`;
}

/**
 * Turn-level provider failure the session survives: same danger marker anatomy as a
 * session failure, but the sentence names the provider and the daemon's next action.
 * A `handoff` step offers the Continue dialog for this session — an offer, never an
 * automatic switch — when a dialog host is mounted around the transcript.
 */
export function ProviderErrorNotice({ view, count }: { view: ProviderErrorView; count: number }) {
  const occurrence = providerErrorOccurrence(view);
  const requestContinue = use(SessionContinueContext);
  return (
    <Marker
      role="alert"
      data-testid="session-error-notice"
      data-provider-error={view.code}
      data-provider-next-action={view.nextAction}
      tone="danger"
      icon={<AlertCircle strokeWidth={1.8} />}
    >
      <b data-testid="provider-error-subject">{providerErrorSubject(view)}</b> —{" "}
      <span data-testid="session-error-detail">
        {PROVIDER_ERROR_CAUSE[view.code]} {PROVIDER_ERROR_NEXT_STEP[view.nextAction]}
      </span>
      {occurrence ? (
        <>
          {" "}
          <MarkerMeta data-testid="provider-error-occurrence">{occurrence}</MarkerMeta>
        </>
      ) : null}
      <ClusterCount count={count} />
      {view.nextAction === "handoff" && requestContinue ? (
        <span className="mt-1 flex">
          <Button
            className="text-muted hover:text-fg"
            data-testid="provider-error-continue"
            onClick={() => requestContinue()}
            size="xs"
            type="button"
            variant="ghost"
          >
            <ArrowRightLeft aria-hidden="true" />
            Continue with another agent…
          </Button>
        </span>
      ) : null}
    </Marker>
  );
}
