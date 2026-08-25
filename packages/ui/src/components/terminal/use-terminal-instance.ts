"use client";

import { useEffect, useEffectEvent, useRef } from "react";

import type { TerminalDimensions, TerminalEngine, TerminalEngineLoader } from "./terminal-engine";
import { readTerminalMetrics, terminalMetricsEqual } from "./terminal-metrics";
import {
  addTerminalInstanceListener,
  attachTerminalInstance,
  detachTerminalInstance,
  getTerminalInstance,
  notifyTerminalInstance,
  registerTerminalInstance,
  type TerminalInstance,
} from "./terminal-registry";
import { attachTerminalRenderer, type TerminalRendererKind } from "./terminal-renderer";
import { observeTerminalTheme, readTerminalTheme, terminalThemesEqual } from "./terminal-theme";

export interface UseTerminalInstanceOptions {
  instanceKey: string;
  readOnly: boolean;
  screenReaderMode: boolean;
  scrollbackLines: number;
  engineLoader: TerminalEngineLoader;
  onData: (data: string) => void;
  onSelectionChange: (selection: string) => void;
  onProposeDimensions: (dimensions: TerminalDimensions) => void;
  onRendererChange: (renderer: TerminalRendererKind) => void;
  onAttached: (instance: TerminalInstance) => void;
}

/** The emulator settings one view wants written onto the buffer it adopts. */
interface TerminalViewOptions {
  readOnly: boolean;
  screenReaderMode: boolean;
  scrollbackLines: number;
}

/**
 * Owns one emulator instance across mounts.
 *
 * Callbacks reach the emulator as effect events, so a re-render never tears the
 * emulator down and a subscription always calls the current props: the effect
 * keys on identity alone, which is what lets a tab switch reattach the same
 * buffer instead of building a new one. Each mount registers its own listener on
 * attach and drops it on detach, so a reattached buffer always talks to the view
 * that is on screen now.
 */
