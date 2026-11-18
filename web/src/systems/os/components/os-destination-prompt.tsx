import { Button } from "@compozy/ui";

import { useDesktop } from "../hooks/use-desktop";
import { shortcutActionLabel } from "../lib/window-manager-shortcuts";
import { windowManagerStore } from "../stores/window-manager-store";
import { OsShortcutChords } from "./os-shortcut-chords";

/**
 * The invitation to pick a destination, shared by a New Tab window and an empty desktop (#1799):
 * the button opens the palette scoped to that New Tab window, or, with `windowId` null, to the
 * empty desktop, whose first tab the pick becomes.
 */
export function OsDestinationPrompt({
  label,
  windowId,
  testId,
}: {
  label: string;
  windowId: string | null;
  testId: string;
}) {
  const paletteShortcutLabel = useDesktop(state =>
    shortcutActionLabel(
      state.windowManagerConfig?.effectiveShortcuts,
      "palette.open",
      typeof navigator === "undefined" ? "" : navigator.platform
    )
  );
  return (
    <div
      className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
      data-testid={testId}
    >
      <p className="text-small-body leading-relaxed text-subtle">{label}</p>
      <Button
        type="button"
        onClick={() =>
          windowManagerStore.trigger.paletteIntentRequested({
            intent: { kind: "destination", windowId },
          })
        }
        size="sm"
        variant="secondary"
      >
        Choose a destination
      </Button>
      {paletteShortcutLabel ? (
        <p className="text-small-body text-subtle">
          or press <OsShortcutChords label={paletteShortcutLabel} />
        </p>
      ) : null}
    </div>
  );
}
