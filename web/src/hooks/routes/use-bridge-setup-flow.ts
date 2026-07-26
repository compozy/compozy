import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import {
  useRegisterBridgeWebhook,
  useVerifyBridge,
  fingerprintBridgeProviderConfig,
  type BridgeHealth,
  type BridgeProvider,
  type BridgeSecretBinding,
  type BridgeSummary,
  type BridgeVerifyResponse,
  type BridgeWebhookRegistrationResponse,
} from "@/systems/bridges";
import { projectBridgeSetup, summarizeBridgeCheckStatus } from "@/systems/bridges/lib/bridge-setup";

interface BridgeSetupFlowInput {
  bindings: BridgeSecretBinding[];
  bridge: BridgeSummary | undefined;
  health: BridgeHealth | undefined;
  provider: BridgeProvider | undefined;
}

interface BridgeSetupEvidence {
  bridgeId: string;
  registrationFingerprint: string;
  registration: BridgeWebhookRegistrationResponse | null;
  verificationFingerprint: string;
  verification: BridgeVerifyResponse | null;
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
  const [evidence, setEvidence] = useState<BridgeSetupEvidence | null>(null);
  const operationRef = useRef(0);
  const verifyMutation = useVerifyBridge();
  const registerMutation = useRegisterBridgeWebhook();

  useEffect(
    () => () => {
      operationRef.current += 1;
    },
    []
  );

  const fingerprints = setupEvidenceFingerprints(bridge, bindings, provider);
  const registrationRevisionRef = useRef(fingerprints.registration);
  const verificationRevisionRef = useRef(fingerprints.verification);
  useEffect(() => {
    registrationRevisionRef.current = fingerprints.registration;
    verificationRevisionRef.current = fingerprints.verification;
  }, [fingerprints.registration, fingerprints.verification]);
  const evidenceMatchesBridge = evidence !== null && evidence.bridgeId === bridge?.id;
  const currentRegistration =
    evidenceMatchesBridge && evidence.registrationFingerprint === fingerprints.registration
      ? evidence.registration
      : null;
  const currentVerification =
    evidenceMatchesBridge && evidence.verificationFingerprint === fingerprints.verification
      ? evidence.verification
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

  const clearEvidence = () => {
    operationRef.current += 1;
    setEvidence(null);
  };

  const clearVerification = () => {
    operationRef.current += 1;
    setEvidence(current =>
      current === null
        ? null
        : {
            ...current,
            verification: null,
            verificationFingerprint: verificationRevisionRef.current,
          }
    );
  };

  const verify = async () => {
    if (!bridge) return;
    const operation = operationRef.current + 1;
    operationRef.current = operation;
    const operationFingerprint = fingerprints.verification;

    setEvidence(current => ({
      bridgeId: bridge.id,
      registrationFingerprint: fingerprints.registration,
      registration:
        current?.bridgeId === bridge.id &&
        current.registrationFingerprint === fingerprints.registration
          ? current.registration
          : null,
      verificationFingerprint: fingerprints.verification,
      verification: null,
    }));

    try {
      const result = await verifyMutation.mutateAsync({ id: bridge.id });
      if (
        operationRef.current !== operation ||
        verificationRevisionRef.current !== operationFingerprint
      ) {
        return;
      }
      if (result.bridge_instance_id !== bridge.id) {
        toast.error("The verification response did not match the selected bridge.");
        return;
      }
      setEvidence(current => ({
        bridgeId: bridge.id,
        registrationFingerprint: fingerprints.registration,
        registration:
          current?.bridgeId === bridge.id &&
          current.registrationFingerprint === fingerprints.registration
            ? current.registration
            : null,
        verificationFingerprint: fingerprints.verification,
        verification: result,
      }));
      toastVerificationResult(summarizeBridgeCheckStatus(result.checks), bridge.display_name);
    } catch (error) {
      if (
        operationRef.current !== operation ||
        verificationRevisionRef.current !== operationFingerprint
      ) {
        return;
      }
      toast.error(error instanceof Error ? error.message : "Failed to verify bridge");
    }
  };

  const registerWebhook = async () => {
    if (!bridge) return;
    const operation = operationRef.current + 1;
    operationRef.current = operation;
    const operationFingerprint = fingerprints.registration;

    setEvidence({
      bridgeId: bridge.id,
      registrationFingerprint: fingerprints.registration,
      registration: null,
      verificationFingerprint: fingerprints.verification,
      verification: null,
    });

    try {
      const result = await registerMutation.mutateAsync({ id: bridge.id });
      if (
        operationRef.current !== operation ||
        registrationRevisionRef.current !== operationFingerprint
      ) {
        return;
      }
      if (result.bridge_instance_id !== bridge.id) {
        toast.error("The registration response did not match the selected bridge.");
        return;
      }
      setEvidence({
        bridgeId: bridge.id,
        registrationFingerprint: fingerprints.registration,
        registration: result,
        verificationFingerprint: fingerprints.verification,
        verification: null,
      });
      toastRegistrationResult(result, bridge.display_name);
    } catch (error) {
      if (
        operationRef.current !== operation ||
        registrationRevisionRef.current !== operationFingerprint
      ) {
        return;
      }
      toast.error(error instanceof Error ? error.message : "Failed to register bridge webhook");
    }
  };

  return {
    clearEvidence,
    clearVerification,
    isRegistering: registerMutation.isPending,
    isVerifying: verifyMutation.isPending,
    projection,
    registerWebhook,
    verify,
  };
}

function toastVerificationResult(
  status: ReturnType<typeof summarizeBridgeCheckStatus>,
  bridgeName: string
) {
  switch (status) {
    case "pass":
      toast.success(`Verified bridge ${bridgeName}.`);
      return;
    case "warn":
      toast.warning(`Verification completed with warnings for ${bridgeName}.`);
      return;
    case "fail":
      toast.error(`Verification found setup issues for ${bridgeName}.`);
      return;
    case "skipped":
      toast.info(`Verification checks were skipped for ${bridgeName}.`);
      return;
    case null:
      toast.warning(`Verification returned no checks for ${bridgeName}.`);
  }
}

function toastRegistrationResult(result: BridgeWebhookRegistrationResponse, bridgeName: string) {
  switch (result.status) {
    case "pass":
      toast.success(`Registered the webhook for ${bridgeName}.`);
      return;
    case "warn":
      toast.warning(result.remediation || `Webhook registration is uncertain for ${bridgeName}.`);
      return;
    case "fail":
      toast.error(result.remediation || "Telegram webhook registration failed.");
      return;
    case "skipped":
      toast.info(result.remediation || `Webhook registration was skipped for ${bridgeName}.`);
  }
}
