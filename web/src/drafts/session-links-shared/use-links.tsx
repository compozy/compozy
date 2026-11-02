/** The draft's linked items: unlinking works and can be undone, as the real list would. */
import { useState } from "react";

import { toast } from "@compozy/ui";

import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import { linkShortName } from "./link-model";

export function useLinks(initial: readonly SessionLink[]) {
  const [links, setLinks] = useState<readonly SessionLink[]>(initial);
  const unlink = (link: SessionLink) => {
    setLinks(current => current.filter(item => item.id !== link.id));
    toast.message(`Unlinked ${linkShortName(link)}`, {
      description: link.facts.pull_request
        ? "It's still on GitHub. Only the link to this session is gone."
        : "Its files are untouched. Only the link to this session is gone.",
      action: {
        label: "Undo",
        onClick: () => setLinks(current => [...current, link]),
      },
    });
  };
  const add = (added: readonly SessionLink[]) => setLinks(current => [...current, ...added]);
  return { links, unlink, add };
}
