import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { extensionNetworkConfirmation, ExtensionsApiError } from "@/systems/extensions";
import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import type { ExtensionUpdateRequest } from "../types";
import {
  prepareExtensionInputs,
  type ExtensionInputDefinitions,
  type ExtensionInputDraft,
} from "./extension-install-model";
import { marketplaceErrorMessage } from "./marketplace-ui";

export interface MarketplaceUpdateRequest {
  body: ExtensionUpdateRequest;
  name: string;
}

type TrackUpdate = <T>(action: () => Promise<T>) => Promise<T>;
type UpdateTarget = { label: string; request: MarketplaceUpdateRequest; track: TrackUpdate };
type Recovery = UpdateTarget &
  (
    | { kind: "network"; digest: string }
    | { kind: "inputs"; definitions: ExtensionInputDefinitions }
  );
type RecoveryState = { phase: "idle" } | { phase: "review" | "submitting"; recovery: Recovery };

const recoveryLogic = createStoreLogic<
  RecoveryState,
  {
    recoveryRequired: { recovery: Recovery };
    recoveryDismissed: {};
    recoverySubmitted: {};
    recoveryFailed: {};
    recoveryCompleted: {};
  }
>({
  context: { phase: "idle" },
  on: {
    recoveryRequired: (_context, event) => ({ phase: "review", recovery: event.recovery }),
    recoveryDismissed: context => (context.phase === "submitting" ? undefined : { phase: "idle" }),
    recoverySubmitted: context =>
      context.phase === "review" ? { ...context, phase: "submitting" } : undefined,
    recoveryFailed: context =>
      context.phase === "submitting" ? { ...context, phase: "review" } : undefined,
    recoveryCompleted: () => ({ phase: "idle" }),
  },
});

export function useMarketplaceUpdateRecovery(
  mutate: (request: MarketplaceUpdateRequest) => Promise<unknown>,
  notifySuccess: (label: string, version: string | undefined) => void
) {
  const store = useStore(recoveryLogic);
  const state = useSelector(store, snapshot => snapshot.context);

  const runUpdate = async (
    label: string,
    request: MarketplaceUpdateRequest,
    track: TrackUpdate
  ): Promise<boolean> => {
    try {
      await mutate(request);
      return true;
    } catch (error) {
      const confirmation = extensionNetworkConfirmation(error);
      if (confirmation) {
        store.trigger.recoveryRequired({
          recovery: { kind: "network", digest: confirmation.digest, label, request, track },
        });
        return false;
      }
      const definitions = missingInputDefinitions(error);
      if (!definitions) throw error;
      store.trigger.recoveryRequired({
        recovery: { kind: "inputs", definitions, label, request, track },
      });
      return false;
    }
  };

  const confirm = (draft: ExtensionInputDraft = {}) => {
    const current = store.getSnapshot().context;
    if (current.phase !== "review") return;
    const { recovery } = current;
    const body = { ...recovery.request.body };
    if (recovery.kind === "network") body.confirm_network_digest = recovery.digest;
    else {
      const prepared = prepareExtensionInputs(recovery.definitions, draft);
      if (!prepared.valid) return;
      body.inputs = { ...body.inputs, ...prepared.inputs };
    }
    store.trigger.recoverySubmitted();
    void recovery
      .track(async () => {
        if (!(await runUpdate(recovery.label, { ...recovery.request, body }, recovery.track)))
          return;
        store.trigger.recoveryCompleted();
        notifySuccess(recovery.label, body.version);
      })
      .catch((error: unknown) => {
        toast.error(marketplaceErrorMessage(error, `Failed to update ${recovery.label}`));
      })
      .finally(() => store.trigger.recoveryFailed());
  };

  return {
    runUpdate,
    confirm,
    dismiss: () => store.trigger.recoveryDismissed(),
    pending: state.phase === "submitting",
    recovery: state.phase === "idle" ? null : state.recovery,
  };
}

function missingInputDefinitions(error: unknown): ExtensionInputDefinitions | undefined {
  if (!(error instanceof ExtensionsApiError) && !(error instanceof MarketplaceApiError))
    return undefined;
  const code = error instanceof ExtensionsApiError ? error.code : error.diagnosticCode;
  const definitions = error.inputDefinitions;
  if (code !== "extension_inputs_required" || !definitions?.length || !error.requiredInputs?.length)
    return undefined;
  const required = new Set(error.requiredInputs);
  const declared = new Set(definitions.map(input => input.id));
  if (!error.requiredInputs.every(id => declared.has(id))) return undefined;
  return definitions.filter(input => required.has(input.id));
}
