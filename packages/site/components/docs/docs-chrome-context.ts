"use client";

import { createContext } from "react";

export interface DocsChromeValue {
  menu: boolean;
  search: boolean;
  openMenu: () => void;
  openSearch: () => void;
  close: () => void;
}

export const DocsChromeContext = createContext<DocsChromeValue>({
  menu: false,
  search: false,
  openMenu: () => {},
  openSearch: () => {},
  close: () => {},
});
