import { render as rtlRender, type RenderResult } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";

import { Topbar, TopbarSlotProvider, useTopbarSlotValue } from "@agh/ui";

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
  const wrap = (child: ReactNode) => (
    <TopbarSlotProvider>
      <Topbar title="Test" />
      <TopbarToolbarHost />
      {child}
    </TopbarSlotProvider>
  );
  const result = rtlRender(wrap(ui));
  return {
    ...result,
    rerender: (next: ReactNode) => result.rerender(wrap(next)),
  };
}
