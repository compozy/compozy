/** S1: the file's sources, each with its row menu, the approvals they wait for, and "+ Add source". */
import { Plus } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import { canAddSource } from "../../lib/agent-check-in-source-draft";
import {
  checkInScriptsAwaitingApproval,
  checkInSourceParams,
  checkInSourceStatus,
  checkInSourceTitle,
  type CheckInSource,
} from "../../lib/agent-check-in-sources";
import { AddCheckInSourceDialog } from "./add-check-in-source-dialog";
import { CheckInScriptApprovals } from "./check-in-script-approval";
import { CheckInSourceRow } from "./check-in-source-row";
import { SettingRow } from "./rich-controls";

interface RowControlsProps {
  model: CheckInsEditorModel;
  source: CheckInSource;
  title: string;
  status: string | undefined;
  editable: boolean;
  onEdit: () => void;
}

function RowControls({ model, source, title, status, editable, onEdit }: RowControlsProps) {
  const { sources: actions } = model;
  const enable =
    status === "unavailable" && source.kind ? (
      <Button onClick={actions.openExtensions} size="sm" type="button" variant="link">
        Turn it on
      </Button>
    ) : null;
  return (
    <>
      {enable}
      {editable ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`More for ${title}`}
            render={<Button size="icon-sm" type="button" variant="quiet" />}
          >
            <TopbarOverflowIcon aria-hidden="true" className="size-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={onEdit}>Edit</DropdownMenuItem>
            <DropdownMenuItem onClick={() => actions.remove(source.id)} variant="destructive">
              Remove
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </>
  );
}

/** The file's sources in order, each with how its last read went in the project on the page. */
export function CheckInSources({
  model,
  disabled,
  agentLabel,
}: {
  model: CheckInsEditorModel;
  disabled: boolean;
  agentLabel: string;
}) {
  const { settings, instance, catalog, readOnly } = model;
  // `null` closed, `""` adding, otherwise the id of the source being edited.
  const [editing, setEditing] = useState<string | null>(null);
  const now = model.now.getTime();
  const states = instance?.sources ?? [];
  const editable = !disabled && !readOnly;
  const addable = editable && canAddSource(settings.sources.length, catalog);
  const editingSource = settings.sources.find(source => source.id === editing);
  if (settings.sources.length === 0 && !addable) return null;
  const waiting = readOnly ? 0 : checkInScriptsAwaitingApproval(settings.sources, catalog).length;
  return (
    <SettingRow
      title="Sources"
      value={
        waiting > 0 ? `${waiting} ${waiting === 1 ? "needs" : "need"} your approval` : undefined
      }
    >
      <CheckInScriptApprovals agentLabel={agentLabel} model={model} />
      {settings.sources.length > 0 ? (
        <div className="flex flex-col divide-y divide-line-soft">
          {settings.sources.map(source => {
            // Scripts that are off have nothing to approve or run; the poll status says so.
            const script = catalog?.scripts_enabled
              ? catalog.scripts.find(item => item.name === source.script)
              : undefined;
            const state = states.find(item => item.id === source.id);
            const title = checkInSourceTitle(source, catalog);
            return (
              <CheckInSourceRow
                actions={
                  <RowControls
                    editable={editable}
                    model={model}
                    onEdit={() => setEditing(source.id)}
                    source={source}
                    status={state?.status}
                    title={title}
                  />
                }
                key={source.id}
                params={checkInSourceParams(source)}
                script={Boolean(source.script)}
                status={checkInSourceStatus(source, state, now, script)}
                title={title}
              />
            );
          })}
        </div>
      ) : null}
      {addable ? (
        <div className="pt-2">
          <Button onClick={() => setEditing("")} size="sm" type="button" variant="quiet">
            <Plus aria-hidden="true" data-icon="inline-start" />
            Add source
          </Button>
        </div>
      ) : null}
      {editing === null ? null : (
        <AddCheckInSourceDialog
          actions={model.sources}
          catalog={catalog}
          key={editing}
          onClose={() => setEditing(null)}
          others={settings.sources.flatMap(source => (source.id === editing ? [] : [source.id]))}
          saving={model.document.saving}
          source={editingSource}
        />
      )}
    </SettingRow>
  );
}
