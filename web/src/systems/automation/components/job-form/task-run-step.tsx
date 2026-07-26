import { Info } from "lucide-react";

import { Field, FieldLabel, Input, NativeSelect, NativeSelectOption, Textarea } from "@agh/ui";
import {
  NetworkParticipationFields,
  networkParticipationDraftFromPayload,
  type NetworkParticipationDraft,
} from "@/systems/network";

import type { CreateAutomationJobRequest } from "../../types";

type TaskDraft = NonNullable<CreateAutomationJobRequest["task"]>;
type OwnerKind = NonNullable<TaskDraft["owner"]>["kind"];

interface TaskRunStepProps {
  disabled?: boolean;
  jobName: string;
  task: TaskDraft;
  onTaskTitle: (next: string) => void;
  onTaskDescription: (next: string) => void;
  onOwnerKind: (kind: OwnerKind | "") => void;
  onOwnerRef: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
}

const OWNER_KINDS: ReadonlyArray<{ value: OwnerKind; label: string }> = [
  { value: "pool", label: "Agent pool" },
  { value: "agent_session", label: "Agent session" },
  { value: "human", label: "Human" },
  { value: "automation", label: "Automation" },
  { value: "extension", label: "Extension" },
  { value: "network_peer", label: "Network peer" },
];

const OWNER_REF_PLACEHOLDER: Record<OwnerKind, string> = {
  pool: "pool name",
  agent_session: "session id",
  human: "human handle",
  automation: "automation id",
  extension: "extension id",
  network_peer: "peer id",
};

function ownerRefPlaceholder(kind: string): string {
  return (
    (OWNER_REF_PLACEHOLDER as Record<string, string | undefined>)[kind] ??
    "select an owner kind first"
  );
}

/** Task output path: the durable task the job materializes on each tick. */
export function TaskRunStep({
  disabled = false,
  jobName,
  task,
  onTaskTitle,
  onTaskDescription,
  onOwnerKind,
  onOwnerRef,
  onNetworkParticipationChange,
}: TaskRunStepProps) {
  const ownerKind = task.owner?.kind ?? "";

  return (
    <div className="space-y-4">
      <Field>
        <FieldLabel htmlFor="job-task-title">Task title</FieldLabel>
        <Input
          data-testid="job-task-title"
          id="job-task-title"
          onChange={event => onTaskTitle(event.target.value)}
          placeholder={jobName}
          value={task.title ?? ""}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="job-task-desc">Description</FieldLabel>
        <Textarea
          data-testid="job-task-desc"
          id="job-task-desc"
          onChange={event => onTaskDescription(event.target.value)}
          placeholder="What 'done' means for the materialized task."
          value={task.description ?? ""}
        />
      </Field>
      <NetworkParticipationFields
        allowedStrategies={["named", "run"]}
        disabled={disabled}
        onChange={onNetworkParticipationChange}
        testIdPrefix="job-task-participation"
        value={networkParticipationDraftFromPayload(task.network_participation)}
      />
      <Field>
        <FieldLabel htmlFor="job-owner-kind">Owner</FieldLabel>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-[170px_minmax(0,1fr)]">
          <NativeSelect
            data-testid="job-owner-kind"
            id="job-owner-kind"
            onChange={event => onOwnerKind(event.target.value as OwnerKind | "")}
            value={ownerKind}
          >
            <NativeSelectOption value="">Unassigned</NativeSelectOption>
            {OWNER_KINDS.map(option => (
              <NativeSelectOption key={option.value} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <Input
            aria-label="Owner reference"
            className="font-mono text-form-label"
            data-testid="job-owner-ref"
            disabled={ownerKind === ""}
            onChange={event => onOwnerRef(event.target.value)}
            placeholder={ownerRefPlaceholder(ownerKind)}
            value={task.owner?.ref ?? ""}
          />
        </div>
      </Field>
      <div className="flex items-start gap-2 text-form-hint leading-snug text-subtle">
        <Info aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
        <span>
          Each tick creates a fresh task run instead of a session; the run shows as delegated. The
          task owns its retries, so job-level retry is None.
        </span>
      </div>
    </div>
  );
}
