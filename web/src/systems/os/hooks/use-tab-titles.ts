import { useSyncExternalStore } from "react";

import type { SessionPayload } from "@/systems/session";

import type { OsWindow } from "../lib/os-types";
import { subscribeWindowSlotRegistry, windowSlotSnapshot } from "../lib/window-slot-registry";
import type { OsTabModel } from "./use-os-tab-bar";
import { osWindowTitle } from "./use-os-window-title";

/** The live title of each tab's lead window, re-read whenever a window publishes a new label. */
export function useTabTitles(
  models: readonly OsTabModel[],
  sessionOf: (win: OsWindow) => SessionPayload | undefined
): readonly string[] {
  // One string snapshot keeps the read stable between publications; NUL never occurs in a title.
  const joined = useSyncExternalStore(subscribeWindowSlotRegistry, () =>
    models
      .map(model => {
        const lead = model.members[0]!;
        return osWindowTitle(lead, sessionOf(lead), windowSlotSnapshot(lead.id));
      })
      .join("\0")
  );
  return models.length === 0 ? [] : joined.split("\0");
}
