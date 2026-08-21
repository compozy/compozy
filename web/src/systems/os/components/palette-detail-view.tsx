import { Button, Markdown, MetadataList, SkeletonRows } from "@compozy/ui";

import type { CmdPaletteViewAction, CmdPaletteViewDetail } from "../lib/cmd-palette-types";

export function PaletteDetailView({
  detail,
  onAction,
}: {
  detail: CmdPaletteViewDetail | null;
  onAction?: (action: CmdPaletteViewAction) => void;
}) {
  if (detail === null) {
    return (
      <aside
        className="w-64 shrink-0 p-4 text-small-body text-muted"
        data-testid="palette-detail-empty"
      >
        Select an item to inspect it.
      </aside>
    );
  }
  if (detail.is_loading) {
    return (
      <aside className="w-64 shrink-0 p-4" data-testid="palette-detail-loading">
        <SkeletonRows count={4} />
      </aside>
    );
  }
  return (
    <aside
      className="w-64 shrink-0 overflow-y-auto p-4"
      data-testid="palette-detail-pane"
      aria-label="Item details"
    >
      {detail.markdown ? <Markdown compact>{detail.markdown}</Markdown> : null}
      {detail.metadata && detail.metadata.length > 0 ? (
        <MetadataList className="mt-4">
          {detail.metadata.map(field => (
            <MetadataList.Row key={field.label} label={field.label}>
              {field.value}
            </MetadataList.Row>
          ))}
        </MetadataList>
      ) : null}
      {detail.actions && detail.actions.length > 0 ? (
        <div className="mt-4 flex flex-wrap gap-2">
          {detail.actions.map(action => (
            <Button
              key={rowActionKey(action)}
              size="sm"
              variant={action.destructive ? "destructive" : "outline"}
              onClick={() => onAction?.(action)}
            >
              {action.title}
            </Button>
          ))}
        </div>
      ) : null}
    </aside>
  );
}

function rowActionKey(action: CmdPaletteViewAction): string {
  const target = action.action;
  return [
    action.title,
    action.handler ?? "",
    action.submit_form ? "submit" : "",
    target?.kind ?? "",
    target?.tool ?? target?.op ?? target?.view ?? target?.app ?? target?.url ?? "",
  ].join(":");
}
