import {
  CircleAlert,
  Link2,
  Lock,
  MoreHorizontal,
  Pencil,
  Play,
  SkipForward,
  Trash2,
} from "lucide-react";
import type { ComponentProps } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Pill,
  Time,
  cn,
} from "@compozy/ui";

import { automationLastRunMeta } from "../lib/automation-formatters";
import { AUTOMATION_START_ICON } from "../lib/automation-start-icon";
import type { AutomationSentence } from "../lib/automation-sentence";
import type { AutomationStart, AutomationView } from "../lib/automation-view";

/** Kind glyph for the icon well: `clock-3` schedule · `radio` event · `webhook` link. */
export function AutomationStartGlyph({
  start,
  className,
  ...props
}: { start: AutomationStart } & ComponentProps<"svg">) {
  const Glyph = AUTOMATION_START_ICON[start];
  return (
    <Glyph aria-hidden="true" className={cn("size-4", className)} data-start={start} {...props} />
  );
}

/** Sentence with the glue muted and the varying parts one step stronger. */
export function AutomationSentenceText({
  sentence,
  className,
  ...props
}: { sentence: AutomationSentence } & ComponentProps<"span">) {
  return (
    <span className={className} {...props}>
      {sentence.map((segment, index) =>
        segment.emphasis ? (
          <em
            className={cn(
              "text-fg-2 not-italic",
              segment.missing && "border-b border-dashed border-warning text-warning"
            )}
            key={`${index}-${segment.text}`}
          >
            {segment.text}
          </em>
        ) : (
          <span key={`${index}-${segment.text}`}>{segment.text}</span>
        )
      )}
    </span>
  );
}

/** Exception-only badges: hollow "Off" and the config/package lock. */
export function AutomationBadges({ view }: { view: Pick<AutomationView, "enabled" | "source"> }) {
  return (
    <>
      {view.enabled ? null : (
        <Pill data-testid="automation-off-badge" form="hollow" size="xs">
          Off
        </Pill>
      )}
      {view.source === "dynamic" ? null : (
        <Pill data-testid="automation-source-badge" size="xs">
          <Lock aria-hidden="true" className="size-3" />
          {view.source === "config" ? "From config" : "From package"}
        </Pill>
      )}
    </>
  );
}

/** Last-run truth: danger text + glyph for a failed run, neutral otherwise, nothing if never ran. */
export function AutomationLastRunText({ view }: { view: AutomationView }) {
  const meta = automationLastRunMeta(view.lastRun);
  if (!meta) return null;
  return (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1",
        meta.tone === "danger" ? "text-danger" : "text-subtle"
      )}
      data-testid={`automation-last-run-${view.id}`}
      data-tone={meta.tone}
    >
      {meta.glyph === "fail" ? <CircleAlert aria-hidden="true" className="size-3" /> : null}
      {meta.glyph === "skip" ? <SkipForward aria-hidden="true" className="size-3" /> : null}
      <span className="truncate">
        {meta.text}
        {meta.at ? (
          <>
            {" "}
            <Time iso={meta.at} />
          </>
        ) : null}
      </span>
    </span>
  );
}

export function AutomationPublicLinkText({ view }: { view: AutomationView }) {
  if (view.start !== "webhook") return null;
  return <span>{view.publicLinkLive ? "Public link live" : "Public link off"}</span>;
}

export interface AutomationOverflowHandlers {
  onRunNow: (view: AutomationView) => void;
  onEdit: (view: AutomationView) => void;
  onDelete: (view: AutomationView) => void;
  onCopyLink: (view: AutomationView) => void;
}

interface AutomationOverflowMenuProps extends AutomationOverflowHandlers {
  view: AutomationView;
  runDisabled: boolean;
  runPending: boolean;
}

/** Only the actions the daemon supports for this kind and source (Business Rules 10–11). */
export function AutomationOverflowMenu({
  view,
  runDisabled,
  runPending,
  onRunNow,
  onEdit,
  onDelete,
  onCopyLink,
}: AutomationOverflowMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-haspopup="menu"
            aria-label={`More actions for ${view.name}`}
            data-testid={`automation-more-${view.id}`}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        <MoreHorizontal aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {view.canRunNow ? (
          <DropdownMenuItem
            data-testid={`automation-run-now-${view.id}`}
            disabled={runDisabled || runPending}
            onClick={() => onRunNow(view)}
          >
            <Play aria-hidden="true" />
            {runPending ? "Starting…" : "Run now"}
          </DropdownMenuItem>
        ) : null}
        {view.webhookPath ? (
          <DropdownMenuItem onClick={() => onCopyLink(view)}>
            <Link2 aria-hidden="true" />
            Copy link
          </DropdownMenuItem>
        ) : null}
        {view.canEdit ? (
          <>
            <DropdownMenuItem
              data-testid={`automation-edit-${view.id}`}
              onClick={() => onEdit(view)}
            >
              <Pencil aria-hidden="true" />
              Edit
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-danger"
              data-testid={`automation-delete-${view.id}`}
              onClick={() => onDelete(view)}
            >
              <Trash2 aria-hidden="true" />
              Delete automation…
            </DropdownMenuItem>
          </>
        ) : view.source === "config" ? (
          <DropdownMenuItem disabled>
            <Lock aria-hidden="true" />
            Edit in config.toml
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
