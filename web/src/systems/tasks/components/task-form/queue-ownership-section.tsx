import { useId } from "react";

import {
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  PillGroup,
  type PillGroupItem,
} from "@agh/ui";

import type { TaskOwnerKind } from "../../types";

interface OwnerKindOption {
  value: TaskOwnerKind;
  label: string;
  placeholder: string;
  description: string;
}

const UNASSIGNED_OWNER_DESCRIPTION =
  "Leave ownership empty unless a specific agent, session, human, automation, extension, or peer owns the work.";

const OWNER_KIND_OPTIONS: OwnerKindOption[] = [
  {
    value: "pool",
    label: "Agent / pool",
    placeholder: "Agent name or pool id (e.g. landing_builder)",
    description:
      "Use an agent name or worker-pool id. Matching agent sessions can claim queued runs.",
  },
  {
    value: "agent_session",
    label: "Exact session",
    placeholder: "Session id (e.g. sess-...)",
    description: "Use the exact session id. Agent names belong under Agent / pool.",
  },
  {
    value: "human",
    label: "Human",
    placeholder: "Human id or handle (e.g. pedro)",
    description: "Use this when a human operator owns the task.",
  },
  {
    value: "automation",
    label: "Automation",
    placeholder: "Automation id",
    description: "Use this when a daemon automation owns the task.",
  },
  {
    value: "extension",
    label: "Extension",
    placeholder: "Extension id",
    description: "Use this when an installed extension owns the task.",
  },
  {
    value: "network_peer",
    label: "Network peer",
    placeholder: "Peer id",
    description: "Use this when a Network peer owns the task.",
  },
];

const ATTEMPT_VALUES = [1, 2, 3, 5] as const;
const ATTEMPT_ITEMS: PillGroupItem<string>[] = ATTEMPT_VALUES.map(value => ({
  value: String(value),
  label: <span className="font-mono tabular-nums">{value}</span>,
  testId: `task-attempts-${value}`,
}));

const APPROVAL_OPTIONS: PillGroupItem<"none" | "manual">[] = [
  { value: "none", label: "No approval", testId: "task-approval-none" },
  { value: "manual", label: "Human-in-the-loop", testId: "task-approval-manual" },
];

const APPROVAL_HINT_MANUAL =
  "The first run waits for approval in your Inbox before an owner can claim it.";
const APPROVAL_HINT_RETRY =
  "A run is retried up to its max attempts before the task is marked failed.";

function resolveOwnerKindOption(kind: TaskOwnerKind | ""): OwnerKindOption | null {
  if (!kind) {
    return null;
  }
  return OWNER_KIND_OPTIONS.find(option => option.value === kind) ?? null;
}

function resolveAttemptsValue(maxAttempts: number | null): string {
  if (typeof maxAttempts === "number" && ATTEMPT_VALUES.includes(maxAttempts as 1 | 2 | 3 | 5)) {
    return String(maxAttempts);
  }
  return "1";
}

interface QueueOwnershipSectionProps {
  ownerKind: TaskOwnerKind | "";
  ownerRef: string;
  maxAttempts: number | null;
  approvalPolicy: "none" | "manual";
  onOwnerKind: (kind: TaskOwnerKind | "") => void;
  onOwnerRef: (value: string) => void;
  onMaxAttempts: (value: number) => void;
  onApprovalPolicy: (policy: "none" | "manual") => void;
}

/**
 * Queue & ownership — who runs the task and how retries / approval behave.
 * Owner kind drives the ref placeholder and help line; the ref input is
 * disabled until a kind is chosen. The approval hint flips between the
 * retry contract and the manual-approval gate.
 */
export function QueueOwnershipSection({
  ownerKind,
  ownerRef,
  maxAttempts,
  approvalPolicy,
  onOwnerKind,
  onOwnerRef,
  onMaxAttempts,
  onApprovalPolicy,
}: QueueOwnershipSectionProps) {
  const ownerHelpId = useId();
  const ownerKindOption = resolveOwnerKindOption(ownerKind);
  const ownerDescription = ownerKindOption?.description ?? UNASSIGNED_OWNER_DESCRIPTION;
  const ownerRefPlaceholder = ownerKindOption?.placeholder ?? "Select an owner kind first";
  const ownerRefDisabled = ownerKind === "";
  const attemptsValue = resolveAttemptsValue(maxAttempts);
  const approvalHint = approvalPolicy === "manual" ? APPROVAL_HINT_MANUAL : APPROVAL_HINT_RETRY;

  return (
    <div className="flex flex-col gap-4">
      <Field>
        <FieldLabel htmlFor="task-owner-kind">
          Owner
          <span className="font-normal text-faint"> (leave unassigned to let a pool claim it)</span>
        </FieldLabel>
        <NativeSelect
          aria-describedby={ownerHelpId}
          aria-label="Owner kind"
          className="w-full"
          data-testid="task-owner-kind"
          id="task-owner-kind"
          onChange={event => onOwnerKind(event.target.value as TaskOwnerKind | "")}
          value={ownerKind}
        >
          <NativeSelectOption value="">Unassigned</NativeSelectOption>
          {OWNER_KIND_OPTIONS.map(option => (
            <NativeSelectOption key={option.value} value={option.value}>
              {option.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <FieldDescription data-testid="task-owner-help" id={ownerHelpId}>
          {ownerDescription}
        </FieldDescription>
        <Input
          aria-describedby={ownerHelpId}
          aria-label="Owner reference"
          className="mt-2 font-mono"
          data-testid="task-owner-ref"
          disabled={ownerRefDisabled}
          onChange={event => onOwnerRef(event.target.value)}
          placeholder={ownerRefPlaceholder}
          value={ownerRef}
        />
      </Field>

      <div className="grid gap-4 md:grid-cols-2">
        <Field>
          <FieldLabel>Max attempts</FieldLabel>
          <PillGroup
            aria-label="Max attempts"
            className="w-full flex-wrap"
            data-testid="task-attempts-options"
            items={ATTEMPT_ITEMS}
            onChange={next => onMaxAttempts(Number(next))}
            size="sm"
            value={attemptsValue}
          />
        </Field>

        <Field>
          <FieldLabel>Approval</FieldLabel>
          <PillGroup
            aria-label="Approval policy"
            className="w-full flex-wrap"
            data-testid="task-approval-value"
            items={APPROVAL_OPTIONS}
            onChange={onApprovalPolicy}
            size="sm"
            value={approvalPolicy}
          />
        </Field>
      </div>

      <p className="text-form-hint text-subtle" data-testid="task-approval-hint">
        {approvalHint}
      </p>
    </div>
  );
}
