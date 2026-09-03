/**
 * Accelerated rendering with a fallback that is scoped to one terminal.
 *
 * The fallback deliberately keeps no module-level state. A machine that fails
 * to give one terminal a GPU context very often gives the next one a context
 * just fine — a process-global latch turns one recoverable failure into a
 * permanently degraded session for every pane on screen.
 */

import type { Terminal } from "@xterm/xterm";

import type { TerminalEngine, TerminalRendererAddon } from "./terminal-engine";

export type TerminalRendererKind = "webgl" | "dom";

export interface TerminalRendererBinding {
  kind: TerminalRendererKind;
  dispose(): void;
}

export interface AttachRendererOptions {
  engine: TerminalEngine;
  terminal: Terminal;
  /** Called when an already-running accelerated renderer loses its context. */
  onFallback(kind: TerminalRendererKind): void;
}

/**
 * Loads the accelerated renderer for one terminal, degrading to the DOM
 * renderer on construction failure, activation failure, or a later context
 * loss. The addon is disposed on every failure path: a partially activated
 * addon left attached keeps painting nothing.
 */
export function attachTerminalRenderer(options: AttachRendererOptions): TerminalRendererBinding {
  const { engine, terminal, onFallback } = options;
  let addon: TerminalRendererAddon | null = null;
  try {
    addon = engine.createRendererAddon();
    terminal.loadAddon(addon);
  } catch {
    disposeQuietly(addon);
    return { kind: "dom", dispose: () => undefined };
  }
  let contextLossSubscription: { dispose(): void } | null = null;
  try {
    contextLossSubscription = addon.onContextLoss(() => {
      disposeQuietly(addon);
      addon = null;
      onFallback("dom");
    });
  } catch {
    disposeQuietly(addon);
    return { kind: "dom", dispose: () => undefined };
  }
  return {
    kind: "webgl",
    dispose: () => {
      contextLossSubscription?.dispose();
      disposeQuietly(addon);
      addon = null;
    },
  };
}

function disposeQuietly(addon: TerminalRendererAddon | null): void {
  if (!addon) return;
  try {
    addon.dispose();
  } catch {
    // A renderer that cannot even dispose is already gone; the DOM renderer
    // takes over either way and the terminal must not die with the addon.
  }
}
