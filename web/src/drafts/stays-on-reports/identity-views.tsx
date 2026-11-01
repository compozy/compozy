/** S12, S13 and S14, shown inside the hosts they change. */
import { Trash2 } from "@compozy/ui/icons";

import { Marker, cn } from "@compozy/ui";

import { ToastFrame } from "@/systems/os/components/attention-toast";

import {
  TRIGGER_MENU_ITEMS_CLASS,
  TRIGGER_MENU_POPOVER_CLASS,
} from "@/components/assistant-ui/session-composer-trigger-menu-classes";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import {
  CODE_ENGINEER,
  GENERAL,
  ORCHESTRATOR,
  type DraftAgent,
} from "../stays-on-agents-shared/fixtures";
import { DraftComposer } from "../stays-on-agents-shared/transcript";

function MenuRow({ agent, active = false }: { agent: DraftAgent; active?: boolean }) {
  return (
    <div className={cn("flex items-center gap-2.5 rounded-md px-2 py-1.5", active && "bg-hover")}>
      <AgentAvatar
        appearance={agent.appearance}
        label={agent.label}
        provider={agent.provider}
        quietIdle
        size={24}
        surfaceClassName="ring-popover"
      />
      <span className="text-small-body text-fg">{agent.label}</span>
      <span className="truncate text-eyebrow text-subtle">{agent.description}</span>
    </div>
  );
}

export function MentionComposer() {
  return (
    <div className="relative mx-auto w-full max-w-3xl">
      <div className={cn(TRIGGER_MENU_POPOVER_CLASS, "bottom-[calc(100%-0.5rem)] mx-6")}>
        <div className={TRIGGER_MENU_ITEMS_CLASS}>
          <p className="px-2 py-1 text-eyebrow text-subtle">Agents</p>
          <MenuRow active agent={ORCHESTRATOR} />
          <MenuRow agent={CODE_ENGINEER} />
          <MenuRow agent={GENERAL} />
        </div>
      </div>
      <DraftComposer placeholder="">Ask @</DraftComposer>
    </div>
  );
}

function AgentChip({ agent, unresolved = false }: { agent: DraftAgent; unresolved?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-baseline gap-1 align-baseline",
        unresolved ? "text-subtle line-through" : "text-fg"
      )}
    >
      <AgentAvatar
        appearance={agent.appearance}
        className="translate-y-0.5"
        label={agent.label}
        quietIdle
        size={14}
        state={unresolved ? "unknown" : "idle"}
      />
      {agent.label}
    </span>
  );
}

export function UnresolvedComposer() {
  return (
    <DraftComposer placeholder="">
      Ask <AgentChip agent={ORCHESTRATOR} /> and{" "}
      <AgentChip
        agent={{
          ...CODE_ENGINEER,
          label: "Release Captain",
          appearance: { avatar: "generated", seed: "release-captain" },
        }}
        unresolved
      />{" "}
      to compare notes.
    </DraftComposer>
  );
}

export function DeletedAgentMarker() {
  return (
    <Marker icon={<Trash2 aria-hidden="true" className="size-3" />}>
      <b>Release Captain</b> was deleted. Its sessions stay in history.
    </Marker>
  );
}

/** The real toast reading only the `agent_name` an old payload carries. */
export function OldNotification() {
  return (
    <div className="absolute top-14 right-4 z-30 w-80">
      <ToastFrame
        body="Migrate tokens is waiting for an answer."
        mark={
          <AgentAvatar
            appearance={{ avatar: "generated", seed: "code_implementer" }}
            label="Code Implementer"
            quietIdle
            size={32}
            state="unknown"
            surfaceClassName="ring-popover"
          />
        }
        meta="compozy"
        testId="os-attention-toast-needs-you"
        title="Code Implementer needs you"
      />
    </div>
  );
}
