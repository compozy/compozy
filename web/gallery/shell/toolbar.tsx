/**
 * Prototype toolbar: prev/next and the breadcrumb (spec › prototype › state, each a link), open
 * in a new tab, copy link and help; then the state picker, viewport, theme and theme comparison.
 * `children` render at the end of the second row (review actions).
 */
import { ChevronLeftIcon, ChevronRightIcon, Columns2Icon, ExternalLinkIcon } from "lucide-react";
import type { ReactNode } from "react";

import {
  NativeSelect,
  NativeSelectOption,
  Separator,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  buttonVariants,
} from "@compozy/ui";

import { copyTargets, prototypeCrumbs } from "#gallery/shell/crumbs";
import type { Entry, Group } from "#gallery/shell/registry-model";
import { ShortcutsHelp } from "#gallery/shell/shortcuts";
import { CopyLinkMenu, IconAction, GalleryThemeToggle, ViewHeading } from "#gallery/shell/topbar";
import { frameSrc, select, updateView, type GalleryView } from "#gallery/shell/view";
import { ViewportControl } from "#gallery/shell/viewport-control";

type ToolbarProps = {
  entry: Entry;
  group: Group | undefined;
  view: GalleryView;
  state: string;
  dark: boolean;
  helpOpen: boolean;
  onHelp: (open: boolean) => void;
  onStep: (direction: 1 | -1) => void;
  children?: ReactNode;
};

function identity(entry: Entry) {
  const { meta } = entry;
  return [meta.surface, meta.route, `/${entry.slug}`, entry.kind === "draft" ? "draft" : null]
    .filter(Boolean)
    .join(" · ");
}

export function GalleryToolbar(props: ToolbarProps) {
  const { entry, view, state } = props;
  const { meta } = entry;
  const crumbs = prototypeCrumbs(props.group, entry, state);
  return (
    <header className="flex flex-col gap-2" data-testid="gallery-toolbar">
      <div className="flex min-h-10 items-center gap-1">
        <IconAction label="Previous prototype" onClick={() => props.onStep(-1)} shortcut="K">
          <ChevronLeftIcon aria-hidden="true" />
        </IconAction>
        <IconAction label="Next prototype" onClick={() => props.onStep(1)} shortcut="J">
          <ChevronRightIcon aria-hidden="true" />
        </IconAction>
        <div className="ml-1 min-w-0 flex-1">
          <ViewHeading crumbs={crumbs} meta={identity(entry)} />
        </div>
        <Tooltip>
          <TooltipTrigger
            render={
              <a
                aria-label="Open in a new tab"
                className={buttonVariants({ variant: "quiet", size: "icon-sm" })}
                href={frameSrc(entry.slug, { state, theme: view.theme }, true)}
                rel="noreferrer"
                target="_blank"
              />
            }
          >
            <ExternalLinkIcon aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent>Open in a new tab</TooltipContent>
        </Tooltip>
        <CopyLinkMenu links={copyTargets(crumbs)} />
        <ShortcutsHelp onOpenChange={props.onHelp} open={props.helpOpen} />
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        {meta.states.length > 1 ? (
          <NativeSelect
            aria-label="State"
            className="max-w-64"
            data-testid="state-select"
            onChange={event =>
              select({ kind: "prototype", slug: entry.slug, state: event.target.value })
            }
            size="sm"
            value={state}
          >
            {meta.states.map((option, index) => (
              <NativeSelectOption key={option.id} value={option.id}>
                {index + 1}. {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        ) : null}
        <span aria-hidden="true" className="flex-1" />
        <ViewportControl width={view.width} />
        <GalleryThemeToggle dark={props.dark} />
        <IconAction
          aria-pressed={view.compare}
          label={view.compare ? "Stop comparing themes" : "Compare light and dark"}
          onClick={() => updateView({ compare: !view.compare })}
        >
          <Columns2Icon aria-hidden="true" />
        </IconAction>
        {props.children ? (
          <>
            <Separator className="mx-0.5 h-5" orientation="vertical" />
            {props.children}
          </>
        ) : null}
      </div>
    </header>
  );
}
