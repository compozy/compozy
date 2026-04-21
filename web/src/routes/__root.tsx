import type { ReactElement } from "react";

import {
  createRootRoute,
  Outlet,
  type ErrorComponentProps,
  type NotFoundRouteProps,
} from "@tanstack/react-router";

import { AppShellBoundary } from "@/systems/app-shell";

export const Route = createRootRoute({
  component: RootRoute,
  errorComponent: RootErrorBoundary,
  notFoundComponent: RootNotFoundBoundary,
});

function RootRoute(): ReactElement {
  return <Outlet />;
}

function RootErrorBoundary({ error }: ErrorComponentProps): ReactElement {
  const message = describeError(error);
  return (
    <AppShellBoundary
      description="The daemon shell failed to render. Retrying will reload the current route."
      detail={message}
      eyebrow="Root"
      testId="root-route-error"
      title="Shell failed to render"
    />
  );
}

function RootNotFoundBoundary({ data: _data }: NotFoundRouteProps): ReactElement {
  return (
    <AppShellBoundary
      description="The requested route is not part of the daemon web UI."
      eyebrow="Not found"
      testId="root-route-not-found"
      title="Route not found"
    />
  );
}

function describeError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "Unknown rendering error.";
}
