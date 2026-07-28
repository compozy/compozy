import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useDeactivateBundle, useUpdateBundleActivation } from "./use-extension-actions";
import { useBundleActivation } from "./use-extensions";

export function useBundleActivationDetail(id: string) {
  const query = useBundleActivation(id);
  const update = useUpdateBundleActivation();
  const deactivate = useDeactivateBundle();
  const navigate = useNavigate();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmNetworkRequirement, setConfirmNetworkRequirement] = useState(false);
  const activation = query.data;

  const applyUpdate = () => {
    if (!activation) return;
    update.mutate({
      id,
      body: {
        expected_version: activation.version,
        ...(confirmNetworkRequirement ? { confirm_network_requirement: true } : {}),
      },
    });
  };

  const confirmDeactivate = async () => {
    await deactivate.mutateAsync(id);
    setDialogOpen(false);
    void navigate({ search: { tab: "installed" }, to: "/marketplace/bundles" });
  };

  return {
    activation,
    applyUpdate,
    confirmDeactivate,
    confirmNetworkRequirement,
    deactivate,
    dialogOpen,
    query,
    setConfirmNetworkRequirement,
    setDialogOpen,
    update,
  };
}
