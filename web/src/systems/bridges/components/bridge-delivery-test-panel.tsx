import { SearchCheck, SendHorizontal } from "lucide-react";

import {
  ActionResultBanner,
  Button,
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
  FormSection,
  Input,
  MetadataList,
  NativeSelect,
  NativeSelectOption,
  Pill,
  Section,
  Spinner,
  Textarea,
  type PillTone,
} from "@agh/ui";

import { describeBridgeTestTarget } from "../lib/bridge-formatters";
import type {
  BridgeTestDeliveryDraft,
  SendBridgeTestResponse,
  TestBridgeDeliveryResponse,
} from "../types";

export interface BridgeDeliveryTestPanelProps {
  bridgeName?: string;
  /** A real send needs an enabled bridge; a dry run does not. */
  bridgeEnabled: boolean;
  draft: BridgeTestDeliveryDraft;
  onDraftChange: (draft: BridgeTestDeliveryDraft) => void;
  onDryRun: () => void;
  onSendTest: () => void;
  dryRunResult: TestBridgeDeliveryResponse | null;
  sendTestResult: SendBridgeTestResponse | null;
  isDryRunPending: boolean;
  isSendTestPending: boolean;
}

function resultTone(status: string): PillTone {
  switch (status) {
    case "resolved":
    case "ready":
    case "delivered":
      return "success";
    case "error":
    case "failed":
      return "danger";
    case "pending":
    case "committed_result_unavailable":
      return "warning";
    default:
      return "neutral";
  }
}

function updateTargetField(
  draft: BridgeTestDeliveryDraft,
  field: "group_id" | "peer_id" | "thread_id",
  value: string
): BridgeTestDeliveryDraft {
  return { ...draft, target: { ...draft.target, [field]: value } };
}

function DryRunResult({ result }: { result: TestBridgeDeliveryResponse }) {
  return (
    <Section
      data-testid="bridge-test-delivery-result"
      label="Resolved target"
      right={
        <Pill mono tone={resultTone(result.status)}>
          {result.status}
        </Pill>
      }
    >
      <p className="text-small-body text-fg">{describeBridgeTestTarget(result.delivery_target)}</p>
      {result.message ? (
        <p className="mt-2 text-small-body leading-relaxed text-muted">Message: {result.message}</p>
      ) : null}
    </Section>
  );
}

function SendTestResult({ result }: { result: SendBridgeTestResponse }) {
  const committedResultUnavailable = result.status === "committed_result_unavailable";
  const committedResultMessage =
    result.error?.message ??
    "The provider accepted the mutation but did not return a verifiable result.";
  const committedResultSentence = /[.!?]$/.test(committedResultMessage)
    ? committedResultMessage
    : `${committedResultMessage}.`;

  return (
    <Section
      data-testid="bridge-send-test-result"
      label="Delivery result"
      right={
        <Pill mono tone={resultTone(result.status)}>
          {result.status}
        </Pill>
      }
    >
      <MetadataList className="grid gap-3 sm:grid-cols-2">
        <MetadataList.Row label="Delivery ID">
          <span className="break-all font-mono">{result.delivery_id}</span>
        </MetadataList.Row>
        <MetadataList.Row label="Target">
          {describeBridgeTestTarget(result.delivery_target)}
        </MetadataList.Row>
        {result.remote_message_id ? (
          <MetadataList.Row label="Remote message ID">
            <span className="break-all font-mono">{result.remote_message_id}</span>
          </MetadataList.Row>
        ) : null}
      </MetadataList>
      {committedResultUnavailable ? (
        <ActionResultBanner
          className="mt-4"
          description={`${committedResultSentence} Inspect the provider before sending this test again.`}
          title="Delivery result is indeterminate"
          tone="warning"
        />
      ) : null}
    </Section>
  );
}

