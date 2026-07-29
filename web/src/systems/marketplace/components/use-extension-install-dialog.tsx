import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { useInstallMarketplaceExtension } from "../hooks/use-marketplace-actions";
import type { ExtensionInstallRequest } from "../types";
import { ExtensionInstallDialog } from "./extension-install-dialog";
import { createExtensionInstallLogic } from "./extension-install-dialog-store";
import { ExtensionTrustDialog } from "./extension-trust-dialog";

const extensionInstallLogic = createExtensionInstallLogic();

export interface ExtensionInstallDialogController {
  dialogs: React.ReactNode;
  isOpen: boolean;
  open: () => void;
}

/**
 * Owns the source-union install entry point: the form gate, the daemon's explicit consent gate for
 * unverified archives, and the daemon's own validation messages on failure.
 */
export function useExtensionInstallDialog(
  options: {
    onInstalled?: () => void;
  } = {}
): ExtensionInstallDialogController {
  const store = useStore(extensionInstallLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const install = useInstallMarketplaceExtension();

  const execute = async (request: ExtensionInstallRequest) => {
    await install.mutateAsync(request);
    toast.success(`${request.ref} installed`, {
      action: options.onInstalled
        ? { label: "View installed →", onClick: () => options.onInstalled?.() }
        : undefined,
    });
  };

  const consentSubject = state.phase === "consent" ? state.request.ref : "";
  const dialogs = (
    <>
      <ExtensionInstallDialog
        error={state.phase === "form" ? state.error : null}
        onOpenChange={open => {
          if (!open) store.trigger.installClosed();
        }}
        onSubmit={request => store.trigger.installRequested({ execute, request })}
        open={state.phase === "form" || (state.phase === "submitting" && !state.consented)}
        pending={state.phase === "submitting"}
      />
      <ExtensionTrustDialog
        description={
          state.phase === "consent" && state.reason
            ? state.reason
            : "This archive is not registry-verified. Installing it runs code with the permissions it declares."
        }
        error={state.phase === "consent" ? state.error : null}
        name={consentSubject}
        onConfirm={() => store.trigger.installConsentRequested({ execute })}
        onOpenChange={open => {
          if (!open) store.trigger.installClosed();
        }}
        open={state.phase === "consent" || (state.phase === "submitting" && state.consented)}
        pending={state.phase === "submitting"}
      />
    </>
  );

  return {
    dialogs,
    isOpen: state.phase !== "closed",
    open: () => store.trigger.installOpened(),
  };
}
