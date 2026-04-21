import type { Decorator, Preview } from "@storybook/react-vite";
import { withThemeByClassName } from "@storybook/addon-themes";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Fragment, createElement, useState, type ReactNode } from "react";
import { initialize, mswLoader } from "msw-storybook-addon";

import { UIProvider } from "@compozy/ui";

import "../src/styles.css";
import { routeTree } from "@/routeTree.gen";
import { storybookSystemHandlerGroups, storybookSystemHandlers } from "@/storybook/msw";
import { resetActiveWorkspaceStoreForTests } from "@/systems/app-shell";
import { setRunStreamFactoryOverrideForTests } from "@/systems/runs";

initialize({ onUnhandledRequest: "bypass" });

type StoryRenderer = () => ReactNode;
type StorybookDecorator = Decorator;
export type StorybookRouterMode = "app" | "stub";

export interface StorybookRouterOptions {
  kind?: StorybookRouterMode;
  initialEntries?: string[];
}

export function createStorybookQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Number.POSITIVE_INFINITY,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

function createStubStorybookRouter(
  Story: StoryRenderer = () => null,
  options?: StorybookRouterOptions
) {
  const rootRoute = createRootRoute();
  const storyRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: Story,
  });

  return createRouter({
    routeTree: rootRoute.addChildren([storyRoute]),
    history: createMemoryHistory({
      initialEntries: options?.initialEntries ?? ["/"],
    }),
  });
}

function createAppStorybookRouter(options?: StorybookRouterOptions) {
  return createRouter({
    routeTree,
    history: createMemoryHistory({
      initialEntries: options?.initialEntries ?? ["/"],
    }),
    defaultPreload: "intent",
    scrollRestoration: true,
    defaultStructuralSharing: true,
  });
}

export function createStorybookRouter(
  Story: StoryRenderer = () => null,
  options?: StorybookRouterOptions
) {
  if (options?.kind === "app") {
    return createAppStorybookRouter(options);
  }

  return createStubStorybookRouter(Story, options);
}

function resetStorybookAppState() {
  resetActiveWorkspaceStoreForTests();
  setRunStreamFactoryOverrideForTests(null);
  if (typeof window !== "undefined") {
    window.sessionStorage.clear();
  }
}

function StorybookProvidersBoundary({
  Story,
  routerOptions,
}: {
  Story: StoryRenderer;
  routerOptions?: StorybookRouterOptions;
}) {
  const [queryClient] = useState(createStorybookQueryClient);
  const [router] = useState(() => {
    if (routerOptions?.kind === "app") {
      resetStorybookAppState();
    }
    return createStorybookRouter(Story, routerOptions);
  });

  return createElement(
    UIProvider,
    null,
    createElement(
      QueryClientProvider,
      { client: queryClient },
      routerOptions?.kind === "app"
        ? createElement(
            Fragment,
            null,
            createElement(Story),
            createElement(RouterProvider, { router })
          )
        : createElement(RouterProvider, { router })
    )
  );
}

export const themeDecorator: StorybookDecorator = withThemeByClassName({
  themes: {
    light: "",
    dark: "dark",
  },
  defaultTheme: "dark",
});

export const routerDecorator: StorybookDecorator = (
  Story: StoryRenderer,
  context?: { parameters?: { router?: StorybookRouterOptions } }
) =>
  createElement(StorybookProvidersBoundary, {
    Story,
    routerOptions: context?.parameters?.router,
  });

export const storybookDecorators: StorybookDecorator[] = [themeDecorator, routerDecorator];
export const storybookLoaders = [mswLoader];
export { storybookSystemHandlerGroups, storybookSystemHandlers };

const preview: Preview = {
  decorators: storybookDecorators,
  loaders: storybookLoaders,
  parameters: {
    backgrounds: {
      disable: true,
    },
    controls: {
      expanded: true,
    },
    msw: {
      handlers: storybookSystemHandlerGroups,
    },
  },
};

export default preview;
