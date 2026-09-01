import { act, render, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { destroyTerminalInstances } from "../terminal-registry";
import { TerminalView, type TerminalViewHandle } from "../terminal-view";
import { createFakeFontFaceSet, installFakeFontFaceSet } from "./fake-font-face-set";
import { createFakeEngine, type FakeEngine } from "./fake-terminal-engine";

/**
 * Canonical suite for the `TerminalView` primitive (UT-080, UT-081, UT-082).
 *
 * Invariant: one emulator per identity survives mount and unmount without
 * disposal and rebinds to whichever view is showing it; its palette comes from
 * the terminal tokens and follows a theme change; read-only suppresses local
 * input; renderer fallback is scoped to a single instance; a proposed size
 * never mutates the emulator.
 */

/**
 * The canonical ramp as `tokens.css` declares it, installed where production
 * puts it.
 *
 * These are the `--terminal-*` names, not the `--color-terminal-*` utility
 * adapters: the bridge resolves the canonical identity, and an adapter that
 * stopped pointing at it must not be able to hide behind a passing test.
 */
const TERMINAL_TOKENS: Record<string, string> = {
  "--terminal-bg": "#131211",
  "--terminal-fg": "#e3e1de",
  "--terminal-cursor": "#f7f6f4",
  "--terminal-selection": "rgba(255, 255, 255, 0.16)",
  "--terminal-ansi-0": "#2a2927",
  "--terminal-ansi-1": "#e0635a",
  "--terminal-ansi-9": "#ef837b",
  "--terminal-ansi-15": "#f7f6f4",
};

let instanceCounter = 0;

function nextInstanceId(): string {
  instanceCounter += 1;
  return `term-fixture-${instanceCounter}`;
}

function loaderFor(engine: FakeEngine) {
  return () => Promise.resolve(engine);
}

beforeEach(() => {
  for (const [token, value] of Object.entries(TERMINAL_TOKENS)) {
    document.documentElement.style.setProperty(token, value);
  }
});

let restoreFonts: (() => void) | null = null;

afterEach(() => {
  destroyTerminalInstances(() => true);
  restoreFonts?.();
  restoreFonts = null;
  for (const token of Object.keys(TERMINAL_TOKENS)) {
    document.documentElement.style.removeProperty(token);
  }
  document.documentElement.classList.remove("theme-switched");
});

describe("TerminalView", () => {
  it("Should keep one emulator across mounts and build its palette from terminal tokens", async () => {
    const engine = createFakeEngine();
    const instanceId = nextInstanceId();
    const first = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    terminal.write("first screen");

    first.unmount();
    expect(terminal.disposed).toBe(false);

    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
      />
    );
    await waitFor(() => expect(engine.lastTerminal().openedIn).toHaveLength(1));
    expect(engine.terminals).toHaveLength(1);
    expect(engine.lastTerminal().writes).toEqual(["first screen"]);

    const theme = terminal.options.theme ?? {};
    expect(theme.background).toBe("#131211");
    expect(theme.foreground).toBe("#e3e1de");
    expect(theme.black).toBe("#2a2927");
    expect(theme.brightRed).toBe("#ef837b");
    expect(theme.brightWhite).toBe("#f7f6f4");
  });

  it("Should resolve the palette from the canonical tokens, not the utility aliases", async () => {
    const engine = createFakeEngine();
    // The utility adapters are what Tailwind generates; they are not the
    // bridge's source. Pointing one somewhere else must not change the screen.
    document.documentElement.style.setProperty("--color-terminal-bg", "#ff0000");
    document.documentElement.style.setProperty("--color-terminal-ansi-1", "#ff0000");

    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    const theme = engine.lastTerminal().options.theme ?? {};
    expect(theme.background).toBe("#131211");
    expect(theme.red).toBe("#e0635a");

    document.documentElement.style.removeProperty("--color-terminal-bg");
    document.documentElement.style.removeProperty("--color-terminal-ansi-1");
  });

  it("Should rebind input, selection and renderer callbacks to the view that reattached", async () => {
    const engine = createFakeEngine();
    const instanceId = nextInstanceId();
    const firstData = vi.fn();
    const firstSelection = vi.fn();
    const secondData = vi.fn();
    const secondSelection = vi.fn();
    const secondRenderer = vi.fn();

    const first = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
        onData={firstData}
        onSelectionChange={firstSelection}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    first.unmount();

    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
        onData={secondData}
        onRendererChange={secondRenderer}
        onSelectionChange={secondSelection}
      />
    );
    await waitFor(() => expect(secondRenderer).toHaveBeenCalledWith("webgl"));

    terminal.emitData("ls\r");
    terminal.emitSelectionChange("expected 201, received 500");

    expect(secondData).toHaveBeenCalledWith("ls\r");
    expect(secondSelection).toHaveBeenCalledWith("expected 201, received 500");
    // The unmounted view is gone, not merely quiet: a binding left on the first
    // mount would deliver a live terminal's keystrokes to a dead component.
    expect(firstData).not.toHaveBeenCalled();
    expect(firstSelection).not.toHaveBeenCalled();
    expect(engine.terminals).toHaveLength(1);
    expect(terminal.disposed).toBe(false);
  });

  it("Should reconfigure a reattached buffer for the view now showing it", async () => {
    const engine = createFakeEngine();
    const instanceId = nextInstanceId();
    const writableData = vi.fn();
    const watcherData = vi.fn();

    const writable = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
        onData={writableData}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    expect(terminal.options.disableStdin).toBe(false);
    writable.unmount();

    render(
      <TerminalView
        aria-label="Terminal output — watching"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
        onData={watcherData}
        readOnly
      />
    );
    await waitFor(() => expect(terminal.options.disableStdin).toBe(true));

    terminal.emitData("rm -rf /\r");

    expect(watcherData).not.toHaveBeenCalled();
    expect(writableData).not.toHaveBeenCalled();
    expect(terminal.options.cursorBlink).toBe(false);
  });

  it("Should keep following theme changes after a reattach", async () => {
    const engine = createFakeEngine();
    const instanceId = nextInstanceId();
    const first = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    first.unmount();

    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={instanceId}
      />
    );
    await waitFor(() => expect(terminal.options.theme?.background).toBe("#131211"));

    document.documentElement.style.setProperty("--terminal-bg", "#050403");
    document.documentElement.classList.add("theme-switched");

    await waitFor(() => expect(terminal.options.theme?.background).toBe("#050403"));
  });

  it("Should resolve the write promise only after the emulator reports the parse", async () => {
    const engine = createFakeEngine();
    const handleRef = React.createRef<TerminalViewHandle>();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    const parsed = vi.fn();
    void handleRef.current?.write("output").then(parsed);
    await Promise.resolve();
    expect(parsed).not.toHaveBeenCalled();

    engine.lastTerminal().completeWrite(0);
    await waitFor(() => expect(parsed).toHaveBeenCalledOnce());
  });

  it("Should suppress local input and still the cursor when read-only", async () => {
    const engine = createFakeEngine();
    const onData = vi.fn();
    render(
      <TerminalView
        aria-label="Terminal output — watching"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        onData={onData}
        readOnly
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();

    terminal.emitData("rm -rf /\r");

    expect(onData).not.toHaveBeenCalled();
    expect(terminal.options.disableStdin).toBe(true);
    expect(terminal.options.cursorBlink).toBe(false);
    expect(terminal.options.cursorInactiveStyle).toBe("outline");
  });

  it("Should forward local input when the view is writable", async () => {
    const engine = createFakeEngine();
    const onData = vi.fn();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        onData={onData}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    engine.lastTerminal().emitData("ls\r");

    expect(onData).toHaveBeenCalledWith("ls\r");
    expect(engine.lastTerminal().options.disableStdin).toBe(false);
  });

  it("Should fall back to the DOM renderer per pane when WebGL cannot start", async () => {
    const failing = createFakeEngine({ rendererFailure: "activate" });
    const healthy = createFakeEngine();
    const onFailingRenderer = vi.fn();
    const onHealthyRenderer = vi.fn();

    render(
      <TerminalView
        aria-label="Failing pane"
        engineLoader={loaderFor(failing)}
        instanceId={nextInstanceId()}
        onRendererChange={onFailingRenderer}
      />
    );
    await waitFor(() => expect(onFailingRenderer).toHaveBeenCalledWith("dom"));

    render(
      <TerminalView
        aria-label="Healthy pane"
        engineLoader={loaderFor(healthy)}
        instanceId={nextInstanceId()}
        onRendererChange={onHealthyRenderer}
      />
    );

    // The second pane still tries WebGL: one pane's failure is never a
    // process-global latch.
    await waitFor(() => expect(onHealthyRenderer).toHaveBeenCalledWith("webgl"));
    expect(healthy.rendererAddons).toHaveLength(1);
  });

  it("Should fall back to the DOM renderer per pane when the addon cannot be built", async () => {
    const failing = createFakeEngine({ rendererFailure: "construct" });
    const onRendererChange = vi.fn();
    render(
      <TerminalView
        aria-label="Failing pane"
        engineLoader={loaderFor(failing)}
        instanceId={nextInstanceId()}
        onRendererChange={onRendererChange}
      />
    );

    await waitFor(() => expect(onRendererChange).toHaveBeenCalledWith("dom"));
    expect(failing.rendererAddons).toHaveLength(0);
    expect(failing.lastTerminal().disposed).toBe(false);
  });

  it("Should fall back for that pane alone when a live renderer loses its context", async () => {
    const engine = createFakeEngine();
    const onRendererChange = vi.fn();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        onRendererChange={onRendererChange}
      />
    );
    await waitFor(() => expect(onRendererChange).toHaveBeenCalledWith("webgl"));

    act(() => engine.rendererAddons[0].loseContext());

    await waitFor(() => expect(onRendererChange).toHaveBeenLastCalledWith("dom"));
    expect(engine.rendererAddons[0].disposed).toBe(true);
    expect(engine.lastTerminal().disposed).toBe(false);
  });

  it("Should report a proposed size without resizing, and resize only when told", async () => {
    const engine = createFakeEngine({ proposedDimensions: { cols: 132, rows: 44 } });
    const onProposeDimensions = vi.fn();
    const handleRef = React.createRef<TerminalViewHandle>();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
        onProposeDimensions={onProposeDimensions}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    await waitFor(() => expect(onProposeDimensions).toHaveBeenCalledWith({ cols: 132, rows: 44 }));

    expect(engine.lastTerminal().resizes).toEqual([]);

    handleRef.current?.applyDimensions({ cols: 96, rows: 28 });

    expect(engine.lastTerminal().resizes).toEqual([{ cols: 96, rows: 28 }]);
  });

  it("Should expose the selected scrollback range for quoting", async () => {
    const engine = createFakeEngine();
    const handleRef = React.createRef<TerminalViewHandle>();
    const onSelectionChange = vi.fn();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
        onSelectionChange={onSelectionChange}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    engine.lastTerminal().emitSelectionChange("expected 201, received 500");

    expect(onSelectionChange).toHaveBeenCalledWith("expected 201, received 500");
    // The emulator counts rows from zero; the quote block, `--lines A-B` and
    // the journal all count from one, so the conversion happens here, once.
    expect(handleRef.current?.getSelectionRange()).toEqual({
      startLine: 13,
      endLine: 15,
      text: "expected 201, received 500",
    });
  });

  it("Should normalize an upward selection into an ordered quote range", async () => {
    const engine = createFakeEngine({
      selectionPosition: {
        start: { x: 4, y: 14 },
        end: { x: 0, y: 12 },
      },
    });
    const handleRef = React.createRef<TerminalViewHandle>();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    engine.lastTerminal().emitSelectionChange("expected 201, received 500");

    expect(handleRef.current?.getSelectionRange()).toEqual({
      startLine: 13,
      endLine: 15,
      text: "expected 201, received 500",
    });
  });

  it("Should hold what is written before the emulator exists, in order", async () => {
    const engine = createFakeEngine();
    const engineLoad: { land?: () => void } = {};
    const handleRef = React.createRef<TerminalViewHandle>();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={() =>
          new Promise(resolve => {
            engineLoad.land = () => resolve(engine);
          })
        }
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engineLoad.land).toBeDefined());

    // A stream that starts painting immediately writes into nothing until the
    // engine lands — and none of it may be lost.
    const parsed = vi.fn();
    handleRef.current?.applyDimensions({ cols: 40, rows: 10 });
    void handleRef.current?.write("first").then(parsed);
    handleRef.current?.reset();
    void handleRef.current?.write("second").then(parsed);

    engineLoad.land?.();
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    // The size applied before the first byte is applied first here too.
    expect(terminal.resizes).toEqual([{ cols: 40, rows: 10 }]);
    expect(parsed).not.toHaveBeenCalled();

    terminal.completeWrite(0);
    await waitFor(() => expect(parsed).toHaveBeenCalledTimes(1));
    // Serialized: the second write only starts once the first has been parsed.
    expect(terminal.writes).toEqual(["first", "second"]);
    terminal.completeWrite(1);
    await waitFor(() => expect(parsed).toHaveBeenCalledTimes(2));
  });

  it("Should keep live operations behind a startup write still being parsed", async () => {
    const engine = createFakeEngine();
    const engineLoad: { land?: () => void } = {};
    const handleRef = React.createRef<TerminalViewHandle>();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={() =>
          new Promise(resolve => {
            engineLoad.land = () => resolve(engine);
          })
        }
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engineLoad.land).toBeDefined());

    void handleRef.current?.write("startup");
    engineLoad.land?.();
    await waitFor(() => expect(engine.lastTerminal().writes).toEqual(["startup"]));

    handleRef.current?.applyDimensions({ cols: 120, rows: 36 });
    handleRef.current?.reset();
    void handleRef.current?.write("live");
    expect(engine.lastTerminal().resizes).toEqual([]);
    expect(engine.lastTerminal().resetCount).toBe(0);
    expect(engine.lastTerminal().writes).toEqual(["startup"]);

    engine.lastTerminal().completeWrite(0);
    await waitFor(() => expect(engine.lastTerminal().writes).toEqual(["startup", "live"]));
    expect(engine.lastTerminal().resizes).toEqual([{ cols: 120, rows: 36 }]);
    expect(engine.lastTerminal().resetCount).toBe(1);
    engine.lastTerminal().completeWrite(1);
  });

  it("Should reject queued writes when loading fails and retry on a later mount", async () => {
    const engine = createFakeEngine();
    const loadFailure = new Error("terminal chunk unavailable");
    let attempts = 0;
    const retryingLoader = () => {
      attempts += 1;
      return attempts === 1 ? Promise.reject(loadFailure) : Promise.resolve(engine);
    };
    const handleRef = React.createRef<TerminalViewHandle>();
    const first = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={retryingLoader}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    const firstOutcome = vi.fn();
    void handleRef.current?.write("first attempt").then(
      () => firstOutcome("resolved"),
      cause => firstOutcome(cause)
    );

    await waitFor(() => expect(firstOutcome).toHaveBeenCalledExactlyOnceWith(loadFailure));
    first.unmount();

    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={retryingLoader}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const secondOutcome = vi.fn();
    void handleRef.current?.write("second attempt").then(secondOutcome);
    engine.lastTerminal().completeWrite(0);

    await waitFor(() => expect(secondOutcome).toHaveBeenCalledOnce());
    expect(attempts).toBe(2);
  });

  it("Should not announce attachment when a startup write is abandoned", async () => {
    const engine = createFakeEngine();
    const engineLoad: { land?: () => void } = {};
    const handleRef = React.createRef<TerminalViewHandle>();
    const onAttached = vi.fn();
    const { unmount } = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={() =>
          new Promise(resolve => {
            engineLoad.land = () => resolve(engine);
          })
        }
        handleRef={handleRef}
        instanceId={nextInstanceId()}
        onAttached={onAttached}
      />
    );
    await waitFor(() => expect(engineLoad.land).toBeDefined());
    void handleRef.current?.write("startup").catch(() => undefined);
    engineLoad.land?.();
    await waitFor(() => expect(engine.lastTerminal().writes).toEqual(["startup"]));

    unmount();
    destroyTerminalInstances(() => true);
    await Promise.resolve();
    await Promise.resolve();

    expect(onAttached).not.toHaveBeenCalled();
  });

  it("Should keep queued output through a StrictMode remount", async () => {
    const engine = createFakeEngine();
    const engineLoad: { land?: () => void } = {};
    const handleRef = React.createRef<TerminalViewHandle>();
    const view = (
      <TerminalView
        aria-label="Terminal output"
        engineLoader={() =>
          new Promise(resolve => {
            engineLoad.land = () => resolve(engine);
          })
        }
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    render(<React.StrictMode>{view}</React.StrictMode>);
    await waitFor(() => expect(engineLoad.land).toBeDefined());

    // StrictMode tears an effect down and sets it up again on the same mount.
    // Output queued across that is still perfectly valid and must survive.
    const outcome = vi.fn();
    void handleRef.current?.write("still valid").then(
      () => outcome("resolved"),
      () => outcome("rejected")
    );
    await Promise.resolve();
    expect(outcome).not.toHaveBeenCalled();

    engineLoad.land?.();
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    engine.lastTerminal().completeWrite(0);

    await waitFor(() => expect(outcome).toHaveBeenCalledExactlyOnceWith("resolved"));
  });

  it("Should refuse a live write whose view went away before it was drawn", async () => {
    const engine = createFakeEngine();
    const handleRef = React.createRef<TerminalViewHandle>();
    const { unmount } = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        handleRef={handleRef}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();

    const outcome = vi.fn();
    // Rejected, never resolved: the client reads a resolve as "drawn" and would
    // return credit and resume past bytes nobody ever saw.
    const write = handleRef.current?.write("never drawn").then(
      () => outcome("resolved"),
      () => outcome("rejected")
    );

    unmount();
    destroyTerminalInstances(() => true);
    await write;

    expect(outcome).toHaveBeenCalledExactlyOnceWith("rejected");
    // A parse callback that arrives afterwards changes nothing.
    terminal.completeWrite(0);
    await Promise.resolve();
    expect(outcome).toHaveBeenCalledOnce();
  });

  it("Should keep the grid reachable by assistive technology", async () => {
    const engine = createFakeEngine();
    const { getByRole } = render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        screenReaderMode
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    expect(getByRole("log", { name: "Terminal output" })).toBeInTheDocument();
    await waitFor(() => expect(engine.lastTerminal().options.screenReaderMode).toBe(true));
  });

  it("Should never leave a keystroke path open on a read-only grid", async () => {
    const engine = createFakeEngine();
    const onData = vi.fn();
    const { getByRole } = render(
      <TerminalView
        aria-label="Terminal output — watching"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        onData={onData}
        readOnly
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));

    await userEvent.click(getByRole("log"));
    await userEvent.keyboard("whoami");

    expect(onData).not.toHaveBeenCalled();
  });

  it("Should pass host shortcuts without stealing an unselected terminal interrupt", async () => {
    const engine = createFakeEngine();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
      />
    );
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    const terminal = engine.lastTerminal();
    const key = (value: string) => new KeyboardEvent("keydown", { key: value, ctrlKey: true });

    expect(terminal.acceptKeyEvent(key("v"))).toBe(false);
    expect(terminal.acceptKeyEvent(key("k"))).toBe(false);
    expect(terminal.acceptKeyEvent(key("c"))).toBe(true);

    terminal.emitSelectionChange("selected output");
    expect(terminal.acceptKeyEvent(key("c"))).toBe(false);
  });

  it("Should gate the first paint on a missing font stack and clear the glyph atlas once it lands", async () => {
    const fonts = createFakeFontFaceSet();
    restoreFonts = installFakeFontFaceSet(fonts);
    const engine = createFakeEngine();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        style={{ fontFamily: '"Test Mono", monospace', fontSize: "12px" }}
      />
    );

    await waitFor(() => expect(fonts.loadCalls).toHaveLength(1));
    // The stack is still loading and the budget has not elapsed: opening the
    // emulator now would rasterize tofu into the atlas.
    expect(engine.terminals).toHaveLength(0);

    fonts.settleLoads();
    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    await waitFor(() => expect(engine.lastTerminal().clearTextureAtlasCount).toBe(1));
    // The clear alone leaves a blank canvas; the repaint is what puts the now
    // correctly-rasterized rows back on screen.
    expect(engine.lastTerminal().refreshCount).toBeGreaterThanOrEqual(2);
  });

  it("Should attach without clearing the glyph atlas when the font stack was already resident", async () => {
    const fonts = createFakeFontFaceSet(['12px "Test Mono"']);
    restoreFonts = installFakeFontFaceSet(fonts);
    const engine = createFakeEngine();
    render(
      <TerminalView
        aria-label="Terminal output"
        engineLoader={loaderFor(engine)}
        instanceId={nextInstanceId()}
        style={{ fontFamily: '"Test Mono", monospace', fontSize: "12px" }}
      />
    );

    await waitFor(() => expect(engine.terminals).toHaveLength(1));
    expect(fonts.loadCalls).toHaveLength(0);
    await act(async () => {});
    expect(engine.lastTerminal().clearTextureAtlasCount).toBe(0);
  });
});
