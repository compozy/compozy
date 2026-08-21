import { useState } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { useInstallMarketplaceExtension } from "../hooks/use-marketplace-actions";
import type { ExtensionInstallRequest } from "../types";
import { ExtensionInstallDialog } from "./extension-install-dialog";
import { createExtensionInstallLogic } from "./extension-install-dialog-store";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { previewExtensionInstall, type ExtensionInstallPreview } from "@/systems/extensions";

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
  const [preview, setPreview] = useState<ExtensionInstallPreview | null>(null);
  const [previewRequest, setPreviewRequest] = useState<ExtensionInstallRequest | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewPending, setPreviewPending] = useState(false);

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
        error={state.phase === "form" ? (previewError ?? state.error) : null}
        onFormChange={() => {
          setPreview(null);
          setPreviewRequest(null);
          setPreviewError(null);
        }}
        onOpenChange={open => {
          if (!open) {
            setPreview(null);
            setPreviewRequest(null);
            setPreviewError(null);
            store.trigger.installClosed();
          }
        }}
        onSubmit={request => {
          if (preview && previewRequest && requestsMatch(previewRequest, request)) {
            store.trigger.installRequested({
              execute,
              request: {
                ...request,
                ...(preview.network_requirement_digest
                  ? { confirm_network_digest: preview.network_requirement_digest }
                  : {}),
              },
            });
            return;
          }
          setPreview(null);
          setPreviewRequest(null);
          setPreviewPending(true);
          setPreviewError(null);
          void previewExtensionInstall(request)
            .then(result => {
              setPreview(result);
              setPreviewRequest(request);
            })
            .catch((error: unknown) => {
              setPreviewError(
                error instanceof Error ? error.message : "The extension could not be previewed."
              );
            })
            .finally(() => setPreviewPending(false));
        }}
        open={state.phase === "form" || (state.phase === "submitting" && !state.consented)}
        pending={previewPending || state.phase === "submitting"}
        preview={preview}
      />
      <ExtensionTrustDialog
        action="install"
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

function requestsMatch(left: ExtensionInstallRequest, right: ExtensionInstallRequest): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}
