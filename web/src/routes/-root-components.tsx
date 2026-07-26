import type { ReactNode } from "react";
import {
  Link,
  Outlet,
  useRouter,
  type ErrorComponentProps,
  type NotFoundRouteProps,
} from "@tanstack/react-router";
import { AlertTriangle, Compass, RefreshCw } from "lucide-react";

import { Button, Empty, buttonVariants } from "@agh/ui";

export function RootComponent() {
  return (
    <div
      data-testid="root-shell"
      className="flex h-dvh flex-col overflow-hidden bg-background text-foreground"
    >
      <SkipToContentLink />
      <Outlet />
    </div>
  );
}

function SkipToContentLink() {
  return (
    <a
      data-testid="skip-to-content"
      href="#app-content"
      className="sr-only fixed top-2 left-2 z-50 rounded-md bg-accent px-3 py-2 font-mono text-form-label font-medium text-accent-ink shadow-highlight focus:not-sr-only focus-visible:outline-none focus-visible:shadow-focus-ring"
    >
      Skip to content
    </a>
  );
}

export function RootRouteErrorBoundary({ error, reset }: ErrorComponentProps) {
  const router = useRouter();
  const handleRetry = () => {
    reset();
    void router.invalidate({ forcePending: true });
  };

  return (
    <RootBoundaryFrame testId="root-route-error">
      <Empty
        className="max-w-xl"
        description={describeRouteError(
          error,
          "The application shell failed before the route could render."
        )}
        icon={AlertTriangle}
        title="Unable to render this route"
        titleAs="h1"
        action={
          <>
            <Button onClick={handleRetry} size="sm" type="button" variant="outline">
              <RefreshCw className="size-3" />
              Retry
            </Button>
            <Link className={buttonVariants({ variant: "outline", size: "sm" })} to="/">
              <Compass className="size-3" />
              Go home
            </Link>
          </>
        }
      />
    </RootBoundaryFrame>
  );
}

export function RootRouteNotFoundBoundary({ routeId }: NotFoundRouteProps) {
  return (
    <RootBoundaryFrame routeId={routeId} testId="root-route-not-found">
      <Empty
        className="max-w-xl"
        description="The requested route does not exist in this build."
        icon={Compass}
        title="Route not found"
        titleAs="h1"
        action={
          <Link className={buttonVariants({ variant: "outline", size: "sm" })} to="/">
            <Compass className="size-3" />
            Go home
          </Link>
        }
      />
    </RootBoundaryFrame>
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

function describeRouteError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim().length > 0) return error.message;
  return fallback;
}
