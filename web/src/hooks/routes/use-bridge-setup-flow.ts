import { useSelector } from "@xstate/store-react";
import { useEffect } from "react";

import {
  useRegisterBridgeWebhook,
  useVerifyBridge,
  fingerprintBridgeProviderConfig,
  type BridgeHealth,
  type BridgeProvider,
  type BridgeSecretBinding,
  type BridgeSummary,
} from "@/systems/bridges";
import { projectBridgeSetup } from "@/systems/bridges/lib/bridge-setup";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { createBridgeSetupFlowLogic } from "./bridge-setup-flow-store";

interface BridgeSetupFlowInput {
  bindings: BridgeSecretBinding[];
  bridge: BridgeSummary | undefined;
  health: BridgeHealth | undefined;
  provider: BridgeProvider | undefined;
}

interface BridgeSetupEvidenceFingerprints {
  registration: string;
  verification: string;
}

function setupEvidenceFingerprints(
  bridge: BridgeSummary | undefined,
  bindings: BridgeSecretBinding[],
  provider: BridgeProvider | undefined
): BridgeSetupEvidenceFingerprints {
  if (!bridge) return { registration: "", verification: "" };
  const bindingState: [string, string, string][] = [];
  for (const binding of bindings) {
    if (binding.bridge_instance_id !== bridge.id) continue;
    bindingState.push([binding.binding_name, binding.secret_ref, binding.updated_at]);
  }
  bindingState.sort(([left], [right]) => left.localeCompare(right));
  const providerState = provider
    ? {
        configSchema: provider.config_schema ?? null,
        enabled: provider.enabled,
        extensionName: provider.extension_name,
        health: provider.health,
        platform: provider.platform,
        secretSlots: [...(provider.secret_slots ?? [])].sort((left, right) =>
          left.name.localeCompare(right.name)
        ),
        state: provider.state,
      }
    : null;
  const registrationState = {
    bindings: bindingState,
    bridge: {
      extensionName: bridge.extension_name,
      id: bridge.id,
      platform: bridge.platform,
      providerConfig: fingerprintBridgeProviderConfig(bridge.provider_config),
    },
  };
  return {
    registration: JSON.stringify(registrationState),
    verification: JSON.stringify({
      ...registrationState,
      enabled: bridge.enabled,
      provider: providerState,
    }),
  };
}

export function useBridgeSetupFlow({ bindings, bridge, health, provider }: BridgeSetupFlowInput) {
  const verifyMutation = useVerifyBridge();
  const registerMutation = useRegisterBridgeWebhook();
  const fingerprints = setupEvidenceFingerprints(bridge, bindings, provider);
  const bridgeId = bridge?.id ?? null;
  const registrationFingerprint = fingerprints.registration;
  const verificationFingerprint = fingerprints.verification;
  const input = {
    bridgeId,
    registrationFingerprint,
    verificationFingerprint,
  };
  const bindingKey = bridge?.id ?? "no-bridge";
  const { store } = useStoreBinding(bindingKey, () =>
    createBridgeSetupFlowLogic().createStore(input)
  );
  const state = useSelector(store, snapshot => snapshot.context);

  useEffect(() => {
    store.trigger.factsChanged({
      bridgeId,
      registrationFingerprint,
      verificationFingerprint,
    });
  }, [bridgeId, registrationFingerprint, store, verificationFingerprint]);

  const evidenceMatchesBridge = state.bridgeId !== null && state.bridgeId === bridge?.id;
  const currentRegistration =
    evidenceMatchesBridge && state.registrationFingerprint === fingerprints.registration
      ? state.registration
      : null;
  const currentVerification =
    evidenceMatchesBridge && state.verificationFingerprint === fingerprints.verification
      ? state.verification
      : null;
  const projection = bridge
    ? projectBridgeSetup({
        bindings,
        bridge,
        health: health ?? null,
        provider: provider ?? null,
        registration: currentRegistration,
        verification: currentVerification,
      })
    : null;

  return {
    clearEvidence: () => store.trigger.evidenceCleared(),
    clearVerification: () => store.trigger.verificationCleared(),
    isRegistering: state.phase === "registering",
    isVerifying: state.phase === "verifying",
    projection,
    registerWebhook: () => {
      if (bridge) {
        store.trigger.registrationRequested({
          bridgeId: bridge.id,
          bridgeName: bridge.display_name,
          register: bridgeId =>
            registerMutation.mutateAsync({ id: bridgeId, profile: bridge.profile_name }),
        });
      }
    },
    verify: () => {
      if (bridge) {
        store.trigger.verificationRequested({
          bridgeId: bridge.id,
          bridgeName: bridge.display_name,
          verify: bridgeId =>
            verifyMutation.mutateAsync({ id: bridgeId, profile: bridge.profile_name }),
        });
      }
    },
  };
}
