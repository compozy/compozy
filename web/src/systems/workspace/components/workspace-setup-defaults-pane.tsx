import { Plus } from "lucide-react";
import { useState } from "react";

import {
  Button,
  CommandSelectChip,
  CommandSelectChipStrip,
  Field,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
} from "@compozy/ui";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import type {
  WorkspaceSetupCollection,
  WorkspaceSetupDefaultsModel,
} from "../lib/workspace-setup-defaults";
import { AgentCommandSelect } from "@/systems/agent";

interface WorkspaceSetupDefaultsPaneProps {
  setup: WorkspaceSetupContent;
  defaults: WorkspaceSetupDefaultsModel;
}

function collectionPlaceholder<T>(
  collection: WorkspaceSetupCollection<T>,
  loading: string,
  empty: string,
  error: string,
  ready: string
): string {
  if (collection.state === "loading") return loading;
  if (collection.state === "error") return error;
  return collection.entries.length === 0 ? empty : ready;
}

function CollectionStateMessage<T>({
  collection,
  loading,
  error,
  testId,
}: {
  collection: WorkspaceSetupCollection<T>;
  loading: string;
  error: string;
  testId: string;
}) {
  if (collection.state === "loading") {
    return (
      <p className="text-form-hint text-muted" data-testid={`${testId}-loading`} role="status">
        {loading}
      </p>
    );
  }

  if (collection.state === "error") {
    return (
      <p className="text-form-hint text-danger" data-testid={`${testId}-error`} role="alert">
        {error}: {collection.message}
      </p>
    );
  }

  return null;
}

/**
 * Right pane of the split shell: the session defaults carried by
 * `CreateWorkspaceRequest`. Every field here is optional and editable later.
 */
export function WorkspaceSetupDefaultsPane({ setup, defaults }: WorkspaceSetupDefaultsPaneProps) {
  const [pendingDir, setPendingDir] = useState("");
  const disabled = setup.submissionMode !== null;
  const agents = defaults.agents.state === "ready" ? defaults.agents.entries : [];
  const agentsUnavailable = defaults.agents.state !== "ready";

  const commitDir = () => {
    setup.addDir(pendingDir);
    setPendingDir("");
  };

  return (
    <FormSection title="Defaults for new sessions">
      <div className="flex flex-col gap-4">
        <Field>
          <FieldLabel htmlFor="workspace-setup-default-agent">Default agent</FieldLabel>
          <AgentCommandSelect
            agents={agents}
            clearLabel="No default agent"
            disabled={disabled || agentsUnavailable}
            onChange={next => setup.setDefaultAgent(next ?? "")}
            placeholder={collectionPlaceholder(
              defaults.agents,
              "Loading agents…",
              "No agents available",
              "Agents unavailable",
              "No default agent"
            )}
            triggerId="workspace-setup-default-agent"
            triggerTestId="workspace-setup-default-agent-select"
            value={setup.draft.defaultAgent || null}
          />
          <CollectionStateMessage
            collection={defaults.agents}
            error="Could not load agents"
            loading="Loading agents…"
            testId="workspace-setup-default-agent"
          />
        </Field>

        <Field>
          <FieldHeader>
            <FieldLabel htmlFor="workspace-setup-add-dir">Other folders agents can read</FieldLabel>
            <HelpTip label="About other folders">
              Sessions in this project can also read these folders.
            </HelpTip>
          </FieldHeader>
          <div className="flex items-center gap-2">
            <Input
              className="font-mono"
              data-testid="workspace-setup-add-dir-input"
              disabled={disabled}
              id="workspace-setup-add-dir"
              onChange={event => setPendingDir(event.target.value)}
              onKeyDown={event => {
                if (event.key !== "Enter") return;
                // The pane lives inside the dialog form; Enter adds a chip here
                // rather than submitting the workspace.
                event.preventDefault();
                commitDir();
              }}
              placeholder="/Users/you/Dev/shared-libs"
              value={pendingDir}
            />
            <Button
              data-testid="workspace-setup-add-dir-submit"
              disabled={disabled || pendingDir.trim() === ""}
              onClick={commitDir}
              size="sm"
              type="button"
              variant="outline"
            >
              <Plus className="size-3.5" />
              Add
            </Button>
          </div>
          {setup.draft.addDirs.length > 0 ? (
            <CommandSelectChipStrip className="mt-2" data-testid="workspace-setup-add-dir-list">
              {setup.draft.addDirs.map(dir => (
                <CommandSelectChip
                  aria-label={`Remove ${dir}`}
                  disabled={disabled}
                  key={dir}
                  onRemove={() => setup.removeDir(dir)}
                >
                  {dir}
                </CommandSelectChip>
              ))}
            </CommandSelectChipStrip>
          ) : null}
        </Field>
      </div>
    </FormSection>
  );
}
