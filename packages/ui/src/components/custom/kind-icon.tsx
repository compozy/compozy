import * as React from "react";
import { Bot, type LucideIcon } from "lucide-react";

import { cn } from "../../lib/utils";
import {
  providerKindIconRegistry,
  type KindIconRegistry,
  type KindIconRegistryEntry,
} from "./kind-icon-registry";

type KindIconTone = "default" | "muted" | "accent";
type KindIconSize = "xs" | "sm" | "md";
type DataAttributes = {
  [key: `data-${string}`]: string | undefined;
};

interface KindIconProps<K extends string = string>
  extends Omit<React.ComponentProps<"span">, "children">, DataAttributes {
  fallback?: LucideIcon;
  kind: K | (string & {});
  registry?: KindIconRegistry<K>;
  size?: KindIconSize;
  tone?: KindIconTone;
}

const KIND_ICON_TONE: Record<KindIconTone, string> = {
  default: "text-fg",
  muted: "text-subtle",
  accent: "text-accent",
};

const KIND_ICON_SIZE: Record<KindIconSize, string> = {
  xs: "size-3",
  sm: "size-4",
  md: "size-5",
};

const KIND_ICON_GLYPH_CLASS = "size-full shrink-0";

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
  size = "sm",
  tone = "muted",
  "data-slot": dataSlot = "kind-icon",
  ...props
}: KindIconProps<K>) {
  const key = normalizeKind(String(kind));
  const entry = registry[key as K];
  return (
    <span
      data-slot={dataSlot}
      data-kind={key}
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        KIND_ICON_SIZE[size],
        KIND_ICON_TONE[tone],
        className
      )}
      {...props}
    >
      <KindIconGlyph className={KIND_ICON_GLYPH_CLASS} entry={entry} fallback={fallback} />
    </span>
  );
}

export { KindIcon };
export type { KindIconProps, KindIconSize, KindIconTone };
