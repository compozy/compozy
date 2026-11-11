"use client";

import { createContext, type RefObject } from "react";

export interface DocsChromeValue {
  menu: boolean;
  search: boolean;
  openMenu: () => void;
  openSearch: () => void;
  close: () => void;
  /** What held focus when search opened: the palette hands focus back to it on close. */
  searchOpener: RefObject<HTMLElement | null>;
}

export const DocsChromeContext = createContext<DocsChromeValue>({
  menu: false,
  search: false,
  openMenu: () => {},
  openSearch: () => {},
  close: () => {},
  searchOpener: { current: null },
});
