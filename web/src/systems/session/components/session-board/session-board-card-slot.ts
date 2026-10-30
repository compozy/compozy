import { createContext, type ComponentType } from "react";

import type { SessionBoardCardProps } from "./session-board-card";

/**
 * The card the board's columns and Archive bar render. `SessionsBoard` provides the live
 * `SessionBoardCard` (runtime label, entry preload, row menus); a prototype provides a card
 * built on `SessionBoardCardView` with fixture data, so the board view never reaches the daemon.
 */
export const SessionBoardCardSlot = createContext<ComponentType<SessionBoardCardProps> | null>(
  null
);

/** The provided card; a board rendered without a provider is a wiring bug, not an empty state. */
export function slottedCard(card: ComponentType<SessionBoardCardProps> | null) {
  if (!card) throw new Error("SessionBoardCardSlot is not provided (render SessionsBoard)");
  return card;
}
