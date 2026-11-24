import { useState } from "react";
import { CheckCircle2, ExternalLink, RefreshCw, Terminal, TriangleAlert } from "@compozy/ui/icons";

import {
  ActionResultBanner,
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CodeBlock,
  CopyIconButton,
  ListingRow,
  SkeletonRows,
  StatusCard,
  toast,
} from "@compozy/ui";

import { ProviderLogo } from "@/systems/settings/components/provider-logo";
import { ProviderStatusLabel } from "@/systems/settings/components/provider-status-label";

import { providerLoginCommand } from "../lib/provider-view";
import type {
  ComputerFixture,
  ComputerProviderAuthFixture,
} from "../mocks/remote-computers-fixtures";
import {
  buildSshUpdateSteps,
  type UpdateRunState,
} from "../mocks/remote-computers-fixtures-lane-b";
import { StepProgressList, type StepProgressItem } from "./step-progress-list";

const PROVIDER_STATUS_VIEW: Record<
  ComputerProviderAuthFixture["status"],
  { label: string; tone: "success" | "warning" | "neutral" }
> = {
  ready: { label: "Ready", tone: "success" },
  signed_out: { label: "Signed out", tone: "warning" },
  not_installed: { label: "Not installed", tone: "neutral" },
};

export interface ComputerDetailsProvidersTabProps {
  computerName: string;
  providers: readonly ComputerProviderAuthFixture[];
  /** Optional provider id already showing the inline remote sign-in terminal preview. */
  initialSignInProviderId?: ComputerProviderAuthFixture["id"] | null;
  providersLoading?: boolean;
  providersError?: Error | null;
  onRetryProviders?: () => void;
  /** False hides `Sign in on <name>` when this device lacks `terminal:operate`. */
  canOperateTerminal?: boolean;
  signingInProviderId?: string | null;
  /** Launches a live remote terminal on `<computer>` running the provider's login command. */
  onSignInProvider?: (provider: ComputerProviderAuthFixture, command: string) => void;
}

