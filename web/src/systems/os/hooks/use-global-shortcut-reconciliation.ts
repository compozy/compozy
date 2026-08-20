import { useEffect, useState } from "react";

import {
  desktopShellBridge,
  type GlobalShortcutBindingWire,
  type GlobalShortcutRegistrationWire,
} from "../lib/desktop-shell-bridge";
import type { WindowManagerGlobalShortcutMap } from "../lib/window-manager-shortcut-types";

export interface GlobalShortcutReconciliation {
  registrations: readonly GlobalShortcutRegistrationWire[];
  shell: boolean;
}

interface ReconciliationState {
  bindingKey: string;
  registrations: readonly GlobalShortcutRegistrationWire[];
}

/** Reconciles daemon-owned intent with Electron and returns shell-owned registration truth. */
export function useGlobalShortcutReconciliation(
  intended: WindowManagerGlobalShortcutMap | undefined
): GlobalShortcutReconciliation {
  const bridge = desktopShellBridge();
  const bindingKey = JSON.stringify(
    Object.entries(intended ?? {})
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([command_id, chord]) => ({ command_id, chord }))
  );
  const [state, setState] = useState<ReconciliationState>({ bindingKey: "", registrations: [] });

  useEffect(() => {
    if (bridge === null) return undefined;
    const bindings = JSON.parse(bindingKey) as GlobalShortcutBindingWire[];
    let cancelled = false;
    void bridge.globalShortcuts.sync(bindings).then(
      result => {
        if (!cancelled) setState({ bindingKey, registrations: result });
      },
      error => {
        if (cancelled) return;
        console.warn("Desktop shell global hotkey synchronization failed", error);
        setState({
          bindingKey,
          registrations: bindings.map(binding => ({
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
    shell: bridge !== null,
  };
}
