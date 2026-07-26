import { Eye, Lock, ShieldCheck, Zap } from "lucide-react";
import type { ComponentType } from "react";

import { Alert, AlertDescription, Field, FormSection, Pill, RadioCard } from "@agh/ui";

import {
  AGENT_CREATE_PERMISSION_OPTIONS,
  type AgentCreateDialogDraft,
  type AgentCreatePermissionChoice,
} from "../lib/agent-create-draft";

interface PermissionPresentation {
  title: string;
  description: string;
  badge: string;
  icon: ComponentType<{ className?: string; size?: number }>;
  /** What the create request carries for this choice. */
  consequence: string;
}

const PERMISSION_PRESENTATION: Record<AgentCreatePermissionChoice, PermissionPresentation> = {
  "": {
    title: "Follow the runtime",
    description: "AGH sends no permission policy; the runtime decides the approval posture.",
    badge: "Runtime default",
    icon: ShieldCheck,
    consequence: "The definition omits permissions — the runtime's own default applies.",
  },
  "deny-all": {
    title: "Ask for everything",
    description: "Every tool call waits for your approval first.",
    badge: "Safest",
    icon: Lock,
    consequence: "Sessions inherit deny-all.",
  },
  "approve-reads": {
    title: "Reads are free",
    description: "Reading is automatic; writes and commands still ask.",
    badge: "Balanced",
    icon: Eye,
    consequence: "Sessions inherit approve-reads.",
  },
  "approve-all": {
    title: "Run everything",
    description: "No approval gates. For trusted, well-scoped agents.",
    badge: "Trusted",
    icon: Zap,
    consequence: "Sessions inherit approve-all.",
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
                // The posture badge stacks under the description rather than beside
                // the title: an inline badge pushes the title off its own row.
                description={
                  <>
                    {presentation.description}
                    <Pill className="mt-1.5 flex w-fit" size="xs">
                      {presentation.badge}
                    </Pill>
                  </>
                }
                icon={presentation.icon}
                key={option.value || "inherit"}
                onSelect={() => onDraftChange({ ...draft, permissions: option.value })}
                selected={draft.permissions === option.value}
                title={presentation.title}
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
