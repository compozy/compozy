import type { ITerminalAddon, ITerminalOptions, Terminal } from "@xterm/xterm";

import type { TerminalEngine, TerminalFitAddon, TerminalRendererAddon } from "../terminal-engine";

/**
 * A stand-in for the emulator library — the one I/O boundary these tests mock.
 *
 * It records what the view asked the emulator to do so assertions can be made
 * about real behaviour (writes parsed before the callback, resizes applied only
 * on demand, options actually assigned) rather than about the mock itself.
 */
export interface FakeTerminal extends Terminal {
  readonly writes: string[];
  readonly resizes: Array<{ cols: number; rows: number }>;
  readonly openedIn: HTMLElement[];
  disposed: boolean;
  emitData(payload: string): void;
  emitSelectionChange(selection: string): void;
  /** Releases the parse callback for the write at `index`. */
  completeWrite(index: number): void;
}

export interface FakeEngineOptions {
  /** Throws where a machine without a usable GPU context would throw. */
  rendererFailure?: "construct" | "activate";
  proposedDimensions?: { cols: number; rows: number };
}

export interface FakeEngine extends TerminalEngine {
  readonly terminals: FakeTerminal[];
  readonly rendererAddons: FakeRendererAddon[];
  /** Every terminal built by this engine, newest last. */
  lastTerminal(): FakeTerminal;
}

export interface FakeRendererAddon extends TerminalRendererAddon {
  loseContext(): void;
  disposed: boolean;
}

export function createFakeEngine(options: FakeEngineOptions = {}): FakeEngine {
  const terminals: FakeTerminal[] = [];
  const rendererAddons: FakeRendererAddon[] = [];
  return {
    terminals,
    rendererAddons,
    lastTerminal: () => {
      const terminal = terminals.at(-1);
      if (!terminal) throw new Error("no terminal was created");
      return terminal;
    },
    createTerminal: terminalOptions => {
      const terminal = createFakeTerminal(terminalOptions, options);
      terminals.push(terminal);
      return terminal;
    },
    createFitAddon: () => createFakeFitAddon(options.proposedDimensions),
    createRendererAddon: () => {
      if (options.rendererFailure === "construct") {
        throw new Error("WebGL context could not be created");
      }
      const addon = createFakeRendererAddon(options.rendererFailure === "activate");
      rendererAddons.push(addon);
      return addon;
    },
  };
}

function createFakeFitAddon(proposed?: { cols: number; rows: number }): TerminalFitAddon {
  return {
    activate: () => undefined,
    dispose: () => undefined,
    proposeDimensions: () => proposed,
  };
}

function createFakeRendererAddon(failOnActivate: boolean): FakeRendererAddon {
  const listeners: Array<() => void> = [];
  return {
    disposed: false,
    activate: () => {
      if (failOnActivate) throw new Error("WebGL addon failed to activate");
    },
    dispose() {
      this.disposed = true;
    },
    onContextLoss: (listener: () => void) => {
      listeners.push(listener);
      return {
        dispose: () => {
          const index = listeners.indexOf(listener);
          if (index >= 0) listeners.splice(index, 1);
        },
      };
    },
    loseContext: () => {
      for (const listener of Array.from(listeners)) listener();
    },
  };
}

function createFakeTerminal(
  terminalOptions: ITerminalOptions,
  engineOptions: FakeEngineOptions
): FakeTerminal {
  void engineOptions;
  const dataListeners: Array<(payload: string) => void> = [];
  const selectionListeners: Array<() => void> = [];
  const pendingWrites: Array<() => void> = [];
  let selection = "";
  let rows = 24;
  let cols = 80;

  const terminal = {
    options: { ...terminalOptions },
    writes: [] as string[],
    resizes: [] as Array<{ cols: number; rows: number }>,
    openedIn: [] as HTMLElement[],
    disposed: false,
    get rows() {
      return rows;
    },
    get cols() {
      return cols;
    },
    loadAddon: (addon: ITerminalAddon) => addon.activate(terminal as unknown as Terminal),
    open: (parent: HTMLElement) => {
      terminal.openedIn.push(parent);
    },
    write: (data: string | Uint8Array, callback?: () => void) => {
      terminal.writes.push(typeof data === "string" ? data : new TextDecoder().decode(data));
      // The real emulator parses asynchronously; holding the callback is what
      // lets a test prove credit is returned on parse, not on receipt.
      if (callback) pendingWrites.push(callback);
    },
    completeWrite: (index: number) => {
      pendingWrites[index]?.();
    },
    resize: (nextCols: number, nextRows: number) => {
      cols = nextCols;
      rows = nextRows;
      terminal.resizes.push({ cols: nextCols, rows: nextRows });
    },
    refresh: () => undefined,
    reset: () => undefined,
    focus: () => undefined,
    dispose: () => {
      terminal.disposed = true;
    },
    getSelection: () => selection,
    getSelectionPosition: () =>
      selection ? { start: { x: 0, y: 12 }, end: { x: 4, y: 14 } } : undefined,
    onData: (listener: (payload: string) => void) => {
      dataListeners.push(listener);
      return { dispose: () => dataListeners.splice(dataListeners.indexOf(listener), 1) };
    },
    onSelectionChange: (listener: () => void) => {
      selectionListeners.push(listener);
      return {
        dispose: () => selectionListeners.splice(selectionListeners.indexOf(listener), 1),
      };
    },
    emitData: (payload: string) => {
      for (const listener of Array.from(dataListeners)) listener(payload);
    },
    emitSelectionChange: (next: string) => {
      selection = next;
      for (const listener of Array.from(selectionListeners)) listener();
    },
  };
  return terminal as unknown as FakeTerminal;
}
