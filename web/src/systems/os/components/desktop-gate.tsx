import { AlertTriangle, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty, Spinner } from "@compozy/ui";

import {
  OnboardingApiError,
  OnboardingSetupPanel,
  useOnboardingStatus,
} from "@/systems/onboarding";

/**
 * Desktop-level onboarding gate: first-run setup renders **over** the shell, not
 * instead of it — you see the desktop you are about to unlock while a blocking
 * panel asks the two setup questions. The chrome marks itself inert
 * (`DesktopShell` threads `firstRun` down); wrapping `children` here would break
 * the `flex min-h-0 flex-1` chain the shell layout depends on.
 *
 * Loading and error keep the standalone frame: before the daemon answers there
 * is nothing truthful to render behind a scrim.
 */
export function DesktopGate({ children }: { children: ReactNode }) {
  const onboarding = useOnboardingStatus();

  if (onboarding.data?.completed === true) {
    return children;
  }

  if (onboarding.data?.completed === false) {
    return (
      <>
        {children}
        <OnboardingSetupPanel onComplete={() => void onboarding.refetch()} />
      </>
    );
  }

  if (onboarding.isError) {
    const view = gateErrorView(onboarding.error);
    return (
      <GateFrame testId="onboarding-gate-error">
        <Empty
          className="max-w-xl"
          cause={view.cause}
          description={view.description}
          icon={AlertTriangle}
          title={view.title}
          titleAs="h1"
          action={
            <Button
              onClick={() => void onboarding.refetch()}
              size="sm"
              type="button"
              variant="outline"
            >
              <RefreshCw className="size-3" />
              Retry
            </Button>
          }
        />
      </GateFrame>
    );
  }

  return (
    <GateFrame testId="onboarding-gate-loading">
      <Spinner />
    </GateFrame>
  );
}

function GateFrame({ children, testId }: { children: ReactNode; testId: string }) {
  return (
    <main
      id="app-content"
      data-testid={testId}
      className="flex min-h-0 flex-1 items-center justify-center bg-canvas"
    >
      {children}
    </main>
  );
}

interface GateErrorView {
  title: string;
  description: string;
  cause?: string;
}

/**
 * What the gate says when onboarding status fails. A 403 is the daemon's access
 * policy answering with fixed, daemon-authored text (e.g. the non-loopback HTTP
 * guard), so the operator reads it verbatim; any other failure keeps runtime
 * messages off screen because they may contain secrets.
 */
function gateErrorView(error: unknown): GateErrorView {
  const refusal =
    error instanceof OnboardingApiError && error.status === 403 ? error.message.trim() : "";
  if (refusal) {
    return { title: "CompozyOS refused this connection", description: refusal };
  }
  return {
    title: "CompozyOS isn't responding",
    description: "Make sure CompozyOS is running, then try again.",
    cause:
      error instanceof Error ? "CompozyOS couldn't confirm whether setup is complete." : undefined,
  };
}
