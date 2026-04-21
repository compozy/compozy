import type { ReactElement } from "react";

import {
  createFileRoute,
  Outlet,
  useRouter,
  type ErrorComponentProps,
  type NotFoundRouteProps,
} from "@tanstack/react-router";

import { AppShellBoundary, AppShellContainer, AppShellErrorBoundary } from "@/systems/app-shell";

export const Route = createFileRoute("/_app")({
  component: AppLayoutRoute,
  errorComponent: AppRouteErrorBoundary,
  notFoundComponent: AppRouteNotFoundBoundary,
});

function AppLayoutRoute(): ReactElement {
  return (
    <AppShellContainer>
      <Outlet />
    </AppShellContainer>
  );
}

function AppRouteErrorBoundary({ error, reset }: ErrorComponentProps): ReactElement {
  const router = useRouter();
  return (
    <AppShellErrorBoundary
      error={error}
      onRetry={() => {
        reset();
        void router.invalidate({ forcePending: true });
      }}
    />
  );
}

function AppRouteNotFoundBoundary({ data: _data }: NotFoundRouteProps): ReactElement {
  return (
    <AppShellBoundary
      description="The requested route does not exist in the daemon web UI."
      eyebrow="Not found"
      testId="app-route-not-found"
      title="Page not found"
    />
  );
}
