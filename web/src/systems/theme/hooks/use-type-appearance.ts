import { toast } from "@compozy/ui";

import { useSettingsShell, useUpdateSettingsShell } from "@/systems/settings";

import {
  DEFAULT_TYPE_APPEARANCE,
  isLetterSpacing,
  isTextSize,
  type TypeAppearance,
} from "../lib/type-appearance";

export interface TypeAppearanceHandle {
  type: TypeAppearance;
  /** False until the daemon answered — the boot cache paints meanwhile. */
  loaded: boolean;
  /** Persists the changed step (`"default"` resets it) for every connected client. */
  setType: (next: TypeAppearance) => void;
}

/**
 * The operator's Appearance text size and letter spacing, read from the
 * daemon-wide `[shell.appearance]` section (`compozy config get/set
 * shell.appearance.text_size|letter_spacing`). A write sends only the changed
 * key, so it never rewrites the fonts or session-list preferences.
 */
export function useTypeAppearance(): TypeAppearanceHandle {
  const query = useSettingsShell();
  const mutation = useUpdateSettingsShell();
  const stored = query.data?.config.appearance;
  const type: TypeAppearance = {
    size: isTextSize(stored?.text_size) ? stored.text_size : DEFAULT_TYPE_APPEARANCE.size,
    spacing: isLetterSpacing(stored?.letter_spacing)
      ? stored.letter_spacing
      : DEFAULT_TYPE_APPEARANCE.spacing,
  };

  return {
    type,
    loaded: query.data !== undefined,
    setType: next => {
      const sizeChanged = next.size !== type.size;
      const appearance = sizeChanged ? { text_size: next.size } : { letter_spacing: next.spacing };
      // mutateAsync, not mutate callbacks: TanStack only runs per-call callbacks
      // for the latest call, so an earlier overlapping save would fail silently.
      mutation.mutateAsync({ config: { sessions: {}, appearance } }).catch((error: unknown) => {
        const subject = sizeChanged ? "text size" : "letter spacing";
        toast.error(
          error instanceof Error && error.message
            ? `Couldn't save the ${subject}: ${error.message}`
            : `Couldn't save the ${subject}. Try again.`
        );
      });
    },
  };
}
