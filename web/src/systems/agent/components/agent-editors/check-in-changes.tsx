/** S6: what a check-in may change on its own, and the actions a proposal can run as. */
import { Plus, Trash2, Wrench, Zap } from "@compozy/ui/icons";

import { Button, Field, FieldLabel, Input, Textarea } from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import {
  actionIdFor,
  checkInActionsProblem,
  type CheckInAction,
  type CheckInAuthorityForm,
} from "../../lib/agent-check-in-authority";
import { SettingRow, ToggleRow } from "./rich-controls";

/** Writes the authority part of the form back into the document. */
function patchAuthority(model: CheckInsEditorModel) {
  const { document } = model;
  return (next: Partial<CheckInAuthorityForm>) =>
    document.setForm({
      ...document.form,
      authority: { ...document.form.authority, ...next },
    });
}

export function CheckInWrites({
  model,
  disabled,
}: {
  model: CheckInsEditorModel;
  disabled: boolean;
}) {
  const patch = patchAuthority(model);
  const { writes } = model.settings.authority;
  const { writeTools, toolsState, writeWarnings } = model.authority;
  const known = new Set(writeTools.map(tool => tool.tool_id));
  const chosen = new Set(writes);
  const others: { id: string; label: string }[] = [];
  for (const id of writes) if (!known.has(id)) others.push({ id, label: id });
  // Selectors the list can't show, such as `mcp__server__*`, stay listed so a save keeps them.
  const rows = [
    ...writeTools.map(tool => ({
      id: tool.tool_id,
      label: tool.server ? `${tool.title} · ${tool.server}` : tool.title,
    })),
    ...others,
  ];
  const toggle = (id: string, on: boolean) =>
    patch({ writes: on ? [...writes, id] : writes.filter(item => item !== id) });
  return (
    <SettingRow
      title="What it can change"
      value={writes.length === 0 ? "Nothing, reads only" : undefined}
    >
      <p className="text-small-body text-muted-foreground">
        Check-ins run unattended. Only these tools can change things, and only if the agent could
        run them without asking.
      </p>
      {rows.length === 0 ? (
        <p className="text-small-body text-subtle">
          {toolsState === "loading"
            ? "Loading its tools…"
            : toolsState === "error"
              ? "Couldn't load its tools."
              : "It has no tools that change things."}
        </p>
      ) : (
        <div className="flex flex-col divide-y divide-line-soft">
          {rows.map(row => (
            <ToggleRow
              checked={chosen.has(row.id)}
              disabled={disabled}
              icon={Wrench}
              key={row.id}
              label={row.label}
              onChange={on => toggle(row.id, on)}
            />
          ))}
        </div>
      )}
      {writeWarnings.map(warning => (
        <p className="text-small-body text-warning" key={warning} role="status">
          {warning}
        </p>
      ))}
    </SettingRow>
  );
}

function ActionEditor({
  action,
  disabled,
  onChange,
  onRemove,
}: {
  action: CheckInAction;
  disabled: boolean;
  onChange: (next: CheckInAction) => void;
  onRemove: () => void;
}) {
  return (
    <div
      aria-label={action.title || "New action"}
      className="flex flex-col gap-3 rounded-lg bg-sunken p-4"
      role="group"
    >
      <Field>
        <FieldLabel htmlFor={`action-title-${action.id}`}>Title</FieldLabel>
        <Input
          disabled={disabled}
          id={`action-title-${action.id}`}
          onChange={event => onChange({ ...action, title: event.target.value })}
          placeholder="Triage new issues"
          value={action.title}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`action-instructions-${action.id}`}>Instructions</FieldLabel>
        <Textarea
          disabled={disabled}
          id={`action-instructions-${action.id}`}
          onChange={event => onChange({ ...action, instructions: event.target.value })}
          placeholder="Label each new issue and say which ones need me."
          value={action.instructions}
        />
      </Field>
      <div className="flex flex-col gap-0.5">
        <ToggleRow
          checked={action.autoAccept}
          disabled={disabled}
          icon={Zap}
          label="Do it without asking me"
          onChange={autoAccept => onChange({ ...action, autoAccept })}
        />
        <p className="text-small-body text-subtle">
          Runs in the agent's main session with its own permissions.
        </p>
      </div>
      <Button
        className="self-start"
        disabled={disabled}
        onClick={onRemove}
        size="sm"
        type="button"
        variant="ghost"
      >
        <Trash2 aria-hidden="true" data-icon="inline-start" />
        Remove action
      </Button>
    </div>
  );
}

export function CheckInActions({
  model,
  disabled,
}: {
  model: CheckInsEditorModel;
  disabled: boolean;
}) {
  const patch = patchAuthority(model);
  const { actions } = model.settings.authority;
  const ids = actions.map(action => action.id);
  const problem = checkInActionsProblem(actions);
  return (
    <SettingRow title="Actions">
      <p className="text-small-body text-muted-foreground">
        What a check-in can propose doing. You pick it, unless it runs without asking.
      </p>
      {actions.map((action, index) => (
        <ActionEditor
          action={action}
          disabled={disabled}
          key={action.id}
          onChange={next =>
            patch({ actions: actions.map((item, at) => (at === index ? next : item)) })
          }
          onRemove={() => patch({ actions: actions.filter((_, at) => at !== index) })}
        />
      ))}
      {problem && actions.length > 0 ? (
        <p className="text-small-body text-warning" role="status">
          {problem}
        </p>
      ) : null}
      <Button
        className="self-start"
        disabled={disabled}
        onClick={() =>
          patch({
            actions: [
              ...actions,
              { id: actionIdFor("action", ids), title: "", instructions: "", autoAccept: false },
            ],
          })
        }
        size="sm"
        type="button"
        variant="secondary"
      >
        <Plus aria-hidden="true" data-icon="inline-start" />
        Add action
      </Button>
    </SettingRow>
  );
}
