import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useToggleExtension, useUpdateExtension } from "./use-extension-actions";
import { useBundleActivations, useExtensionDetail } from "./use-extensions";

export function useExtensionDetailState(name: string) {
  const detail = useExtensionDetail(name);
  const bundles = useBundleActivations();
  const toggle = useToggleExtension();
  const update = useUpdateExtension();
  const navigate = useNavigate();
  const [provenanceOpen, setProvenanceOpen] = useState(false);
  const [removeOpen, setRemoveOpen] = useState(false);
  return {
    bundles,
    detail,
    navigate,
    provenanceOpen,
    removeOpen,
    setProvenanceOpen,
    setRemoveOpen,
    toggle,
    update,
  };
}
