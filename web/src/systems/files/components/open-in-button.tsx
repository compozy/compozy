import type { ReactNode } from "react";

import { canListOpenTargets } from "../lib/open-in-targets";

/**
 * The Open in slot on the viewer header and the outside-path state (`_uiux.md` S5). It is
 * absent outside the desktop app (US-006.AC-6): the split button itself lands with the desktop
 * target bridge, which renders through `children`.
 */
export function OpenInButton({ children }: { children?: ReactNode }) {
  if (!canListOpenTargets()) return null;
  return <>{children}</>;
}
