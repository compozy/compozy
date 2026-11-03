import { useState } from "react";

import {
  SessionListCreateMenu,
  type SessionListCreateMenuProps,
} from "../session-list/session-list-create-menu";
import { StaysOnAddPicker, type SessionListStaysOnTarget } from "./stays-on-add-picker";

export type { SessionListStaysOnTarget } from "./stays-on-add-picker";

/**
 * The session `+` with Add stays-on agent. `staysOn` is the list's one concrete scope; `null`
 * when the list spans several projects, so the picker asks where the agent lives first.
 */
export function StaysOnCreateMenu({
  staysOn,
  ...props
}: Omit<SessionListCreateMenuProps, "onAddStaysOn" | "overlay"> & {
  staysOn: SessionListStaysOnTarget | null;
}) {
  const [adding, setAdding] = useState(false);
  return (
    <SessionListCreateMenu
      {...props}
      onAddStaysOn={() => setAdding(true)}
      overlay={adding ? <StaysOnAddPicker onOpenChange={setAdding} target={staysOn} /> : null}
    />
  );
}
