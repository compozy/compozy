import type { ReactNode } from "react";

import { SessionThreadReadOnlyContext } from "./session-thread-read-only-context";

export function SessionThreadReadOnlyProvider({
  children,
  readOnly,
}: {
  children: ReactNode;
  readOnly: boolean;
}) {
  return <SessionThreadReadOnlyContext value={readOnly}>{children}</SessionThreadReadOnlyContext>;
}
