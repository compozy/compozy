import { AlertTriangle, Trash2 } from "lucide-react";

import { Button, FormSection } from "@compozy/ui";

import type { AgentPayload } from "../types";

export interface AgentSettingsDangerSectionProps {
  agent: AgentPayload;
  onDelete: () => void;
  isDeleting: boolean;
  disabled?: boolean;
}

export function AgentSettingsDangerSection({
  agent,
  onDelete,
  isDeleting,
  disabled = false,
}: AgentSettingsDangerSectionProps) {
  const scopeLabel = agent.origin === "workspace" ? "this project" : "all projects";
  return (
    <FormSection
      data-testid="agent-settings-danger"
      icon={AlertTriangle}
      title="Delete agent"
      description={`Permanently remove ${agent.name} from ${scopeLabel}.`}
      className="border-danger/30"
    >
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
        <div className="min-w-0">
          <p className="text-small-body text-muted">Past sessions stay. This can't be undone.</p>
        </div>
        <Button
          type="button"
          variant="destructive"
          size="sm"
          onClick={onDelete}
          disabled={disabled || isDeleting}
          data-testid="agent-settings-delete"
        >
          <Trash2 aria-hidden="true" />
          {isDeleting ? "Deleting…" : "Delete agent"}
        </Button>
      </div>
    </FormSection>
  );
}
