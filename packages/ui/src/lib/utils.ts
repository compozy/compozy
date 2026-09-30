import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";
import { fontSizeClasses, radiusScale } from "./font-size-classes.generated";

// The class list is generated from the `--text-*` theme tokens in
// packages/ui/src/tokens.css and packages/site/app/global.css, because
// tailwind-merge otherwise reads those utilities as text colors and drops them
// when a color utility sits in the same cn() call. Keep it generated: a
// hand-maintained list silently rots the moment a token is added.
const customTwMerge = extendTailwindMerge({
  extend: {
    // Project radius tokens (`rounded-pill`, `rounded-icon-well`, …) conflict with
    // each other and with Tailwind's own radius names, like the defaults do.
    theme: {
      radius: [...radiusScale],
    },
    classGroups: {
      "font-size": [...fontSizeClasses],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return customTwMerge(clsx(inputs));
}
