import { useState } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "@compozy/ui";

import { useInstallMarketplaceExtension } from "../hooks/use-marketplace-actions";
import type { ExtensionInstallRequest } from "../types";
import { ExtensionDependencyGateDialog } from "./extension-dependency-gate-dialog";
import {
  blockedExtensionDependencies,
  extensionDependencyGates,
  extensionDependencyInstallFields,
} from "./extension-dependency-model";
import { ExtensionInstallDialog } from "./extension-install-dialog";
import type { ExtensionInstallSource } from "./extension-install-model";
import { createExtensionInstallLogic } from "./extension-install-dialog-store";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { useExtensionDependencyGates } from "./use-extension-dependency-gates";
import { previewExtensionInstall, type ExtensionInstallPreview } from "@/systems/extensions";

const extensionInstallLogic = createExtensionInstallLogic();

export interface ExtensionInstallDialogController {
  dialogs: React.ReactNode;
  isOpen: boolean;
  /** Opens the form on the given source; defaults to a local build. */
  open: (source?: ExtensionInstallSource) => void;
}

/**
 * Owns the source-union install entry point: the form gate, each previewed dependency's own trust
 * gates, the daemon's explicit consent gate for unverified archives, and the daemon's own validation
 * messages on failure.
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
  const [initialSource, setInitialSource] = useState<ExtensionInstallSource>("local_path");
  // Each open mounts a fresh form: a success closes through the store, never through the form.
  const [formSession, setFormSession] = useState(0);
  const dependencyGates = useExtensionDependencyGates();

  const resetPreview = () => {
    setPreview(null);
    setPreviewRequest(null);
    setPreviewError(null);
  };

  const execute = async (request: ExtensionInstallRequest) => {
    const result = await install.mutateAsync(request);
    resetPreview();
    const alsoInstalled = result.dependencies_installed ?? [];
    const message =
      alsoInstalled.length > 0
        ? `${request.ref} installed with ${alsoInstalled.join(", ")}`
        : `${request.ref} installed`;
    toast.success(message, {
      action: options.onInstalled
        ? { label: "View installed →", onClick: () => options.onInstalled?.() }
        : undefined,
    });
  };

  /**
   * Installs the previewed artifact with its dependencies in one request. Each dependency passes its
   * own gates first, in preview order; the root's unverified opt-in never covers one, and a cancel
   * aborts the whole install before anything is sent.
   */
  const installPreviewed = (previewed: ExtensionInstallPreview, form: ExtensionInstallRequest) => {
    if (blockedExtensionDependencies(previewed).length > 0) return;
    const request: ExtensionInstallRequest = {
      ...form,
      ...extensionDependencyInstallFields(previewed),
      ...(previewed.digest_sha256 ? { expected_digest: previewed.digest_sha256 } : {}),
    };
    const gates = extensionDependencyGates(previewed);
    if (gates.length === 0) {
      store.trigger.installRequested({ execute, request });
      return;
    }
    void dependencyGates.request(gates).then(confirmed => {
      if (confirmed) {
        store.trigger.installRequested({ execute, request });
        return;
      }
      resetPreview();
      store.trigger.installClosed();
    });
  };

  const consentSubject = state.phase === "consent" ? state.request.ref : "";
  const dialogs = (
    <>
      <ExtensionInstallDialog
        error={state.phase === "form" ? (previewError ?? state.error) : null}
        initialSource={initialSource}
        key={`${initialSource}:${formSession}`}
        onFormChange={resetPreview}
        onOpenChange={open => {
          if (!open) {
            resetPreview();
            store.trigger.installClosed();
          }
        }}
        onSubmit={request => {
          if (preview && previewRequest && requestsMatch(previewRequest, request)) {
            installPreviewed(preview, request);
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
        open={
          // A dependency gate replaces the form on screen until it is confirmed or canceled.
          !dependencyGates.current &&
          (state.phase === "form" || (state.phase === "submitting" && !state.consented))
        }
        pending={previewPending || state.phase === "submitting"}
        preview={preview}
      />
      <ExtensionDependencyGateDialog
        gate={dependencyGates.current}
        onAccept={dependencyGates.accept}
        onCancel={dependencyGates.cancel}
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
    open: (source = "local_path") => {
      resetPreview();
      setInitialSource(source);
      setFormSession(session => session + 1);
      store.trigger.installOpened();
    },
  };
}

function requestsMatch(left: ExtensionInstallRequest, right: ExtensionInstallRequest): boolean {
  return (
    JSON.stringify({ ...left, inputs: undefined }) ===
    JSON.stringify({ ...right, inputs: undefined })
  );
}
