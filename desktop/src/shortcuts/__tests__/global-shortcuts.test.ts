import { describe, expect, it, vi } from "vitest";

import { MACOS_ACCESSIBILITY_SETTINGS_URL, detectAccessibility } from "../accessibility";
import { chordToAccelerator, UnconvertibleShortcutError } from "../accelerator";
import { ElectronGlobalShortcut } from "../electron-global-shortcut";
import { GlobalShortcutPolicy, type GlobalShortcutLike } from "../global-shortcut-policy";

function shortcutHarness(outcomes: boolean[]) {
  return {
    register: vi.fn((_accelerator: string, _callback: () => void) => outcomes.shift() ?? true),
    unregister: vi.fn((_accelerator: string) => undefined),
    unregisterAll: vi.fn(() => undefined),
  } satisfies GlobalShortcutLike;
}

describe("global shortcut policy", () => {
  it.each([
    ["meta+shift+Space", "CommandOrControl+Shift+Space"],
    ["alt+shift+KeyN", "Alt+Shift+N"],
    ["ctrl+Digit1", "Control+1"],
  ])("Should convert %s to %s", (chord, expected) => {
    expect(chordToAccelerator(chord)).toBe(expected);
  });

  it("Should return a typed error for an unconvertible chord", () => {
    expect(() => chordToAccelerator("meta+Semicolon")).toThrow(UnconvertibleShortcutError);
  });

  it("Should restore the previous working binding when the replacement is in use", () => {
    const globalShortcut = shortcutHarness([true, false, true]);
    const policy = new GlobalShortcutPolicy({
      globalShortcut,
      accessibility: () => ({ allowed: true }),
      onInvoke: vi.fn(),
    });
    expect(policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }])).toEqual([
      expect.objectContaining({ status: "registered", active_chord: "meta+shift+Space" }),
    ]);
    expect(policy.sync([{ command_id: "palette.open", chord: "meta+alt+Space" }])).toEqual([
      expect.objectContaining({
        status: "failed_in_use",
        intended_chord: "meta+alt+Space",
        active_chord: "meta+shift+Space",
      }),
    ]);
    expect(globalShortcut.register).toHaveBeenNthCalledWith(
      3,
      "CommandOrControl+Shift+Space",
      expect.any(Function)
    );
  });

  it("Should not report an active chord when restoration also fails", () => {
    const policy = new GlobalShortcutPolicy({
      globalShortcut: shortcutHarness([true, false, false]),
      accessibility: () => ({ allowed: true }),
      onInvoke: vi.fn(),
    });
    policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }]);

    expect(policy.sync([{ command_id: "palette.open", chord: "meta+alt+Space" }])).toEqual([
      expect.not.objectContaining({ active_chord: expect.any(String) }),
    ]);
  });

  it("Should unregister every shortcut during shutdown", () => {
    const globalShortcut = shortcutHarness([true]);
    const policy = new GlobalShortcutPolicy({
      globalShortcut,
      accessibility: () => ({ allowed: true }),
      onInvoke: vi.fn(),
    });
    policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }]);
    policy.unregisterAll();
    expect(globalShortcut.unregisterAll).toHaveBeenCalledOnce();
    expect(policy.status()).toEqual([]);
  });

  it("Should expose only confirmed callbacks to the E2E shortcut seam", () => {
    const callback = vi.fn();
    const runtime = new ElectronGlobalShortcut(shortcutHarness([false, true]));

    expect(runtime.register("CommandOrControl+Shift+Space", callback)).toBe(false);
    expect(runtime.invokeForE2E("CommandOrControl+Shift+Space")).toBe(false);
    expect(runtime.register("CommandOrControl+Shift+Space", callback)).toBe(true);
    expect(runtime.invokeForE2E("CommandOrControl+Shift+Space")).toBe(true);
    expect(callback).toHaveBeenCalledOnce();

    runtime.unregister("CommandOrControl+Shift+Space");
    expect(runtime.invokeForE2E("CommandOrControl+Shift+Space")).toBe(false);
  });

  it("Should report macOS Accessibility denial with the system settings deep link", () => {
    const accessibility = detectAccessibility({ platform: "darwin", isTrusted: () => false });
    const policy = new GlobalShortcutPolicy({
      globalShortcut: shortcutHarness([]),
      accessibility: () => accessibility,
      onInvoke: vi.fn(),
    });
    expect(policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }])).toEqual([
      expect.objectContaining({
        status: "failed_permission",
        settings_url: MACOS_ACCESSIBILITY_SETTINGS_URL,
      }),
    ]);
  });

  it("Should register after Accessibility is granted without restarting", () => {
    let allowed = false;
    const policy = new GlobalShortcutPolicy({
      globalShortcut: shortcutHarness([true]),
      accessibility: () =>
        allowed
          ? { allowed: true }
          : { allowed: false, settingsURL: MACOS_ACCESSIBILITY_SETTINGS_URL },
      onInvoke: vi.fn(),
    });
    expect(policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }])).toEqual([
      expect.objectContaining({ status: "failed_permission" }),
    ]);
    allowed = true;
    expect(policy.sync([{ command_id: "palette.open", chord: "meta+shift+Space" }])).toEqual([
      expect.objectContaining({ status: "registered", active_chord: "meta+shift+Space" }),
    ]);
  });
});
