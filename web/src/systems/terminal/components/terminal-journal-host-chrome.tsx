import { ScrollText } from "lucide-react";

import { useTopbarSlot } from "@compozy/ui";

/** Publishes the journal's identity into the OS window head. */
export function TerminalJournalHostChrome({
  hostChrome,
  projectLabel,
  onBack,
}: {
  hostChrome: boolean;
  projectLabel?: string;
  /** Returns to the terminal underneath the overlay. */
  onBack?: () => void;
}) {
  useTopbarSlot(
    hostChrome
      ? {
          glyph: <ScrollText />,
          crumb: "Journal",
          status: projectLabel ? (
            <span className="truncate text-badge text-subtle">{projectLabel}</span>
          ) : undefined,
          ...(onBack ? { onBack } : {}),
        }
      : null
  );
  return null;
}
