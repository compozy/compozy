import { Boxes, Pencil, Trash2 } from "lucide-react";

import { Button, CatalogCard, Pill } from "@agh/ui";

import type { SettingsSandboxEntry } from "@/systems/settings";
import {
  sandboxBackendLabel,
  sandboxBackendTone,
  sandboxIsDeletable,
  sandboxOrDash,
  sandboxUsageLabel,
} from "../lib/sandbox-labels";

export interface SandboxProfileCardProps {
  entry: SettingsSandboxEntry;
  selected?: boolean;
  onSelect?: (entry: SettingsSandboxEntry) => void;
  onEdit?: (entry: SettingsSandboxEntry) => void;
  onDelete?: (entry: SettingsSandboxEntry) => void;
}

export function SandboxProfileCard({
  entry,
  selected = false,
  onSelect,
  onEdit,
  onDelete,
}: SandboxProfileCardProps) {
  const selectable = onSelect !== undefined;
  const deletable = sandboxIsDeletable(entry);
  const profile = entry.profile;

  return (
    <CatalogCard
      actionable={selectable}
      data-name={entry.name}
      data-testid={`sandbox-page-card-${entry.name}`}
      selected={selected}
    >
      <button
        aria-label={`Inspect ${entry.name}`}
        className="flex min-w-0 flex-col gap-3 text-left"
        data-testid={`sandbox-page-select-${entry.name}`}
        disabled={!selectable}
        onClick={() => onSelect?.(entry)}
        type="button"
      >
        <div className="flex items-start gap-2.5">
          <CatalogCard.Logo>
            <Boxes aria-hidden="true" className="size-3.5" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title className="font-mono text-xs font-medium">
              {entry.name}
            </CatalogCard.Title>
            <span className="text-micro text-subtle">
              {profile.backend}
              {" · "}
              <span className="font-mono">{sandboxOrDash(profile.persistence)}</span>
              {" · "}
              {sandboxUsageLabel(entry.workspace_usage_count)}
            </span>
          </div>
        </div>
        <p className="text-small-body leading-snug text-muted">
          {sandboxBackendLabel(profile.backend)}
        </p>
      </button>
      <CatalogCard.Actions className="justify-between">
        <Pill mono size="sm" tone={sandboxBackendTone(profile.backend)}>
          {profile.backend}
        </Pill>
        <div className="flex items-center gap-1">
          {onEdit ? (
            <Button
              aria-label={`Edit ${entry.name}`}
              data-testid={`sandbox-page-card-${entry.name}-edit`}
              onClick={event => {
                event.stopPropagation();
                onEdit(entry);
              }}
              size="sm"
              type="button"
              variant="ghost"
            >
              <Pencil aria-hidden="true" className="size-3" />
              Edit
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
                deletable
                  ? undefined
                  : "Builtin sandboxes cannot be deleted — override them instead."
              }
              type="button"
              variant="ghost"
            >
              <Trash2 aria-hidden="true" className="size-3" />
            </Button>
          ) : null}
        </div>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