/** Provider sign-in readiness scoped to `<name>` with `Sign in on <name>` remote terminal launch (US-013). */
export function ComputerDetailsProvidersTab({
  computerName,
  providers,
  initialSignInProviderId = null,
  providersLoading = false,
  providersError = null,
  onRetryProviders,
  canOperateTerminal = true,
  signingInProviderId = null,
  onSignInProvider,
}: ComputerDetailsProvidersTabProps) {
  const [activeSignInId, setActiveSignInId] = useState<ComputerProviderAuthFixture["id"] | null>(
    initialSignInProviderId
  );

  const activeProvider = providers.find(item => item.id === activeSignInId) ?? null;

  const triggerSignIn = (provider: ComputerProviderAuthFixture) => {
    const command = providerLoginCommand(provider);
    if (onSignInProvider) {
      onSignInProvider(provider, command);
      return;
    }
    setActiveSignInId(provider.id);
  };

  return (
    <div className="flex flex-col gap-4" data-slot="computer-details-providers-tab">
      <p className="text-small-body text-muted-foreground">
        Provider logins stay on <span className="font-medium text-fg">{computerName}</span> and are
        never copied to this computer.
      </p>

      {providersLoading && providers.length === 0 ? (
        <div
          aria-busy="true"
          className="rounded-lg border border-line bg-card p-3"
          data-testid="computer-providers-loading"
        >
          <SkeletonRows count={3} />
        </div>
      ) : providersError && providers.length === 0 ? (
        <Alert data-testid="computer-providers-error" variant="default">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>Couldn&apos;t load providers on {computerName}</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <span>{providersError.message}</span>
            {onRetryProviders ? (
              <div>
                <Button onClick={onRetryProviders} size="xs" type="button" variant="secondary">
                  <RefreshCw aria-hidden="true" />
                  Retry
                </Button>
              </div>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : providers.length === 0 ? (
        <div className="rounded-lg border border-line bg-card px-4 py-4 text-small-body text-muted-foreground">
          Provider sign-in status on <span className="font-medium text-fg">{computerName}</span> is
          managed on that computer.
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-line bg-card">
          {providers.map(provider => {
            const view = PROVIDER_STATUS_VIEW[provider.status];
            const isSigningIn = signingInProviderId === provider.id;
            return (
              <ListingRow
                className="min-h-14 border-b border-line-soft py-3 last:border-b-0"
                data-provider-id={provider.id}
                interactive={false}
                key={provider.id}
              >
                <ListingRow.Icon>
                  <ProviderLogo provider={provider.id} />
                </ListingRow.Icon>
                <ListingRow.Main>
                  <ListingRow.Name>
                    <ListingRow.Title>{provider.name}</ListingRow.Title>
                    <ProviderStatusLabel label={view.label} tone={view.tone} />
                  </ListingRow.Name>
                  <ListingRow.Meta className="mt-1">
                    {provider.status === "ready" ? (
                      <span>Signed in as {provider.accountLabel ?? "your account"}</span>
                    ) : provider.status === "signed_out" ? (
                      <span className="text-warning">
                        {canOperateTerminal
                          ? `Sessions using ${provider.name} fail until signed in on ${computerName}`
                          : `Turn on Terminals permission on ${computerName} to sign in remotely.`}
                      </span>
                    ) : (
                      <span>CLI not found on {computerName}</span>
                    )}
                  </ListingRow.Meta>
                </ListingRow.Main>
                <ListingRow.Trail>
                  {provider.status === "signed_out" && canOperateTerminal ? (
                    <Button
                      disabled={isSigningIn}
                      onClick={() => triggerSignIn(provider)}
                      size="sm"
                      type="button"
                      variant="secondary"
                    >
                      <Terminal aria-hidden="true" />
                      Sign in on {computerName}
                    </Button>
                  ) : provider.status === "ready" && canOperateTerminal ? (
                    <Button
                      disabled={isSigningIn}
                      onClick={() => triggerSignIn(provider)}
                      size="xs"
                      type="button"
                      variant="ghost"
                    >
                      Reconnect
                    </Button>
                  ) : null}
                </ListingRow.Trail>
              </ListingRow>
            );
          })}
        </div>
      )}

      {activeProvider && !onSignInProvider ? (
        <StatusCard
          className="border border-line bg-sunken/70 p-3.5 shadow-none"
          data-slot="remote-provider-signin-terminal"
          tone="neutral"
        >
          <StatusCard.Header label={`Remote terminal on ${computerName}`}>
            <Button
              className="ml-auto"
              onClick={() => setActiveSignInId(null)}
              size="xs"
              type="button"
              variant="ghost"
            >
              Close
            </Button>
          </StatusCard.Header>
          <StatusCard.Body className="flex flex-col gap-2.5">
            <CodeBlock
              code={`$ ${providerLoginCommand(activeProvider)}\nOpen https://auth.openai.com/device?code=K7M2-9QP4 to finish signing in on ${computerName}\nWaiting for browser sign-in…`}
              copyable={false}
              density="compact"
            />
            <div className="flex flex-wrap items-center gap-2">
              <Button
                onClick={() => toast.message("Opened sign-in link in your browser.")}
                size="xs"
                type="button"
                variant="secondary"
              >
                <ExternalLink aria-hidden="true" />
                Open browser
              </Button>
              <CopyIconButton
                copiedLabel="Sign-in link copied"
                copiedToastLabel="Sign-in link copied"
                copyLabel="Copy sign-in link"
                value="https://auth.openai.com/device?code=K7M2-9QP4"
              />
            </div>
          </StatusCard.Body>
        </StatusCard>
      ) : null}
    </div>
  );
}

export interface ComputerDetailsUpdateTabProps {
  computer: ComputerFixture;
  initialUpdateState?: UpdateRunState;
  liveUpdateState?: UpdateRunState;
  liveUpdateSteps?: readonly StepProgressItem[];
  onStartUpdate?: () => void;
  onCheckVersion?: () => void;
}

/**
 * Update tab shown ONLY when `computer.status === "incompatible"` (`_uiux.md` U7, US-018):
 * - SSH computers: one-click `Update <name>` with verified release step progress
 * - Non-SSH computers: copyable `compozy update` command + `Check version again`
 */
export function ComputerDetailsUpdateTab({
  computer,
  initialUpdateState = "idle",
  liveUpdateState,
  liveUpdateSteps,
  onStartUpdate,
  onCheckVersion,
}: ComputerDetailsUpdateTabProps) {
  const [localUpdateState, setLocalUpdateState] = useState<UpdateRunState>(initialUpdateState);
  const updateState = liveUpdateState ?? localUpdateState;
  const steps = liveUpdateSteps ?? buildSshUpdateSteps(updateState, computer.name);

  if (computer.originKind !== "ssh") {
    return (
      <div className="flex flex-col gap-4" data-slot="computer-details-update-tab">
        <StatusCard className="border border-line bg-sunken/70 p-3.5 shadow-none" tone="warning">
          <StatusCard.Header label={`${computer.name} runs CompozyOS ${computer.version}`} />
          <StatusCard.Body>
            This app requires a compatible CompozyOS version. One-click updates are available for
            computers added over SSH — run this command directly on{" "}
            <span className="font-medium text-fg">{computer.name}</span> instead:
          </StatusCard.Body>
        </StatusCard>

        <CodeBlock code="compozy update" copyable density="compact" language="bash" showPrompt />

        <div className="flex items-center gap-2">
          <Button
            onClick={() => {
              onCheckVersion?.();
              toast.message(`Checking version on ${computer.name}…`);
            }}
            size="sm"
            type="button"
            variant="secondary"
          >
            <RefreshCw aria-hidden="true" />
            Check version again
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4" data-slot="computer-details-update-tab">
      <StatusCard
        className="border border-line bg-sunken/70 p-3.5 shadow-none"
        tone={updateState === "done" ? "success" : "warning"}
      >
        <StatusCard.Header
          label={
            updateState === "done"
              ? `${computer.name} updated`
              : `${computer.name} runs CompozyOS ${computer.version}`
          }
        />
        <StatusCard.Body>
          {updateState === "done"
            ? "Service restarted and reconnected. Your running work stays available."
            : "CompozyOS verifies the release signature on this computer before installing anything remotely."}
        </StatusCard.Body>
        {updateState === "idle" || updateState === "provenance_failed" ? (
          <StatusCard.Footer>
            <Button
              onClick={() => {
                if (onStartUpdate) {
                  onStartUpdate();
                } else {
                  setLocalUpdateState("running");
                  toast.message(`Updating ${computer.name}…`);
                }
              }}
              size="sm"
              type="button"
            >
              <RefreshCw aria-hidden="true" />
              Update {computer.name}
            </Button>
          </StatusCard.Footer>
        ) : null}
      </StatusCard>

      {updateState !== "idle" ? <StepProgressList steps={steps} /> : null}

      {updateState === "done" ? (
        <ActionResultBanner
          description={`${computer.name} is now connected.`}
          icon={CheckCircle2}
          title="Update complete"
          tone="success"
        />
      ) : null}

      {updateState === "provenance_failed" ? (
        <Alert variant="destructive">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>Couldn&apos;t verify this CompozyOS release</AlertTitle>
          <AlertDescription>
            Nothing was installed on {computer.name}. Try again later or verify your network
            connection to releases.compozy.com.
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}
