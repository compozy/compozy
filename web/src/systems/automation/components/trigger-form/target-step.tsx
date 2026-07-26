import { PillGroup } from "@agh/ui";
import type { AgentPayload } from "@/systems/agent";
import { LoopTargetFields, type LoopTargetCatalog, type LoopTargetDraft } from "@/systems/loops";

import type { AutomationTargetMode } from "../../lib/automation-drafts";
import { AgentPromptStep } from "./agent-prompt-step";

interface TriggerTargetStepProps {
  mode: AutomationTargetMode;
  onModeChange: (mode: AutomationTargetMode) => void;
  /** Agent-mode fields. */
  agent: string;
  agents: AgentPayload[];
  agentsLoading?: boolean;
  agentsError?: string | null;
  prompt: string;
  variables: string[];
  onAgentChange: (next: string) => void;
  onPromptChange: (next: string) => void;
  /** Loop-mode fields. */
  catalog: LoopTargetCatalog;
  editorMode: "create" | "edit";
  loopTarget: LoopTargetDraft;
  onLoopTargetChange: (next: LoopTargetDraft) => void;
}

const MODE_ITEMS = [
  { value: "agent" as const, label: "Run agent", testId: "target-mode-agent" },
  { value: "loop" as const, label: "Run loop", testId: "target-mode-loop" },
];

/**
 * The trigger "Then" target step (§9.14): choose between running an agent (today's
 * agent + prompt) or running a Loop (loop picker + typed inputs + an event-payload
 * mapping table, since triggers fire from an event envelope).
 */
export function TriggerTargetStep({
  mode,
  onModeChange,
  agent,
  agents,
  agentsLoading = false,
  agentsError = null,
  prompt,
  variables,
  onAgentChange,
  onPromptChange,
  catalog,
  editorMode,
  loopTarget,
  onLoopTargetChange,
}: TriggerTargetStepProps) {
  return (
    <div className="space-y-4" data-testid="trigger-target-step">
      <PillGroup
        aria-label="Target"
        items={MODE_ITEMS.map(item => ({ ...item, disabled: editorMode === "edit" }))}
        value={mode}
        onChange={onModeChange}
        size="sm"
      />
      {mode === "loop" ? (
        <LoopTargetFields
          catalog={catalog}
          identityDisabled={editorMode === "edit"}
          mode={editorMode}
          value={loopTarget}
          onChange={onLoopTargetChange}
          showMapping
        />
      ) : (
        <AgentPromptStep
          agent={agent}
          agentDisabled={editorMode === "edit"}
          agents={agents}
          agentsError={agentsError}
          agentsLoading={agentsLoading}
          onAgentChange={onAgentChange}
          onPromptChange={onPromptChange}
          prompt={prompt}
          variables={variables}
        />
      )}
    </div>
  );
}
