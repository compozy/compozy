/** S1: the file's sources under "What it looks at", each with its row menu, and "+ Add source". */
import { Plus } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Eyebrow,
  TopbarOverflowIcon,
} from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import { canAddSource } from "../../lib/agent-check-in-source-draft";
import {
  checkInSourceParams,
  checkInSourceStatus,
  checkInSourceTitle,
  type CheckInSource,
} from "../../lib/agent-check-in-sources";
import { AddCheckInSourceDialog } from "./add-check-in-source-dialog";
import { CheckInSourceRow } from "./check-in-source-row";

interface RowControlsProps {
  model: CheckInsEditorModel;
  source: CheckInSource;
  title: string;
  status: string | undefined;
  editable: boolean;
  onEdit: () => void;
}

function RowControls({ model, source, title, status, editable, onEdit }: RowControlsProps) {
  const { sources: actions, catalog, readOnly } = model;
  const script = catalog?.scripts.find(item => item.name === source.script);
  const approve =
    !readOnly && status === "unapproved" && script && !script.approved ? (
      <Button
        aria-busy={actions.approving === script.name || undefined}
        disabled={actions.approving === script.name}
        onClick={() => actions.approve(script.name, script.sha256)}
        size="sm"
        type="button"
        variant="neutral"
      >
        Approve
      </Button>
    ) : null;
  const enable =
    status === "unavailable" && source.kind ? (
      <Button onClick={actions.openExtensions} size="sm" type="button" variant="link">
        Turn it on
      </Button>
    ) : null;
  return (
    <>
      {approve}
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
}: {
  model: CheckInsEditorModel;
  disabled: boolean;
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
  return (
    <div className="flex flex-col pt-3">
      {settings.sources.length > 0 ? (
        <>
          <Eyebrow className="text-subtle">Sources</Eyebrow>
          <div className="flex flex-col divide-y divide-line-soft">
            {settings.sources.map(source => {
              const script = catalog?.scripts.find(item => item.name === source.script);
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
        </>
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
    </div>
  );
}
