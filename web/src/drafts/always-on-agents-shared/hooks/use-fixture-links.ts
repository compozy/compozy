import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  linkKey,
  linkShortName,
  linkUnlinkedNote,
  type SessionLinkedWorkItem,
} from "@/systems/session/lib/session-linked-work";

/** The prototype's list: unlinking works and can be undone, as the real list does. */
export function useFixtureLinks(initial: readonly SessionLinkedWorkItem[]) {
  const [items, setItems] = useState<readonly SessionLinkedWorkItem[]>(initial);
  const unlink = (item: SessionLinkedWorkItem) => {
    setItems(current => current.filter(entry => linkKey(entry) !== linkKey(item)));
    toast.message(`Unlinked ${linkShortName(item)}`, {
      description: linkUnlinkedNote(item),
      action: { label: "Undo", onClick: () => setItems(current => [...current, item]) },
    });
  };
  const add = (added: readonly SessionLinkedWorkItem[]) =>
    setItems(current => [...current, ...added]);
  return { items, unlink, add };
}
