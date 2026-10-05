import type { ReactNode } from "react";
import {
  Link,
  Outlet,
  useRouter,
  type ErrorComponentProps,
  type NotFoundRouteProps,
} from "@tanstack/react-router";
import { AlertTriangle, Compass, RefreshCw } from "lucide-react";

import { Button, Empty, buttonVariants } from "@compozy/ui";

import { GatewayAccessBoundary } from "@/systems/gateway";

export function RootComponent() {
  return (
    <GatewayAccessBoundary>
      <div
        data-testid="root-shell"
        className="flex h-dvh flex-col overflow-hidden bg-background text-foreground"
      >
        <SkipToContentLink />
        <Outlet />
      </div>
    </GatewayAccessBoundary>
  );
}

function SkipToContentLink() {
  return (
    <a
      data-testid="skip-to-content"
      href="#app-content"
      className="sr-only fixed top-2 left-2 z-50 rounded-pill bg-primary px-3 py-2 text-form-label font-medium text-primary-foreground focus:not-sr-only focus-visible:outline-none focus-visible:shadow-focus-ring"
    >
      Skip to content
    </a>
  );
}

const remoteHTTPAPIBlockedMessage =
  "remote HTTP API access is disabled unless the daemon is bound to a loopback host";

export function RootRouteErrorBoundary({ error }: ErrorComponentProps) {
  const router = useRouter();
  const handleRetry = () => {
    void router.invalidate();
  };

  return (
    <GatewayAccessBoundary>
      <RootBoundaryFrame testId="root-route-error">
        <Empty
          className="max-w-xl"
          cause={routeErrorCause(error)}
          description={
            error instanceof Error && error.message === remoteHTTPAPIBlockedMessage
              ? remoteHTTPAPIBlockedMessage
              : "This screen didn't load. Reload to try again."
          }
          icon={AlertTriangle}
          title="Something went wrong"
          titleAs="h1"
          action={
            <>
              <Button onClick={handleRetry} size="sm" type="button" variant="secondary">
                <RefreshCw className="size-3" />
                Retry
              </Button>
              <Link className={buttonVariants({ variant: "secondary", size: "sm" })} to="/">
                <Compass className="size-3" />
                Go home
              </Link>
            </>
          }
        />
      </RootBoundaryFrame>
    </GatewayAccessBoundary>
  );
}

export function RootRouteNotFoundBoundary({ routeId }: NotFoundRouteProps) {
  return (
    <GatewayAccessBoundary>
      <RootBoundaryFrame routeId={routeId} testId="root-route-not-found">
        <Empty
          className="max-w-xl"
          description="This page doesn't exist."
          icon={Compass}
          title="Page not found"
          titleAs="h1"
          action={
            <Link className={buttonVariants({ variant: "secondary", size: "sm" })} to="/">
              <Compass className="size-3" />
              Go home
            </Link>
          }
        />
      </RootBoundaryFrame>
    </GatewayAccessBoundary>
  );
}

function RootBoundaryFrame({
  children,
  routeId,
  testId,
}: {
  children: ReactNode;
  routeId?: string;
  testId: string;
}) {
  return (
    <div className="flex min-h-dvh flex-col bg-background text-foreground">
      <main
        data-route-id={routeId}
        data-testid={testId}
        className="flex flex-1 items-center justify-center overflow-y-auto px-6 py-8"
      >
        {children}
      </main>
    </div>
  );
}

/** Safe detail for the collapsed disclosure; runtime messages may contain secrets. */
function routeErrorCause(error: unknown): string | undefined {
  if (!(error instanceof Error)) return undefined;
  return "The route stopped before CompozyOS could render this screen.";
}
