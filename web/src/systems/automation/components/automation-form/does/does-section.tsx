import { Bot, Repeat2, SquareCheck } from "lucide-react";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import type { AutomationFormDraft } from "../../../lib/automation-form-draft";
import type { AutomationDoes } from "../../../lib/automation-sentence";
import {
  AutomationChoiceCards,
  AutomationLockLine,
  type AutomationChoice,
} from "../automation-choice-cards";
import { AutomationFormSection } from "../automation-form-section";
import { AgentTarget } from "./agent-target";
import { TaskTarget } from "./task-target";
import type { AgentPayload } from "@/systems/agent";
import { LoopTargetFields } from "@/systems/loops";

const TASK_SCHEDULE_ONLY = "Only scheduled automations can create tasks";

const DOES_CHOICES: readonly AutomationChoice<AutomationDoes>[] = [
  {
    value: "agent",
    icon: Bot,
    title: "Ask an agent",
    description: "Send a message to an agent in a new session.",
  },
  {
    value: "loop",
    icon: Repeat2,
    title: "Start a Loop",
    description: "Run one of your Loops with set inputs.",
  },
  {
    value: "task",
    icon: SquareCheck,
    title: "Create a task",
    description: "Add a task for an agent pool or a person.",
  },
];

/** What the fixed target is, with a lock: `summarizer 🔒`. */
function fixedTargetName(draft: AutomationFormDraft, does: AutomationDoes): string | null {
  if (does === "agent") return draft.agent_name.trim() || null;
  if (does === "loop") return draft.loop_target?.loop_name.trim() || null;
  return null;
}

interface DoesSectionProps {
  agents: AgentPayload[];
  agentsError?: string | null;
  agentsLoading?: boolean;
  draft: AutomationFormDraft;
  form: AutomationFormModel;
  isPending: boolean;
  lockedLoop?: string;
  mode: "create" | "edit";
  number: number;
}

/** Does: ask an agent, start a Loop, or create a task (schedules only). */
export function DoesSection({
  agents,
  agentsError,
  agentsLoading,
  draft,
  form,
  isPending,
  lockedLoop,
  mode,
  number,
}: DoesSectionProps) {
  const targetLocked = mode === "edit" || lockedLoop !== undefined;
  const choices = DOES_CHOICES.map(choice => {
    if (targetLocked) {
      if (choice.value !== form.does) return { ...choice, lockedReason: "Locked" };
      const name = fixedTargetName(draft, choice.value);
      return name
        ? { ...choice, description: <AutomationLockLine>{name}</AutomationLockLine> }
        : choice;
    }
    if (choice.value === "task" && draft.start !== "schedule") {
      return { ...choice, lockedReason: TASK_SCHEDULE_ONLY };
    }
    return choice;
  });

  return (
    <AutomationFormSection
      data-testid="automation-form-does"
      description={
        mode === "edit"
          ? "The agent and the kind of target stay. The message can change."
          : "What should happen?"
      }
      number={number}
      title="Does"
    >
      <AutomationChoiceCards
        choices={choices}
        label="What it does"
        onChange={form.onDoes}
        testIdPrefix="automation-does"
        value={form.does}
      />
      <div
        className="rounded-lg bg-sunken p-3.5"
        data-testid={`automation-does-inset-${form.does}`}
      >
        {form.does === "agent" ? (
          <AgentTarget
            agent={draft.agent_name}
            agentLocked={mode === "edit"}
            agents={agents}
            agentsError={agentsError}
            agentsLoading={agentsLoading}
            form={form}
            prompt={draft.prompt}
            start={draft.start}
          />
        ) : null}
        {form.does === "loop" ? (
          <LoopTargetFields
            catalog={form.loopCatalog}
            identityDisabled={targetLocked}
            mode={mode}
            onChange={form.onLoopTargetChange}
            showMapping={draft.start !== "schedule"}
            value={form.loopTarget}
            workspaceId={form.loopWorkspaceId}
          />
        ) : null}
        {form.does === "task" && draft.task ? (
          <TaskTarget disabled={isPending} form={form} name={draft.name} task={draft.task} />
        ) : null}
      </div>
    </AutomationFormSection>
  );
}