export function useTerminalInstance(options: UseTerminalInstanceOptions): {
  containerRef: React.RefObject<HTMLDivElement | null>;
  instanceRef: React.RefObject<TerminalInstance | null>;
} {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const instanceRef = useRef<TerminalInstance | null>(null);
  const { instanceKey, engineLoader, readOnly, screenReaderMode, scrollbackLines } = options;

  const emitData = useEffectEvent((data: string) => {
    // Read-only is a client courtesy, and this is where it is paid: the
    // daemon's lease is the authority, but a watcher's keystrokes never even
    // leave the browser.
    if (options.readOnly) return;
    options.onData(data);
  });
  const emitSelectionChange = useEffectEvent((selection: string) => {
    options.onSelectionChange(selection);
  });
  const emitRendererChange = useEffectEvent((renderer: TerminalRendererKind) => {
    options.onRendererChange(renderer);
  });
  const emitProposedDimensions = useEffectEvent((dimensions: TerminalDimensions) => {
    options.onProposeDimensions(dimensions);
  });
  const emitAttached = useEffectEvent((instance: TerminalInstance) => {
    options.onAttached(instance);
  });
  const readViewOptions = useEffectEvent(
    (): TerminalViewOptions => ({
      readOnly: options.readOnly,
      screenReaderMode: options.screenReaderMode,
      scrollbackLines: options.scrollbackLines,
    })
  );

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;
    let cancelled = false;
    let release: (() => void) | null = null;

    void (async () => {
      const engine = await engineLoader();
      if (cancelled) return;
      const instance =
        getTerminalInstance(instanceKey) ??
        createInstance(engine, instanceKey, container, emitRendererChange);
      attachTerminalInstance(instance, container);
      const unsubscribe = addTerminalInstanceListener(instance, {
        onData: emitData,
        onSelectionChange: emitSelectionChange,
        onRendererChange: emitRendererChange,
      });
      release = () => {
        unsubscribe();
        detachTerminalInstance(instance);
        if (instanceRef.current === instance) instanceRef.current = null;
      };
      refreshTerminalPaint(instance);
      applyResolvedStyle(instance);
      applyViewOptions(instance, readViewOptions());
      instanceRef.current = instance;
      emitAttached(instance);
      emitRendererChange(instance.renderer.kind);
      // The first proposal has to come from the attach itself. A container that
      // never resizes again would otherwise leave the daemon with no size at
      // all, and the resize observer only speaks when geometry changes.
      reportProposedDimensions(instance, emitProposedDimensions);
    })();

    return () => {
      cancelled = true;
      release?.();
      release = null;
    };
  }, [engineLoader, instanceKey]);

  useEffect(() => {
    const instance = instanceRef.current;
    if (!instance) return;
    applyViewOptions(instance, { readOnly, screenReaderMode, scrollbackLines });
  }, [readOnly, screenReaderMode, scrollbackLines]);

  useEffect(() => {
    const container = containerRef.current;
    const view = container?.ownerDocument?.defaultView;
    if (!container || !view || typeof view.ResizeObserver !== "function") return undefined;
    const observer = new view.ResizeObserver(() => {
      const instance = instanceRef.current;
      if (instance) reportProposedDimensions(instance, emitProposedDimensions);
    });
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  return { containerRef, instanceRef };
}

/**
 * Writes the mounting view's options onto the emulator.
 *
 * Called on every attach, because a reattached instance was last configured by
 * whichever view showed it before — a watch-only pane must not inherit a
 * writable pane's settings just because the buffer survived.
 */
function applyViewOptions(instance: TerminalInstance, view: TerminalViewOptions): void {
  Object.assign(instance.terminal.options, readOnlyOptions(view.readOnly), {
    screenReaderMode: view.screenReaderMode,
    scrollback: view.scrollbackLines,
  });
}

/** Publishes a size the container could host. A degenerate fit says nothing. */
function reportProposedDimensions(
  instance: TerminalInstance,
  report: (dimensions: TerminalDimensions) => void
): void {
  const proposed = instance.fit.proposeDimensions();
  if (proposed && proposed.cols > 0 && proposed.rows > 0) {
    report(proposed);
  }
}

function createInstance(
  engine: TerminalEngine,
  instanceKey: string,
  container: HTMLElement,
  onRendererFallback: (renderer: TerminalRendererKind) => void
): TerminalInstance {
  const host = container.ownerDocument.createElement("div");
  host.dataset.slot = "terminal-view-host";
  host.style.width = "100%";
  host.style.height = "100%";
  container.appendChild(host);

  // Style is resolved through the host, never through the mounting container:
  // the host travels with the emulator, so a reattached buffer keeps reading
  // live tokens instead of the first container it ever saw.
  const metrics = readTerminalMetrics(host);
  const terminal = engine.createTerminal({
    ...readOnlyOptions(false),
    allowProposedApi: false,
    drawBoldTextInBrightColors: true,
    fontFamily: metrics.fontFamily,
    fontSize: metrics.fontSize,
    letterSpacing: metrics.letterSpacing,
    lineHeight: metrics.lineHeight,
    theme: readTerminalTheme(host),
  });
  const fit = engine.createFitAddon();
  terminal.loadAddon(fit);
  terminal.open(host);

  const instance = registerTerminalInstance({
    key: instanceKey,
    terminal,
    fit,
    host,
    renderer: { kind: "dom", dispose: () => undefined },
    mountCount: 0,
    listeners: new Set(),
    teardown: [],
  });
  instance.renderer = attachTerminalRenderer({
    engine,
    terminal,
    onFallback: kind => {
      instance.renderer = { kind, dispose: () => undefined };
      notifyTerminalInstance(instance, listener => listener.onRendererChange(kind));
      // The very first mount subscribes after the renderer is attached, so a
      // failure during construction has no listener to reach yet.
      if (instance.listeners.size === 0) onRendererFallback(kind);
    },
  });
  bindInstanceEvents(instance);
  return instance;
}

/**
 * Instance-scoped subscriptions, created once and torn down only with the
 * emulator. They fan out to whichever views are mounted, which is what keeps a
 * reattached buffer wired to the pane on screen.
 */
function bindInstanceEvents(instance: TerminalInstance): void {
  const data = instance.terminal.onData(payload => {
    notifyTerminalInstance(instance, listener => listener.onData(payload));
  });
  const selection = instance.terminal.onSelectionChange(() => {
    const selected = instance.terminal.getSelection();
    notifyTerminalInstance(instance, listener => listener.onSelectionChange(selected));
  });
  const theme = observeTerminalTheme(instance.host, () => applyResolvedStyle(instance));
  instance.teardown.push(
    () => data.dispose(),
    () => selection.dispose(),
    theme
  );
}

/** Re-resolves palette and metrics from live CSS, writing only on drift. */
function applyResolvedStyle(instance: TerminalInstance): void {
  const nextTheme = readTerminalTheme(instance.host);
  const currentTheme = instance.terminal.options.theme ?? {};
  if (!terminalThemesEqual(currentTheme, nextTheme)) {
    instance.terminal.options.theme = nextTheme;
  }
  const nextMetrics = readTerminalMetrics(instance.host);
  const currentMetrics = {
    fontFamily: instance.terminal.options.fontFamily ?? "",
    fontSize: instance.terminal.options.fontSize ?? 0,
    lineHeight: instance.terminal.options.lineHeight ?? 0,
    letterSpacing: instance.terminal.options.letterSpacing ?? 0,
  };
  if (!terminalMetricsEqual(currentMetrics, nextMetrics)) {
    Object.assign(instance.terminal.options, nextMetrics);
  }
}

/**
 * Watching reads as a hollow, still cursor. The emulator has no hollow *active*
 * cursor, so a read-only view also declines focus — the outline style is then
 * the only cursor it can paint.
 */
function readOnlyOptions(readOnly: boolean): {
  cursorBlink: boolean;
  cursorInactiveStyle: "outline";
  cursorStyle: "block";
  disableStdin: boolean;
} {
  return {
    cursorBlink: !readOnly,
    cursorInactiveStyle: "outline",
    cursorStyle: "block",
    disableStdin: readOnly,
  };
}

/**
 * A re-attached host paints nothing until the emulator is told to repaint: the
 * renderer skipped every frame while the node was out of the document.
 */
function refreshTerminalPaint(instance: TerminalInstance): void {
  const rows = instance.terminal.rows;
  if (rows > 0) instance.terminal.refresh(0, rows - 1);
}
