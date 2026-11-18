import { useState } from "react";

import {
  SessionListCreateMenu,
  type SessionListCreateMenuProps,
} from "../session-list/session-list-create-menu";
import { AlwaysOnAddPicker, type SessionListAlwaysOnTarget } from "./always-on-add-picker";

export type { SessionListAlwaysOnTarget } from "./always-on-add-picker";

/**
 * The session `+` with Add always-on agent. `alwaysOn` is the list's one concrete scope; `null`
 * when the list spans several projects, so the picker asks where the agent lives first. A host
 * that also offers the verb elsewhere (its project menu) holds `adding`, so the picker still
 * opens over this `+`.
 */
export function AlwaysOnCreateMenu({
  alwaysOn,
  adding: heldAdding,
  onAddingChange,
  ...props
}: Omit<SessionListCreateMenuProps, "onAddAlwaysOn" | "overlay"> & {
  alwaysOn: SessionListAlwaysOnTarget | null;
  adding?: boolean;
  onAddingChange?: (adding: boolean) => void;
}) {
  const [ownAdding, setOwnAdding] = useState(false);
  const adding = heldAdding ?? ownAdding;
  const setAdding = onAddingChange ?? setOwnAdding;
  return (
    <SessionListCreateMenu
      {...props}
      onAddAlwaysOn={() => setAdding(true)}
      overlay={
        adding ? (
          // A new list scope starts the picker over, so it never adds to the old place.
          <AlwaysOnAddPicker
            key={alwaysOn?.place ?? "several"}
            onOpenChange={setAdding}
            target={alwaysOn}
          />
        ) : null
      }
    />
  );
}
