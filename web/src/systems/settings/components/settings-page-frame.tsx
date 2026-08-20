import type { ReactNode } from "react";

import { cn } from "@compozy/ui";

import type { SettingsRestartViewState } from "../lib/restart-presentation";
import type { SettingsSectionSlug } from "../types";
import { SettingsRestartNotice } from "./settings-restart-notice";

export interface SettingsPageFrameProps {
  slug: SettingsSectionSlug;
  /**
   * Visible consequence line only — inherited defaults, irreversible apply,
   * or what this save will do. Restating the page title belongs in `help`.
   */
  description?: ReactNode;
  /** Quiet dotsep meta entries after the description (live counts, freshness). */
  meta?: ReadonlyArray<{ key: string; content: ReactNode }>;
  restart?: SettingsRestartViewState;
  /**
   * Column width: forms at 768px, listings (Providers) at 960px, and the layout
   * canvas at 1040px, where a stage plus its inspector needs the extra room.
   */
  width?: SettingsPageWidth;
  /** Floating save bar (or null for per-item / immediate pages). */
  saveBar?: ReactNode;
  children: ReactNode;
}

export type SettingsPageWidth = "form" | "wide" | "canvas";

const WIDTH_CLASS: Record<SettingsPageWidth, string> = {
  form: "max-w-settings-page-form",
  wide: "max-w-settings-page-wide",
  canvas: "max-w-settings-page-canvas",
};

/**
 * Settings page scaffold (design system §04): centered column (768px forms /
 * 960px listings), optional consequence line + quiet meta, optional restart
 * notice, groups, and the floating save bar inside the scroll region.
 */
export function SettingsPageFrame({
  slug,
  description,
  meta,
  restart,
  width = "form",
  saveBar,
  children,
}: SettingsPageFrameProps) {
  const hasSubhead = Boolean(description || (meta && meta.length > 0));

  return (
    <div
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
      data-testid={`settings-page-${slug}`}
    >
      <div className="min-h-0 flex-1 overflow-y-auto" data-testid={`settings-page-${slug}-body`}>
        <div
          className={cn(
            "mx-auto flex w-full flex-col gap-6 px-6 pt-5 pb-settings-page-bottom",
            WIDTH_CLASS[width]
          )}
        >
          {hasSubhead ? (
            <div
              className="flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-3.5 text-form-label text-subtle"
              data-testid={`settings-page-${slug}-subhead`}
            >
              {description ? (
                <span className="max-w-settings-page-description">{description}</span>
              ) : null}
              {meta?.map((entry, index) => (
                <span className="inline-flex items-center gap-2" key={entry.key}>
                  {description || index > 0 ? (
                    <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                  ) : null}
                  {entry.content}
                </span>
              ))}
            </div>
          ) : null}
          {restart ? <SettingsRestartNotice restart={restart} slug={slug} /> : null}
          {children}
          {saveBar}
        </div>
      </div>
    </div>
  );
}
