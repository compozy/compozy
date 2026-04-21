import type { ReactElement, ReactNode } from "react";

import {
  AppShell,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellSidebar,
  Button,
  SectionHeading,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";

export interface AppShellBoundaryProps {
  eyebrow: string;
  title: string;
  description: string;
  detail?: string;
  testId: string;
  action?: ReactNode;
}

export function AppShellBoundary({
  eyebrow,
  title,
  description,
  detail,
  testId,
  action,
}: AppShellBoundaryProps): ReactElement {
  return (
    <AppShell>
      <AppShellSidebar>
        <p className="eyebrow text-muted-foreground">Daemon</p>
        <p className="text-sm leading-6 text-muted-foreground">{description}</p>
      </AppShellSidebar>
      <AppShellMain>
        <AppShellHeader>
          <SectionHeading eyebrow={eyebrow} title={title} />
        </AppShellHeader>
        <AppShellContent>
          <SurfaceCard data-testid={testId}>
            <SurfaceCardHeader>
              <div>
                <SurfaceCardEyebrow>{eyebrow}</SurfaceCardEyebrow>
                <SurfaceCardTitle>{title}</SurfaceCardTitle>
                <SurfaceCardDescription>{description}</SurfaceCardDescription>
              </div>
            </SurfaceCardHeader>
            {detail ? (
              <SurfaceCardBody>
                <p className="text-sm text-muted-foreground" data-testid={`${testId}-detail`}>
                  {detail}
                </p>
              </SurfaceCardBody>
            ) : null}
            {action ? <SurfaceCardBody>{action}</SurfaceCardBody> : null}
          </SurfaceCard>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}

export function AppShellErrorBoundary({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry: () => void;
}): ReactElement {
  const message = describeError(error, "The requested page failed to load.");
  return (
    <AppShellBoundary
      action={
        <Button onClick={onRetry} size="sm" type="button" variant="secondary">
          Retry
        </Button>
      }
      description="The daemon returned an error while resolving this page."
      detail={message}
      eyebrow="Error"
      testId="app-route-error"
      title="Something went wrong"
    />
  );
}

function describeError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}
