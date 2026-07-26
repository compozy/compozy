import { AlertTriangle, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty, Spinner } from "@agh/ui";

import { OnboardingSetupPanel, useOnboardingStatus } from "@/systems/onboarding";

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
    return (
      <GateFrame testId="onboarding-gate-error">
        <Empty
          className="max-w-xl"
          description={describeGateError(
            onboarding.error,
            "AGH could not confirm whether first-run setup is complete."
          )}
          icon={AlertTriangle}
          title="Unable to check onboarding"
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

function describeGateError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}
