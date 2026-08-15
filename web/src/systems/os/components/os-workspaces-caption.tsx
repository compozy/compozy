import { cn } from "@compozy/ui";

export interface OsWorkspacesCaptionModel {
  /** Crossfade key — changes on every focus move. */
  key: string;
  title: string;
  /** One optional plain-language clause (add tile / Global identity only). */
  meta?: string;
  /** `root_dir` for workspaces; picker hint for the add tile; `~` for Global. */
  path?: string;
}

export interface OsWorkspacesCaptionProps {
  model: OsWorkspacesCaptionModel;
  reducedMotion: boolean;
}

/**
 * Identity of the focused tile, written once under the strip: name + path.
 * No agent/session counts live here (locked caption contract). The reserved
 * min-height keeps the strip still while captions change length.
 */
export function OsWorkspacesCaption({ model, reducedMotion }: OsWorkspacesCaptionProps) {
  return (
    <div
      data-slot="os-workspaces-caption"
      data-testid="os-workspaces-caption"
      className="mt-4 flex min-h-12 flex-col items-center text-center min-[960px]:min-h-13"
    >
      <div
        key={model.key}
        className={cn("flex flex-col items-center gap-[3px]", !reducedMotion && "os-wsov-cap-swap")}
      >
        <b className="text-item-title font-semibold tracking-tight text-fg-strong">{model.title}</b>
        {model.meta ? <span className="text-form-label text-muted">{model.meta}</span> : null}
        {model.path ? (
          <span className="font-mono text-mono-id text-faint tabular-nums">{model.path}</span>
        ) : null}
      </div>
    </div>
  );
}
