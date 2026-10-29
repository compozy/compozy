/**
 * Shared top bar pieces of every gallery view: icon actions with tooltips, the theme toggle, the
 * breadcrumb (each level links to its own view) and the "copy link" menu, which copies the link of
 * any level on screen (state, prototype, spec).
 */
import { LinkIcon, MoonIcon, SunIcon } from "lucide-react";
import { Fragment, type ComponentProps, type ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
  Kbd,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  toast,
} from "@compozy/ui";

import type { Crumb } from "#gallery/shell/crumbs";
import { SelectionLink } from "#gallery/shell/selection-link";
import { selectionUrl, updateView } from "#gallery/shell/view";

export function IconAction({
  label,
  shortcut,
  children,
  ...props
}: ComponentProps<typeof Button> & { label: string; shortcut?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={<Button aria-label={label} size="icon-sm" variant="quiet" {...props} />}
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>
        {label}
        {shortcut ? <Kbd>{shortcut}</Kbd> : null}
      </TooltipContent>
    </Tooltip>
  );
}

export function GalleryThemeToggle({ dark }: { dark: boolean }) {
  return (
    <IconAction
      label={dark ? "Use light theme" : "Use dark theme"}
      onClick={() => updateView({ theme: dark ? "light" : "dark" })}
      shortcut="T"
    >
      {dark ? <SunIcon aria-hidden="true" /> : <MoonIcon aria-hidden="true" />}
    </IconAction>
  );
}

/** Breadcrumb of the view; the last crumb is the page itself, the others link to their view. */
function GalleryCrumbs({ crumbs }: { crumbs: readonly Crumb[] }) {
  return (
    <nav aria-label="Breadcrumb" className="min-w-0">
      <ol className="flex min-w-0 items-center gap-1 text-small-body text-muted">
        {crumbs.map((crumb, index) => {
          const last = index === crumbs.length - 1;
          return (
            <Fragment key={`${crumb.target.kind}:${crumb.label}`}>
              <li className="min-w-0 truncate">
                {last ? (
                  <h1 aria-current="page" className="truncate text-small-body font-medium text-fg">
                    {crumb.label}
                  </h1>
                ) : (
                  <SelectionLink
                    className="rounded-xs transition-colors outline-none hover:text-fg focus-visible:shadow-focus-ring"
                    to={crumb.target}
                  >
                    {crumb.label}
                  </SelectionLink>
                )}
              </li>
              {last ? null : (
                <li aria-hidden="true" className="text-faint">
                  /
                </li>
              )}
            </Fragment>
          );
        })}
      </ol>
    </nav>
  );
}

/** Title block of a view: breadcrumb plus a mono line of identifiers. */
export function ViewHeading({ crumbs, meta }: { crumbs: readonly Crumb[]; meta: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col">
      <GalleryCrumbs crumbs={crumbs} />
      <span className="truncate font-mono text-mono-id tracking-mono-id text-subtle">{meta}</span>
    </div>
  );
}

function copy(url: string, label: string) {
  void navigator.clipboard.writeText(url).then(
    () => toast.success("Link copied", { description: label }),
    () => toast.error("Could not copy the link")
  );
}

/** Copies the link of one level of what is on screen, from the deepest to the widest. */
export function CopyLinkMenu({ links }: { links: readonly Crumb[] }) {
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label="Copy link"
                  data-testid="copy-link"
                  size="icon-sm"
                  variant="quiet"
                />
              }
            />
          }
        >
          <LinkIcon aria-hidden="true" />
        </TooltipTrigger>
        <TooltipContent>Copy link</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuGroup>
          <DropdownMenuLabel>Copy link to</DropdownMenuLabel>
          {links.map(link => (
            <DropdownMenuItem
              data-testid={`copy-link-${link.target.kind}`}
              key={`${link.target.kind}:${link.label}`}
              onClick={() =>
                copy(
                  selectionUrl(link.target, window.location.search, window.location.href),
                  link.label
                )
              }
            >
              <span className="flex min-w-0 flex-col">
                <span className="truncate">{link.label}</span>
                {link.detail ? (
                  <span className="truncate font-mono text-mono-id tracking-mono-id text-subtle">
                    {link.detail}
                  </span>
                ) : null}
              </span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
