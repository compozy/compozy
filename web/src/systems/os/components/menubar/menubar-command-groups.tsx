import { Fragment, type ReactNode } from "react";

import { MenubarSeparator } from "@compozy/ui";

export interface MenubarCommandGroup {
  id: string;
  content: ReactNode;
}

/** Inserts separators only between groups that actually rendered content. */
export function MenubarCommandGroups({ groups }: { groups: readonly MenubarCommandGroup[] }) {
  const present = groups.filter(group => group.content != null && group.content !== false);
  return present.map((group, index) => (
    <Fragment key={group.id}>
      {index > 0 ? <MenubarSeparator /> : null}
      {group.content}
    </Fragment>
  ));
}
