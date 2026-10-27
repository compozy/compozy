import { toast } from "sonner";

import { useSettingsShell, useUpdateSettingsShell } from "@/systems/settings";

import type { FontAppearance, FontRole } from "../lib/font-appearance";

export interface FontAppearanceHandle {
  appearance: FontAppearance;
  /** False until the daemon answered — the boot cache paints meanwhile. */
  loaded: boolean;
  /** Persists one role's family (`""` resets it) for every connected client. */
  setFont: (role: FontRole, family: string) => void;
  saving: boolean;
}

/**
 * The operator's Appearance fonts, read from the daemon-wide
 * `[shell.appearance]` section (`compozy config get/set shell.appearance.*`).
 * A write sends only the changed key, so it never rewrites session-list
 * preferences or the other font.
 */
export function useFontAppearance(): FontAppearanceHandle {
  const query = useSettingsShell();
  const mutation = useUpdateSettingsShell();
  const stored = query.data?.config.appearance;

  return {
    appearance: { sans: stored?.ui_font ?? "", mono: stored?.mono_font ?? "" },
    loaded: query.data !== undefined,
    setFont: (role, family) => {
      // mutateAsync, not mutate callbacks: TanStack only runs per-call callbacks
      // for the latest call, so an earlier overlapping save would fail silently.
      // The optimistic choice is rolled back; say why instead of just reverting.
      mutation
        .mutateAsync({
          config: {
            sessions: {},
            appearance: role === "sans" ? { ui_font: family } : { mono_font: family },
          },
        })
        .catch((error: unknown) => {
          const subject = role === "sans" ? "Interface font" : "Code font";
          toast.error(
            error instanceof Error && error.message
              ? `Couldn't save the ${subject}: ${error.message}`
              : `Couldn't save the ${subject}. Try again.`
          );
        });
    },
    saving: mutation.isPending,
  };
}
