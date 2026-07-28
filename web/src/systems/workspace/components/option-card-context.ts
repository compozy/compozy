import * as React from "react";

export type OptionCardSize = "compact" | "comfortable";

export interface OptionCardContextValue {
  size: OptionCardSize;
}

export const OptionCardContext = React.createContext<OptionCardContextValue | null>(null);
