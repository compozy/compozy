import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { createContext, use, useState, type ReactNode } from "react";

import { Toaster, TooltipProvider, UIProvider } from "@compozy/ui";

import { resetClientStores } from "@/storybook/reset-client-stores";

import type { Theme } from "#gallery/shell/view";

const CanvasContent = createContext<ReactNode>(null);

function Content() {
  return use(CanvasContent);
}

function createCanvasRouter() {
  const root = createRootRoute({ component: Content });
  const catchAll = createRoute({ getParentRoute: () => root, path: "$" });
  return createRouter({
    routeTree: root.addChildren([catchAll]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
}

/**
 * The provider stack of one prototype canvas, mirroring the app and Storybook: `UIProvider` with
 * the user's reduced-motion preference, `TooltipProvider`, a fresh memory-history router (links
 * never reach production loaders or change the page URL) and the toast region in the canvas
 * theme. Mounting a canvas resets the module-level client stores, so a state never inherits the
 * previous one's edits. There is deliberately no QueryClient: prototypes render from fixtures.
 */
export function PrototypeCanvas({ theme, children }: { theme: Theme; children: ReactNode }) {
  const [router] = useState(() => {
    resetClientStores();
    return createCanvasRouter();
  });
  return (
    <UIProvider reducedMotion="user">
      <TooltipProvider>
        <CanvasContent value={children}>
          <RouterProvider router={router} />
        </CanvasContent>
        <Toaster expand position="top-right" theme={theme} visibleToasts={5} />
      </TooltipProvider>
    </UIProvider>
  );
}
