/**
 * The emulator's I/O boundary.
 *
 * Nothing in this package imports `@xterm/*` statically. The engine is fetched
 * on first attach, which keeps the emulator and its stylesheet out of every
 * bundle that merely imports `@compozy/ui`, and gives tests one seam to replace
 * instead of a module graph to intercept.
 */

import type { ITerminalAddon, ITerminalOptions, Terminal } from "@xterm/xterm";

export interface TerminalDimensions {
  cols: number;
  rows: number;
}

/** The addon surface the view drives, narrowed to what it actually calls. */
export interface TerminalFitAddon extends ITerminalAddon {
  proposeDimensions(): TerminalDimensions | undefined;
}

export interface TerminalRendererAddon extends ITerminalAddon {
  onContextLoss(listener: () => void): { dispose(): void };
}

export interface TerminalEngine {
  createTerminal(options: ITerminalOptions): Terminal;
  createFitAddon(): TerminalFitAddon;
  createRendererAddon(): TerminalRendererAddon;
}

export type TerminalEngineLoader = () => Promise<TerminalEngine>;

let pendingEngine: Promise<TerminalEngine> | null = null;

/**
 * Loads the real emulator once per document and shares the result.
 *
 * The stylesheet import rides the same dynamic boundary as the code: importing
 * it at module scope would defeat the lazy split, because a CSS import is a
 * side effect the bundler cannot shake out.
 */
export const loadTerminalEngine: TerminalEngineLoader = () => {
  pendingEngine ??= (async () => {
    const [core, fit, webgl] = await Promise.all([
      import("@xterm/xterm"),
      import("@xterm/addon-fit"),
      import("@xterm/addon-webgl"),
      import("@xterm/xterm/css/xterm.css"),
    ]);
    return {
      createTerminal: (options: ITerminalOptions) => new core.Terminal(options),
      createFitAddon: () => new fit.FitAddon(),
      createRendererAddon: () => new webgl.WebglAddon(),
    };
  })().catch((cause: unknown) => {
    // A chunk request can fail transiently. Keeping that rejected promise here
    // would poison every later terminal mount in the document.
    pendingEngine = null;
    throw cause;
  });
  return pendingEngine;
};
