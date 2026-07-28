import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useToggleExtension, useUpdateExtension } from "./use-extension-actions";
import { useBundleActivations, useExtensionDetail } from "./use-extensions";

export type ExtensionDetailDialog = "provenance" | "remove" | null;

/**
 * Detail-screen UI state is local to the mounted extension route. Queries and
 * mutations keep their existing TanStack Query ownership.
 */
export function useExtensionDetailState(name: string) {
  const detail = useExtensionDetail(name);
  const bundles = useBundleActivations();
  const toggle = useToggleExtension();
  const update = useUpdateExtension();
  const navigate = useNavigate();
  const [activeDialog, setActiveDialog] = useState<ExtensionDetailDialog>(null);

  return {
    activeDialog,
    bundles,
    detail,
    dismissDialog: () => setActiveDialog(null),
    navigate,
    requestProvenance: () => setActiveDialog("provenance"),
    requestRemoval: () => setActiveDialog("remove"),
    toggle,
    update,
  };
}
