import { Boxes, Pencil, Trash2 } from "lucide-react";

import { Button, ListingRow, Pill } from "@agh/ui";

import { SettingsSourceBadge, type SettingsSandboxEntry } from "@/systems/settings";
import {
  sandboxBackendLabel,
  sandboxBackendTone,
  sandboxIsDeletable,
  sandboxOrDash,
} from "../lib/sandbox-labels";

export interface SandboxProfileRowProps {
  entry: SettingsSandboxEntry;
  selected?: boolean;
  onSelect?: (entry: SettingsSandboxEntry) => void;
  onEdit?: (entry: SettingsSandboxEntry) => void;
  onDelete?: (entry: SettingsSandboxEntry) => void;
}

export function SandboxProfileRow({
  entry,
  selected = false,
  onSelect,
  onEdit,
  onDelete,
}: SandboxProfileRowProps) {
  const selectable = onSelect !== undefined;
  const deletable = sandboxIsDeletable(entry);
  const usage = entry.workspace_usage_count;

  return (
    <ListingRow
      data-name={entry.name}
      data-testid={`sandbox-page-card-${entry.name}`}
      interactive={selectable}
      selected={selected}
    >
      {selectable ? (
        <ListingRow.Link
          render={
            <button
              aria-label={`Inspect ${entry.name}`}
              data-testid={`sandbox-page-select-${entry.name}`}
              onClick={() => onSelect(entry)}
              type="button"
            />
          }
        >
          <SandboxProfileRowBody entry={entry} />
        </ListingRow.Link>
      ) : (
        <SandboxProfileRowBody entry={entry} />
      )}
      <ListingRow.Trail>
        <ListingRow.Stat data-testid={`sandbox-page-card-${entry.name}-usage`}>
          <ListingRow.Stat.Value>{usage}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{usage === 1 ? "workspace" : "workspaces"}</ListingRow.Stat.Label>
        </ListingRow.Stat>
        {onEdit ? (
          <Button
            aria-label={`Edit ${entry.name}`}
            data-testid={`sandbox-page-card-${entry.name}-edit`}
            onClick={event => {
              event.stopPropagation();
              onEdit(entry);
            }}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <Pencil aria-hidden="true" className="size-3" />
          </Button>
        ) : null}
        {onDelete ? (
          <Button
            aria-label={`Delete ${entry.name}`}
            data-testid={`sandbox-page-card-${entry.name}-delete`}
            disabled={!deletable}
            onClick={event => {
              event.stopPropagation();
              onDelete(entry);
            }}
            size="icon-sm"
            title={
              deletable ? undefined : "Builtin sandboxes cannot be deleted — override them instead."
            }
            type="button"
            variant="ghost"
          >
            <Trash2 aria-hidden="true" className="size-3" />
          </Button>
        ) : null}
      </ListingRow.Trail>
    </ListingRow>
  );
}

function SandboxProfileRowBody({ entry }: { entry: SettingsSandboxEntry }) {
  const profile = entry.profile;
  return (
    <>
      <ListingRow.Icon>
        <Boxes aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title>{entry.name}</ListingRow.Title>
          <Pill mono size="sm" tone={sandboxBackendTone(profile.backend)}>
            {profile.backend}
          </Pill>
        </ListingRow.Name>
        <ListingRow.Description>{sandboxBackendLabel(profile.backend)}</ListingRow.Description>
        <ListingRow.Meta data-testid={`sandbox-page-card-${entry.name}-profile`}>
          <span className="font-mono text-mono-id text-subtle">
            sync {sandboxOrDash(profile.sync_mode)}
          </span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span className="font-mono text-mono-id text-subtle">
            {sandboxOrDash(profile.persistence)}
          </span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span className="font-mono text-mono-id text-subtle">
            {sandboxOrDash(profile.runtime_root)}
          </span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <SettingsSourceBadge
            data-testid={`sandbox-page-card-${entry.name}-source`}
            shadowed={entry.source_metadata.shadowed_sources ?? []}
            source={entry.source_metadata.effective_source}
          />
        </ListingRow.Meta>
      </ListingRow.Main>
    </>
  );
}
