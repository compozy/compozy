import { useTopbarSlot } from "@compozy/ui";

import { OsDestinationPrompt } from "../../components/os-destination-prompt";

/**
 * The empty tab created by ⌘T / the deck's `+` (US-005). It owns a real route
 * so the URL stays truthful, and its only job is inviting a destination: the
 * shell palette opens scoped to this window and navigating replaces the tab.
 */
export function NewTabWindow({ windowId }: { windowId: string }) {
  useTopbarSlot({
    crumb: <span className="text-muted-foreground">New tab</span>,
  });

  return (
    <OsDestinationPrompt label="This tab is empty" windowId={windowId} testId="new-tab-empty" />
  );
}
