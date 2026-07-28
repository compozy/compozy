import type { ReactNode } from "react";

import {
  ScopeSelectorContext,
  type ScopeSelectorContextValue,
} from "./scope-selector-context-value";

interface ScopeSelectorProviderProps {
  value: ScopeSelectorContextValue;
  children: ReactNode;
}

export function ScopeSelectorProvider({ value, children }: ScopeSelectorProviderProps) {
  return <ScopeSelectorContext value={value}>{children}</ScopeSelectorContext>;
}

export type { ScopeSelectorContextValue };
