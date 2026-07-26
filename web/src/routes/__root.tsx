import { createRootRouteWithContext } from "@tanstack/react-router";

import type { RouterContext } from "@/integrations/tanstack-query/root-context";
import {
  RootComponent,
  RootRouteErrorBoundary,
  RootRouteNotFoundBoundary,
} from "./-root-components";

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
  errorComponent: RootRouteErrorBoundary,
  notFoundComponent: RootRouteNotFoundBoundary,
});
