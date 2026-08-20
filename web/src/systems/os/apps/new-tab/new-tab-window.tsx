import { Button, useTopbarSlot } from "@compozy/ui";

import { OsShortcutChords } from "../../components/os-shortcut-chords";
import { useDesktop } from "../../hooks/use-desktop";
import { shortcutActionLabel } from "../../lib/window-manager-shortcuts";
import { windowManagerStore } from "../../stores/window-manager-store";

/**
 * The empty tab created by ⌘T / the deck's `+` (US-005). It owns a real route
 * so the URL stays truthful, and its only job is inviting a destination: the
 * shell palette opens scoped to this window and navigating replaces the tab.
 */
export function NewTabWindow({ windowId }: { windowId: string }) {
  const paletteShortcutLabel = useDesktop(state =>
    shortcutActionLabel(
      state.windowManagerConfig?.effectiveShortcuts,
      "palette.open",
      typeof navigator === "undefined" ? "" : navigator.platform
    )
  );
  useTopbarSlot({
    crumb: <span className="text-muted">New tab</span>,
  });

  return (
    <div
      className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
      data-testid="new-tab-empty"
    >
      <p className="text-small-body leading-relaxed text-subtle">
        Empty tab — open any route or session
      </p>
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
