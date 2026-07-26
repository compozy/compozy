import { useState } from "react";
import { toast } from "sonner";

import {
  compactBridgeDeliveryDefaults,
  createBridgeTestDeliveryDraft,
  useSendBridgeTest,
  useTestBridgeDelivery,
  type BridgeSummary,
  type BridgeTestDeliveryDraft,
  type SendBridgeTestResponse,
  type TestBridgeDeliveryResponse,
} from "@/systems/bridges";

function optionalMessage(value: string): string | undefined {
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

/**
 * Delivery checks for one bridge, folded into edit-Advanced (D2).
 *
 * Both intents share a single target draft: the dry run and the real send are
 * two verbs against the same target, and two independent drafts let them drift
 * apart without the operator noticing.
 */
export function useBridgeDeliveryTests(bridge: BridgeSummary | undefined) {
  const [draft, setDraft] = useState<BridgeTestDeliveryDraft>(() =>
    createBridgeTestDeliveryDraft()
  );
  const [dryRunResult, setDryRunResult] = useState<TestBridgeDeliveryResponse | null>(null);
  const [sendTestResult, setSendTestResult] = useState<SendBridgeTestResponse | null>(null);

  const dryRunMutation = useTestBridgeDelivery();
  const sendTestMutation = useSendBridgeTest();

  /** Seeds the target from the bridge whenever the editor is opened. */
  const resetDraft = () => {
    setDraft(createBridgeTestDeliveryDraft(bridge));
    setDryRunResult(null);
    setSendTestResult(null);
  };

  const submitDryRun = async () => {
    if (!bridge) return;

    try {
      const result = await dryRunMutation.mutateAsync({
        id: bridge.id,
        data: {
          message: optionalMessage(draft.message),
          target: {
            bridge_instance_id: bridge.id,
            ...compactBridgeDeliveryDefaults(draft.target),
          },
        },
      });
      setDryRunResult(result);
      toast.success(`Resolved delivery target for ${bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to resolve bridge target");
    }
  };

  const submitSendTest = async () => {
    if (!bridge) return;
    if (!bridge.enabled) {
      toast.error("Enable the bridge before sending a real test message.");
      return;
    }

    const message = draft.message.trim();
    if (message === "") {
      toast.error("Enter a message before sending a real bridge test.");
      return;
    }

    try {
      const result = await sendTestMutation.mutateAsync({
        id: bridge.id,
        data: {
          message,
          target: {
            bridge_instance_id: bridge.id,
            ...compactBridgeDeliveryDefaults(draft.target),
          },
        },
      });
      setSendTestResult(result);
      toast.success(`Sent a test message through ${bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to send bridge test message");
    }
  };

  return {
    resetDraft,
    panelProps: {
      bridgeEnabled: Boolean(bridge?.enabled),
      draft,
      dryRunResult,
      isDryRunPending: dryRunMutation.isPending,
      isSendTestPending: sendTestMutation.isPending,
      onDraftChange: setDraft,
      onDryRun: submitDryRun,
      onSendTest: submitSendTest,
      sendTestResult,
    },
  };
}
