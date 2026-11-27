import { useContext } from "react";

import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { ComputerScopeContext } from "@/lib/computer-scope-context";

export function useCurrentComputerId(explicitComputerId?: ComputerId): ComputerId {
  const ambient = useContext(ComputerScopeContext);
  return normalizeComputerId(explicitComputerId ?? ambient);
}
