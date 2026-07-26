import { AlertTriangle, Trash2 } from "lucide-react";

import { Button, FormSection } from "@agh/ui";

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
  const scopeLabel = agent.origin === "workspace" ? "this workspace" : "the global agent home";
  return (
    <FormSection
      data-testid="agent-settings-danger"
      icon={AlertTriangle}
      title="Danger zone"
      description={`Permanently remove ${agent.name} from ${scopeLabel}.`}
      className="border-danger/30"
    >
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
        <div className="min-w-0">
          <p className="text-small-body font-medium text-danger">Delete agent</p>
          <p className="text-small-body text-muted">
            Existing session records are kept. This cannot be undone from the UI.
          </p>
        </div>
        <Button
          type="button"
          variant="destructive"
          size="sm"
          onClick={onDelete}
          disabled={disabled || isDeleting}
          data-testid="agent-settings-delete"
        >
          <Trash2 className="size-3" />
          {isDeleting ? "Deleting…" : "Delete agent"}
        </Button>
      </div>
    </FormSection>
  );
}
