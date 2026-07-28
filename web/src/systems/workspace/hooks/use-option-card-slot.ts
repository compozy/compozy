import * as React from "react";

import { OptionCardContext, type OptionCardContextValue } from "../components/option-card-context";

export function useOptionCardSlot(slot: string): OptionCardContextValue {
  const ctx = React.use(OptionCardContext);
  if (!ctx) {
    throw new Error(`OptionCard.${slot} must be used inside <OptionCard>.`);
  }
  return ctx;
}