/**
 * Delivery test folded into edit-bridge Advanced (D2).
 *
 * Both intents share one target draft: a dry run resolves the target with no
 * provider side effect, a send delivers one real message. This is the single
 * entry point — the standalone dialog it replaced is deleted, so the two paths
 * can no longer disagree about which target they are testing.
 */
export function BridgeDeliveryTestPanel({
  bridgeName,
  bridgeEnabled,
  draft,
  onDraftChange,
  onDryRun,
  onSendTest,
  dryRunResult,
  sendTestResult,
  isDryRunPending,
  isSendTestPending,
}: BridgeDeliveryTestPanelProps) {
  const missingMessage = draft.message.trim() === "";

  return (
    <FormSection
      data-testid="bridge-delivery-test-panel"
      description={`Resolve a target without provider side effects, or send one real message through ${bridgeName ?? "this bridge"}.`}
      icon={SearchCheck}
      title="Delivery test"
    >
      <Field>
        <FieldContent>
          <FieldTitle>Message</FieldTitle>
          <FieldDescription>
            Optional for a dry run — it is echoed with the resolved target. Required to send a real
            message.
          </FieldDescription>
        </FieldContent>
        <Textarea
          data-testid="test-delivery-message"
          onChange={event => onDraftChange({ ...draft, message: event.target.value })}
          placeholder="Deliver a short operator ping."
          value={draft.message}
        />
      </Field>

      <div className="grid gap-4 lg:grid-cols-2">
        <Field>
          <FieldContent>
            <FieldTitle>Mode</FieldTitle>
          </FieldContent>
          <NativeSelect
            data-testid="test-delivery-mode-select"
            onChange={event =>
              onDraftChange({
                ...draft,
                target: {
                  ...draft.target,
                  mode:
                    event.target.value === ""
                      ? undefined
                      : (event.target.value as NonNullable<typeof draft.target.mode>),
                },
              })
            }
            value={draft.target.mode ?? ""}
          >
            <NativeSelectOption value="">Use bridge default</NativeSelectOption>
            <NativeSelectOption value="reply">Reply</NativeSelectOption>
            <NativeSelectOption value="direct-send">Direct send</NativeSelectOption>
          </NativeSelect>
        </Field>
        {(
          [
            ["peer_id", "Peer ID", "peer_123", "test-delivery-peer-input"],
            ["thread_id", "Thread ID", "thread_456", "test-delivery-thread-input"],
            ["group_id", "Group ID", "group_789", "test-delivery-group-input"],
          ] as const
        ).map(([field, label, placeholder, testId]) => (
          <Field key={field}>
            <FieldContent>
              <FieldTitle>{label}</FieldTitle>
            </FieldContent>
            <Input
              data-testid={testId}
              onChange={event => onDraftChange(updateTargetField(draft, field, event.target.value))}
              placeholder={placeholder}
              value={draft.target[field] ?? ""}
            />
          </Field>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Button
          data-testid="submit-test-delivery"
          disabled={isDryRunPending}
          onClick={onDryRun}
          size="sm"
          type="button"
          variant="outline"
        >
          {isDryRunPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
          {isDryRunPending ? "Checking…" : "Check target"}
        </Button>
        <Button
          data-testid="submit-send-test"
          disabled={isSendTestPending || missingMessage || !bridgeEnabled}
          onClick={onSendTest}
          size="sm"
          type="button"
          variant="outline"
        >
          {isSendTestPending ? (
            <Spinner aria-hidden="true" className="size-3" />
          ) : (
            <SendHorizontal className="size-3" />
          )}
          {isSendTestPending ? "Sending…" : "Send test message"}
        </Button>
        {bridgeEnabled ? null : (
          <p className="text-form-hint text-subtle" data-testid="bridge-send-test-disabled-hint">
            Enable the bridge to send a real message.
          </p>
        )}
      </div>

      {dryRunResult ? <DryRunResult result={dryRunResult} /> : null}
      {sendTestResult ? <SendTestResult result={sendTestResult} /> : null}
    </FormSection>
  );
}
