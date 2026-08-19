import type { ReactNode } from "react";

import {
  LoopInputCatalogContext,
  useLoopInputCatalogValue,
} from "../../hooks/use-loop-input-catalogs";
import type { LoopInputCatalogNeeds } from "../../lib/loop-input-catalogs";

interface LoopInputCatalogProviderProps {
  workspaceId: string;
  needs: LoopInputCatalogNeeds;
  children: ReactNode;
}

function LoopInputCatalogProvider({ workspaceId, needs, children }: LoopInputCatalogProviderProps) {
  const value = useLoopInputCatalogValue(workspaceId, needs);
  return (
    <LoopInputCatalogContext.Provider value={value}>{children}</LoopInputCatalogContext.Provider>
  );
}

export function LoopInputCatalogBoundary({
  workspaceId,
  needs,
  children,
}: LoopInputCatalogProviderProps) {
  if (workspaceId.trim() === "") return children;
  return (
    <LoopInputCatalogProvider workspaceId={workspaceId} needs={needs}>
      {children}
    </LoopInputCatalogProvider>
  );
}
