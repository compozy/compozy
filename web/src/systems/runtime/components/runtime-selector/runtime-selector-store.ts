import { createStoreLogic } from "@xstate/store";

import type { RailFilter } from "./runtime-list-model";

export interface RuntimeSelectorActiveRow {
  cursor: string;
  key: string;
}

export type RuntimeSelectorEntryMode = "catalog" | "exact";

interface RuntimeSelectorPopupClosed {
  phase: "closed";
}

interface RuntimeSelectorPopupOpen {
  activeRow: RuntimeSelectorActiveRow | null;
  advancedExpanded: boolean;
  entryMode: RuntimeSelectorEntryMode;
  favoriteAnnouncement: string;
  phase: "open";
  query: string;
  railFilter: RailFilter;
}

/** Popup-local state; query, filter, cursor, and ARIA message only exist while open. */
export type RuntimeSelectorPopupState = RuntimeSelectorPopupClosed | RuntimeSelectorPopupOpen;

type RuntimeSelectorPopupEvents = {
  activeRowChanged: { row: RuntimeSelectorActiveRow | null };
  advancedOptionsToggled: { expanded: boolean };
  exactEntryCanceled: {};
  exactEntryCommitted: {};
  exactEntryStarted: {};
  favoriteAnnounced: { message: string };
  popupClosed: {};
  popupOpened: {};
  queryChanged: { query: string };
  railChanged: { railFilter: RailFilter };
};

function openPopupState(): RuntimeSelectorPopupOpen {
  return {
    activeRow: null,
    advancedExpanded: false,
    entryMode: "catalog",
    favoriteAnnouncement: "",
    phase: "open",
    query: "",
    railFilter: "all",
  };
}

function returnToCatalog(context: RuntimeSelectorPopupState): RuntimeSelectorPopupState {
  if (context.phase !== "open" || context.entryMode !== "exact") return context;
  return { ...context, activeRow: null, entryMode: "catalog", query: "" };
}

export const runtimeSelectorPopupLogic = createStoreLogic<
  RuntimeSelectorPopupState,
  RuntimeSelectorPopupEvents
>({
  context: { phase: "closed" },
  on: {
    activeRowChanged: (context, event) => {
      if (context.phase !== "open") return context;
      return { ...context, activeRow: event.row };
    },
    advancedOptionsToggled: (context, event) => {
      if (context.phase !== "open") return context;
      return { ...context, advancedExpanded: event.expanded };
    },
    exactEntryCanceled: returnToCatalog,
    exactEntryCommitted: returnToCatalog,
    exactEntryStarted: context => {
      if (context.phase !== "open") return context;
      return { ...context, activeRow: null, entryMode: "exact", query: "" };
    },
    favoriteAnnounced: (context, event) => {
      if (context.phase !== "open") return context;
      return { ...context, favoriteAnnouncement: event.message };
    },
    popupClosed: context => (context.phase === "closed" ? context : { phase: "closed" }),
    popupOpened: () => openPopupState(),
    queryChanged: (context, event) => {
      if (context.phase !== "open") return context;
      return { ...context, activeRow: null, query: event.query };
    },
    railChanged: (context, event) => {
      if (context.phase !== "open") return context;
      return { ...context, activeRow: null, railFilter: event.railFilter };
    },
  },
});
