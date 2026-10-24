import { Skeleton, SkeletonText } from "@compozy/ui";

import { ROLE_ORDER } from "../lib/roles-config";
import { LIST_CLASS, LIST_HEAD_CLASS, LIST_PANEL_CLASS } from "./role-list-classes";
import { RolePanelHeaderSkeleton } from "./role-panel-header";

/** The role list in skeleton form: the count line, the toggle and one collapsed row per known role. */
export function RoleListSkeleton() {
  return (
    <section aria-hidden="true" className={LIST_CLASS}>
      <header className={LIST_HEAD_CLASS}>
        <SkeletonText className="w-24 text-small-body" />
        <Skeleton className="h-button-sm w-20 rounded-pill" />
      </header>
      <div className={LIST_PANEL_CLASS}>
        {ROLE_ORDER.map(role => (
          <div className="not-last:border-b not-last:border-line-soft" key={role}>
            <RolePanelHeaderSkeleton />
          </div>
        ))}
      </div>
    </section>
  );
}
