import { useState } from "react";

import {
  SessionListCreateMenu,
  type SessionListCreateMenuProps,
} from "../session-list/session-list-create-menu";
import { StaysOnAddPicker } from "./stays-on-add-picker";

export interface SessionListStaysOnTarget {
  /** A workspace id, or `@user` for Global. */
  place: string;
  name: string;
}

/** The session `+` with Add stays-on agent, for a list showing one concrete scope. */
export function StaysOnCreateMenu({
  staysOn,
  ...props
}: Omit<SessionListCreateMenuProps, "onAddStaysOn" | "overlay"> & {
  staysOn: SessionListStaysOnTarget;
}) {
  const [adding, setAdding] = useState(false);
  return (
    <SessionListCreateMenu
      {...props}
      onAddStaysOn={() => setAdding(true)}
      overlay={
        adding ? (
          <StaysOnAddPicker name={staysOn.name} onOpenChange={setAdding} place={staysOn.place} />
        ) : null
      }
    />
  );
}
