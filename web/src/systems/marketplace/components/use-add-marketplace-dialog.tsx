import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import type { MarketplaceSource } from "../types";
import { AddMarketplaceDialog } from "./add-marketplace-dialog";
import { pluralPlugins } from "./add-marketplace-model";

export interface AddMarketplaceDialogController {
  dialog: React.ReactNode;
  isOpen: boolean;
  open: () => void;
}

/**
 * The one entry point for registering a plugin marketplace, shared by Add ▾ in Browse and by
 * Settings › Marketplace. Success closes the dialog and offers "Show in Marketplace" through the
 * router, which the OS shell resolves to the Marketplace window wherever the dialog was opened.
 */
export function useAddMarketplaceDialog(): AddMarketplaceDialogController {
  const [isOpen, setIsOpen] = useState(false);
  const navigate = useNavigate();

  const announce = (source: MarketplaceSource) => {
    toast.success(`${source.name} added`, {
      description: `${pluralPlugins(source.plugins)} listed · ${source.installable} can be installed`,
      action: {
        label: "Show in Marketplace",
        onClick: () => void navigate({ search: {}, to: "/marketplace" }),
      },
    });
  };

  return {
    dialog: <AddMarketplaceDialog onAdded={announce} onOpenChange={setIsOpen} open={isOpen} />,
    isOpen,
    open: () => setIsOpen(true),
  };
}
