import type { ReactNode } from "react";

import { TerminalStoreProvider } from "../../contexts/terminal-store-context";

/**
 * The capture harness for Visual Contract rows.
 *
 * The design boards render each piece inside a padded stage at a fixed window
 * width. Reproducing that geometry here is what makes a reference and an
 * implementation capture comparable at the same viewport — it is a property of
 * the *capture*, not of the product, and it never appears in the app.
 */
export function TerminalVisualStage({
  children,
  width = "wide",
}: {
  children: ReactNode;
  /** `wide` matches the board's journal frame; `default` its window frame. */
  width?: "default" | "wide" | "tile" | "block";
}) {
  // A capture is a fixed viewport, so the frame is bounded: an unbounded window
  // grows past 900px and clips whatever is pinned to its bottom edge — which is
  // exactly where the notices that several contracts bind live.
  const frame = {
    default: "h-[520px] w-[min(820px,100%)]",
    wide: "max-h-[560px] w-[min(1060px,100%)]",
    tile: "h-[220px] w-[min(340px,100%)]",
    block: "w-full max-w-[820px]",
  }[width];
  return (
    <TerminalStoreProvider>
      <div className="flex min-h-[520px] items-start justify-center bg-rail px-5 pt-7 pb-8">
        <div
          className={`${frame} flex flex-col overflow-hidden rounded-lg border border-line bg-canvas`}
          data-testid="terminal-visual-stage"
        >
          {children}
        </div>
      </div>
    </TerminalStoreProvider>
  );
}
