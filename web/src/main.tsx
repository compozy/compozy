import { StrictMode } from "react";
import ReactDOM from "react-dom/client";

import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { Toaster } from "sonner";

import { UIProvider } from "@compozy/ui";

import { createDaemonQueryClient } from "./lib/query-client";
import { routeTree } from "./routeTree.gen";

import "./styles.css";

const queryClient = createDaemonQueryClient();

const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  scrollRestoration: true,
  defaultStructuralSharing: true,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("app");
if (rootElement && !rootElement.innerHTML) {
  ReactDOM.createRoot(rootElement).render(
    <StrictMode>
      <UIProvider>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
          <Toaster
            closeButton
            position="bottom-right"
            richColors
            theme="dark"
            toastOptions={{
              classNames: {
                toast:
                  "border border-border bg-popover text-popover-foreground shadow-[var(--shadow-lg)]",
              },
            }}
          />
        </QueryClientProvider>
      </UIProvider>
    </StrictMode>
  );
}
