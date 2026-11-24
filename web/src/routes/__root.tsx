import { createRootRouteWithContext } from "@tanstack/react-router";

import type { RouterContext } from "@/integrations/tanstack-query/root-context";
import {
  RootComponent,
  RootRouteErrorBoundary,
  RootRouteNotFoundBoundary,
} from "./-root-components";
import { guardRootSignInGate } from "./-root-gate";

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: ({ context, location }) => guardRootSignInGate(context.queryClient, location),
  component: RootComponent,
  errorComponent: RootRouteErrorBoundary,
  notFoundComponent: RootRouteNotFoundBoundary,
});
