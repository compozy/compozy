import { LayoutGridIcon, MoonIcon, SunIcon } from "lucide-react";

import {
  Button,
  NativeSelect,
  NativeSelectOption,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  buttonVariants,
} from "@compozy/ui";

import { prototypeParams, setParams, type PrototypeMeta } from "#gallery/contract";
import { selectionHref, updateView, type Theme } from "#gallery/shell/view";

/**
 * Prototype-only control (not part of any design): open in the gallery, state picker and theme
 * toggle, so every state can be reviewed standalone. Hidden with `?chrome=0` for the canvas and
 * screenshots.
 */
export function PrototypeToolbar({
  slug,
  meta,
  state,
  theme,
}: {
  slug: string;
  meta: PrototypeMeta;
  state: string;
  theme: Theme;
}) {
  const galleryHref = selectionHref({ kind: "prototype", slug, state }, window.location.search);
  const nextTheme = theme === "dark" ? "light" : "dark";
  const themeLabel = theme === "dark" ? "Use light theme" : "Use dark theme";
  return (
    <div
      className="fixed right-3 bottom-3 z-50 flex items-center gap-1 rounded-lg bg-elevated p-1 shadow-pop"
      data-prototype-chrome
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <a
              aria-label="Open in the gallery"
              className={buttonVariants({ variant: "quiet", size: "sm" })}
              href={galleryHref}
            />
          }
        >
          <LayoutGridIcon aria-hidden="true" />
          <span className="max-w-48 truncate">{meta.title}</span>
        </TooltipTrigger>
        <TooltipContent>Open in the gallery</TooltipContent>
      </Tooltip>
      {meta.states.length > 1 ? (
        <NativeSelect
          aria-label="Prototype state"
          // Each state is a history entry: Back returns to the previous one.
          onChange={event =>
            setParams({ [prototypeParams.state]: event.target.value }, { push: true })
          }
          size="sm"
          value={state}
        >
          {meta.states.map(option => (
            <NativeSelectOption key={option.id} value={option.id}>
              {option.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      ) : null}
      <Button
        aria-label={themeLabel}
        onClick={() => updateView({ theme: nextTheme })}
        size="icon-sm"
        variant="quiet"
      >
        {theme === "dark" ? <SunIcon aria-hidden="true" /> : <MoonIcon aria-hidden="true" />}
      </Button>
    </div>
  );
}
