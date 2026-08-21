import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import {
  compactBridgeDeliveryDefaults,
  useSendBridgeTest,
  useTestBridgeDelivery,
  type BridgeSummary,
  type BridgeTestDeliveryDraft,
} from "@/systems/bridges";

import { bridgeDeliveryTestsLogic } from "./bridge-delivery-tests-store";

function optionalMessage(value: string): string | undefined {
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

/** Store-owned delivery checks sharing the single target draft rendered by edit-Advanced. */
export function useBridgeDeliveryTests(bridge: BridgeSummary | undefined) {
  const dryRunMutation = useTestBridgeDelivery();
  const sendTestMutation = useSendBridgeTest();
  const store = useStore(bridgeDeliveryTestsLogic);
  const flow = useSelector(store, snapshot => snapshot.context);

  const executeDryRun = (targetBridge: BridgeSummary, draft: BridgeTestDeliveryDraft) =>
    dryRunMutation.mutateAsync({
      id: targetBridge.id,
      profile: targetBridge.profile_name,
      data: {
        message: optionalMessage(draft.message),
        target: {
          bridge_instance_id: targetBridge.id,
          ...compactBridgeDeliveryDefaults(draft.target),
        },
      },
    });
  const executeSendTest = (targetBridge: BridgeSummary, draft: BridgeTestDeliveryDraft) =>
    sendTestMutation.mutateAsync({
      id: targetBridge.id,
      profile: targetBridge.profile_name,
      data: {
        message: draft.message.trim(),
        target: {
          bridge_instance_id: targetBridge.id,
          ...compactBridgeDeliveryDefaults(draft.target),
        },
      },
    });

  const submitSendTest = () => {
    if (!bridge?.enabled) {
      toast.error("Enable the bridge before sending a real test message.");
      return;
    }
    if (flow.draft.message.trim() === "") {
      toast.error("Enter a message before sending a real bridge test.");
      return;
    }
    store.trigger.sendTestSubmitted({ bridge, execute: executeSendTest });
  };

  return {
    resetDraft: () => store.trigger.reset({ bridge }),
    panelProps: {
      bridgeEnabled: Boolean(bridge?.enabled),
      draft: flow.draft,
      dryRunResult: flow.dryRun.phase === "resolved" ? flow.dryRun.result : null,
      isDryRunPending: flow.dryRun.phase === "submitting",
      isSendTestPending: flow.sendTest.phase === "submitting",
      onDraftChange: (draft: BridgeTestDeliveryDraft) => store.trigger.draftChanged({ draft }),
      onDryRun: () => store.trigger.dryRunSubmitted({ bridge, execute: executeDryRun }),
      onSendTest: submitSendTest,
      sendTestResult: flow.sendTest.phase === "resolved" ? flow.sendTest.result : null,
    },
  };
}
