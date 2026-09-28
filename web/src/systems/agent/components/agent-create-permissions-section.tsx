import { Eye, Lock, ShieldCheck, Zap } from "lucide-react";
import type { ComponentType } from "react";

import { Alert, AlertDescription, Field, FormSection, RadioCard } from "@compozy/ui";

import {
  type AgentCreateDialogDraft,
  type AgentCreatePermissionChoice,
} from "../lib/agent-create-draft";
import { AGENT_CREATE_PERMISSION_OPTIONS } from "../lib/agent-permissions";

interface PermissionPresentation {
  icon: ComponentType<{ className?: string; size?: number }>;
  /** What the choice means for new sessions. */
  consequence: string;
}

const PERMISSION_PRESENTATION: Record<AgentCreatePermissionChoice, PermissionPresentation> = {
  "": {
    icon: ShieldCheck,
    consequence: "The agent's provider decides when to ask you.",
  },
  "deny-all": {
    icon: Lock,
    consequence: "New sessions wait for your OK before each action.",
  },
  "approve-reads": {
    icon: Eye,
    consequence: "New sessions read freely and ask before changing anything.",
  },
  "approve-all": {
    icon: Zap,
    consequence: "New sessions act without asking you.",
  },
};

export interface AgentCreatePermissionsSectionProps {
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/**
 * Advanced tier: the permission policy every session inherits.
 *
 * Four cards, not the reference's three: `permissions` is optional on
 * `CreateAgentPayload`, so "omit the field" is a distinct outcome an operator
 * can choose and must be able to see.
 */
export function AgentCreatePermissionsSection({
  draft,
  onDraftChange,
}: AgentCreatePermissionsSectionProps) {
  const selected = PERMISSION_PRESENTATION[draft.permissions];
  return (
    <FormSection
      data-testid="agent-create-permissions-section"
      help="Sessions launched from this agent inherit this policy. It is the ceiling on what the agent may do without asking you first."
      icon={ShieldCheck}
      title="What can it do on its own?"
    >
      <Field>
        <div
          aria-label="Permission policy"
          className="grid gap-2 sm:grid-cols-2"
          data-testid="agent-create-permissions"
          role="radiogroup"
        >
          {AGENT_CREATE_PERMISSION_OPTIONS.map(option => {
            const presentation = PERMISSION_PRESENTATION[option.value];
            return (
              <RadioCard
                data-testid={"agent-create-permissions-" + (option.value || "inherit")}
                description={option.description}
                icon={presentation.icon}
                key={option.value || "inherit"}
                onSelect={() => onDraftChange({ ...draft, permissions: option.value })}
                selected={draft.permissions === option.value}
                title={option.label}
                titleClassName="min-w-0 flex-1 truncate"
              />
            );
          })}
        </div>
        <Alert data-testid="agent-create-permissions-consequence" variant="neutral">
          <AlertDescription>{selected.consequence}</AlertDescription>
        </Alert>
      </Field>
    </FormSection>
  );
}
