"use client";

import { createContext } from "react";

export type SearchSeed = {
  query: string;
  version: number;
};

export type SiteSearchContextValue = {
  query: string;
  seed: SearchSeed;
  openWithQuery: (query: string) => void;
  setQuery: (query: string) => void;
};

export const SiteSearchContext = createContext<SiteSearchContextValue | null>(null);
