import { use, useState } from "react";

import { RailLookContext } from "../rail-look";

/** One fold, seeded from the review state's folded keys. */
export function useFold(key: string) {
  const { collapsed } = use(RailLookContext);
  const [open, setOpen] = useState(!collapsed.has(key));
  return [open, () => setOpen(value => !value)] as const;
}
