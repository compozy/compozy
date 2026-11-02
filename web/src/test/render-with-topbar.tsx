import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render as rtlRender, type RenderResult } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";

import { Topbar, TopbarSlotProvider, useTopbarSlotValue } from "@compozy/ui";

interface RenderWithTopbarResult extends RenderResult {
  rerender: (ui: ReactNode) => void;
}

function TopbarToolbarHost() {
  const slot = useTopbarSlotValue();
  if (!slot?.toolbar) return null;
  return (
    <div data-slot="os-window-toolbar" className="flex min-h-[38px] items-center px-3">
      {slot.toolbar}
    </div>
  );
}

/**
 * Test helper that mounts a route component under a TopbarSlotProvider plus a
 * stub `<Topbar>` and optional toolbar strip, so `useTopbarSlot` publishers
 * (identity, actions, overflow, toolbar) appear in the test DOM.
 */
export function renderWithTopbar(ui: ReactElement): RenderWithTopbarResult {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrap = (child: ReactNode) => (
    <QueryClientProvider client={client}>
      <TopbarSlotProvider>
        <Topbar title="Test" />
        <TopbarToolbarHost />
        {child}
      </TopbarSlotProvider>
    </QueryClientProvider>
  );
  const result = rtlRender(wrap(ui));
  return {
    ...result,
    rerender: (next: ReactNode) => result.rerender(wrap(next)),
  };
}
