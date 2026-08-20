import { useEffect, useState } from "react";

import {
  desktopShellBridge,
  parseGlobalShortcutRegistrations,
  type GlobalShortcutBindingWire,
  type GlobalShortcutRegistrationWire,
} from "../lib/desktop-shell-bridge";
import type { WindowManagerGlobalShortcutMap } from "../lib/window-manager-shortcut-types";

export interface GlobalShortcutReconciliation {
  registrations: readonly GlobalShortcutRegistrationWire[];
}

interface ReconciliationState {
  bindingKey: string;
  registrations: readonly GlobalShortcutRegistrationWire[];
}

function intendedBindings(
  intended: WindowManagerGlobalShortcutMap | undefined
): GlobalShortcutBindingWire[] {
  return Object.entries(intended ?? {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([command_id, chord]) => ({ command_id, chord }));
}

/** Reconciles daemon-owned intent with Electron and returns shell-owned registration truth. */
export function useGlobalShortcutReconciliation(
  intended: WindowManagerGlobalShortcutMap | undefined
): GlobalShortcutReconciliation {
  const bridge = desktopShellBridge();
  const bindings = intendedBindings(intended);
  const bindingKey = JSON.stringify(bindings);
  const [state, setState] = useState<ReconciliationState>({ bindingKey: "", registrations: [] });

  useEffect(() => {
    if (bridge === null) return undefined;
    const currentBindings = JSON.parse(bindingKey) as GlobalShortcutBindingWire[];
    let cancelled = false;
    void bridge.globalShortcuts.sync(currentBindings).then(
      result => {
        if (!cancelled) {
          setState({ bindingKey, registrations: parseGlobalShortcutRegistrations(result) });
        }
      },
      error => {
        if (cancelled) return;
        console.warn("Desktop shell global hotkey synchronization failed", error);
        setState({
          bindingKey,
          registrations: currentBindings.map(binding => ({
            command_id: binding.command_id,
            intended_chord: binding.chord,
            status: "unsupported",
            reason: "desktop shell synchronization failed",
          })),
        });
      }
    );
    return () => {
      cancelled = true;
    };
  }, [bindingKey, bridge]);

  return {
    registrations: bridge !== null && state.bindingKey === bindingKey ? state.registrations : [],
  };
}
