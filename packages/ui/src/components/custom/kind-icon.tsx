import type * as React from "react";
import { Bot, type LucideIcon } from "lucide-react";

import { cn } from "../../lib/utils";
import {
  providerKindIconRegistry,
  type KindIconRegistry,
  type KindIconRegistryEntry,
} from "./kind-icon-registry";

/** `well` sets the glyph on the 26 px mint identity plate (brand-spec rule 3). */
type KindIconTone = "default" | "muted" | "accent" | "well";
type KindIconSize = "xs" | "sm" | "md";
type DataAttributes = {
  [key: `data-${string}`]: string | undefined;
};

type KindIconExplicitGlyph = React.ComponentType<{ className?: string; "aria-hidden"?: true }>;

interface KindIconProps<K extends string = string>
  extends Omit<React.ComponentProps<"span">, "children">, DataAttributes {
  fallback?: LucideIcon;
  /** Glyph source: resolved through `registry`. Ignored when `icon` is set. */
  kind?: K | (string & {});
  /** Explicit glyph that skips the registry (e.g. a verb icon in a dialog well). */
  icon?: KindIconExplicitGlyph;
  /**
   * Pre-rendered glyph node (an app mark published as a ReactNode). Wins over
   * `icon` and `kind`; its `svg` is sized by `size`.
   */
  glyph?: React.ReactNode;
  registry?: KindIconRegistry<K>;
  /** Glyph size; inside the `well` tone it sizes the glyph, not the plate. */
  size?: KindIconSize;
  tone?: KindIconTone;
}

const KIND_ICON_TONE: Record<KindIconTone, string> = {
  default: "text-fg",
  muted: "text-subtle",
  accent: "text-accent",
  well: "size-6.5 rounded-icon-well bg-well text-success",
};

const KIND_ICON_SIZE: Record<KindIconSize, string> = {
  xs: "size-3",
  sm: "size-4",
  md: "size-5",
};

const KIND_ICON_GLYPH_CLASS = "size-full shrink-0";

// A node glyph cannot take a className, so its svg is sized from the plate.
const KIND_ICON_NODE_SIZE: Record<KindIconSize, string> = {
  xs: "[&_svg]:size-3",
  sm: "[&_svg]:size-4",
  md: "[&_svg]:size-5",
};

function normalizeKind(kind: string): string {
  return kind.trim().toLowerCase();
}

interface KindIconGlyphPropsForEntry {
  className: string;
  entry: KindIconRegistryEntry | undefined;
  fallback: LucideIcon;
}

function KindIconGlyph({ className, entry, fallback }: KindIconGlyphPropsForEntry) {
  if (typeof entry === "function") {
    const Icon = entry;
    return <Icon aria-hidden="true" className={className} />;
  }

  if (entry?.render) {
    return entry.render({ "aria-hidden": true, className });
  }

  if (entry?.brand) {
    const Brand = entry.brand;
    return <Brand aria-hidden="true" className={className} />;
  }

  const Icon = entry?.fallback ?? fallback;
  return <Icon aria-hidden="true" className={className} />;
}

function KindIcon<K extends string = string>({
  className,
  fallback = Bot,
  kind,
  registry = providerKindIconRegistry as KindIconRegistry<K>,
  icon: ExplicitIcon,
  glyph,
  size = "sm",
  tone = "muted",
  "data-slot": dataSlot = "kind-icon",
  ...props
}: KindIconProps<K>) {
  const key = kind === undefined ? undefined : normalizeKind(String(kind));
  const well = tone === "well";
  // A well is a fixed plate; `size` then sizes the glyph inside it.
  const glyphClass = well ? cn(KIND_ICON_SIZE[size], "shrink-0") : KIND_ICON_GLYPH_CLASS;
  return (
    <span
      data-slot={dataSlot}
      data-kind={key}
      data-tone={tone}
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        !well && KIND_ICON_SIZE[size],
        KIND_ICON_TONE[tone],
        glyph != null && (well ? KIND_ICON_NODE_SIZE[size] : "[&_svg]:size-full"),
        "[&_svg]:shrink-0",
        className
      )}
      {...props}
    >
      {glyph != null ? (
        glyph
      ) : ExplicitIcon ? (
        <ExplicitIcon aria-hidden className={glyphClass} />
      ) : (
        <KindIconGlyph
          className={glyphClass}
          entry={key === undefined ? undefined : registry[key as K]}
          fallback={fallback}
        />
      )}
    </span>
  );
}

export { KindIcon };
export type { KindIconProps, KindIconSize, KindIconTone };
