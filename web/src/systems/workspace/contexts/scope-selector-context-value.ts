import { createContext } from "react";

export interface ScopeSelectorContextValue {
  selectGlobalScope: () => void;
}

export const ScopeSelectorContext = createContext<ScopeSelectorContextValue | null>(null);
