import { Plus, X } from "lucide-react";
import { useState } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  FormSection,
  Input,
  Pill,
} from "@agh/ui";

import { AgentCommandSelect } from "@/systems/agent";

import type { WorkspaceSetupContent } from "../hooks/use-workspace-setup-content";
import type {
  WorkspaceSetupCollection,
  WorkspaceSetupDefaultsModel,
} from "../lib/workspace-setup-defaults";

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
  const [sandboxOpen, setSandboxOpen] = useState(false);
  const disabled = setup.submissionMode !== null;
  const agents = defaults.agents.state === "ready" ? defaults.agents.entries : [];
  const sandboxes = defaults.sandboxes.state === "ready" ? defaults.sandboxes.entries : [];
  const agentsUnavailable = defaults.agents.state !== "ready";
  const sandboxesUnavailable = defaults.sandboxes.state !== "ready";

  const commitDir = () => {
    setup.addDir(pendingDir);
    setPendingDir("");
  };

  return (
    <FormSection
      description="Optional — every one of these can change later."
      title="Session defaults"
    >
      <div className="flex flex-col gap-4">
        <Field>
          <FieldContent>
            <FieldLabel htmlFor="workspace-setup-default-agent">Default agent</FieldLabel>
            <FieldDescription>Preselected when a session starts here.</FieldDescription>
          </FieldContent>
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
          <FieldContent>
            <FieldTitle id="workspace-setup-sandbox-label">Sandbox profile</FieldTitle>
            <FieldDescription>Isolation applied to sessions in this workspace.</FieldDescription>
          </FieldContent>
          <CommandSelect onOpenChange={setSandboxOpen} open={sandboxOpen}>
            <CommandSelectTrigger
              aria-labelledby="workspace-setup-sandbox-label"
              data-testid="workspace-setup-sandbox-select"
              disabled={disabled || sandboxesUnavailable}
              placeholder={collectionPlaceholder(
                defaults.sandboxes,
                "Loading sandbox profiles…",
                "No sandbox profiles",
                "Sandbox profiles unavailable",
                "No sandbox"
              )}
              selected={setup.draft.sandboxRef !== ""}
            >
              {setup.draft.sandboxRef || null}
            </CommandSelectTrigger>
            <CommandSelectShell inputPlaceholder="Search sandbox profiles…">
              <CommandList>
                <CommandEmpty>No sandbox profiles match.</CommandEmpty>
                <CommandSelectGroup>
                  <CommandItem
                    data-checked={setup.draft.sandboxRef === "" ? "true" : "false"}
                    data-testid="workspace-setup-sandbox-none"
                    onSelect={() => {
                      setup.setSandboxRef("");
                      setSandboxOpen(false);
                    }}
                    value="No sandbox"
                  >
                    <span className="min-w-0 flex-1 truncate text-muted">No sandbox</span>
                  </CommandItem>
                  {sandboxes.map(sandbox => (
                    <CommandItem
                      data-testid={`workspace-setup-sandbox-${sandbox.name}`}
                      key={sandbox.name}
                      onSelect={() => {
                        setup.setSandboxRef(sandbox.name);
                        setSandboxOpen(false);
                      }}
                      value={sandbox.name}
                    >
                      <span className="min-w-0 flex-1 truncate">{sandbox.name}</span>
                      {sandbox.backend ? (
                        <Pill mono size="xs">
                          {sandbox.backend}
                        </Pill>
                      ) : null}
                    </CommandItem>
                  ))}
                </CommandSelectGroup>
              </CommandList>
            </CommandSelectShell>
          </CommandSelect>
          <CollectionStateMessage
            collection={defaults.sandboxes}
            error="Could not load sandbox profiles"
            loading="Loading sandbox profiles…"
            testId="workspace-setup-sandbox"
          />
        </Field>

        <Field>
          <FieldContent>
            <FieldLabel htmlFor="workspace-setup-add-dir">Additional directories</FieldLabel>
            <FieldDescription>Extra roots sessions may read.</FieldDescription>
          </FieldContent>
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
            <div className="mt-2 flex flex-wrap gap-1.5" data-testid="workspace-setup-add-dir-list">
              {setup.draft.addDirs.map(dir => (
                <span
                  className="inline-flex items-center gap-1 rounded-sm bg-canvas px-2 py-1 font-mono text-mono-id tracking-normal text-muted"
                  key={dir}
                >
                  <span className="min-w-0 truncate">{dir}</span>
                  <Button
                    aria-label={`Remove ${dir}`}
                    disabled={disabled}
                    onClick={() => setup.removeDir(dir)}
                    size="icon-xs"
                    type="button"
                    variant="ghost"
                  >
                    <X className="size-3" />
                  </Button>
                </span>
              ))}
            </div>
          ) : null}
        </Field>
      </div>
    </FormSection>
  );
}
