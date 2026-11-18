/** Logo tile and catalog card, on the shipped CatalogCard. */
import { CatalogCard, cn } from "@compozy/ui";
import { ArrowUpRight } from "@compozy/ui/icons";

import { categoryLabel, firstSentence, type Entry } from "./catalog";

const SIZE = {
  sm: "size-10 rounded-[11px]",
  md: "size-12 rounded-[13px]",
  lg: "size-[72px] rounded-[18px]",
  xl: "size-[88px] rounded-[22px]",
};

export function EntryLogo({
  entry,
  size = "md",
  className,
}: {
  entry: Entry;
  size?: keyof typeof SIZE;
  className?: string;
}) {
  const base = cn(
    "relative grid shrink-0 place-items-center overflow-hidden shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)]",
    SIZE[size],
    className
  );
  if (!entry.icon) {
    return (
      <span
        aria-hidden
        className={cn(
          base,
          "bg-[linear-gradient(135deg,#242424,#141414)] font-display text-[45%] text-fg/80"
        )}
        style={{ fontSize: size === "xl" ? 38 : size === "lg" ? 30 : 20 }}
      >
        {entry.name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return (
    <span aria-hidden className={cn(base, entry.light ? "bg-white" : "bg-[#1c1c1c]")}>
      <img
        src={entry.icon}
        alt=""
        draggable={false}
        loading="lazy"
        className={cn("block object-contain", entry.bleed ? "size-full" : "size-[66%]")}
      />
    </span>
  );
}

export function EntryCard({
  entry,
  onOpen,
  showCategory = false,
}: {
  entry: Entry;
  onOpen: (id: string) => void;
  showCategory?: boolean;
}) {
  const byline = showCategory
    ? categoryLabel(entry.category)
    : entry.author
      ? `By ${entry.author}`
      : null;
  return (
    <CatalogCard
      actionable
      className="group/card relative min-h-[148px] gap-3.5 rounded-[16px] border border-[color:var(--light-070)] bg-[color:var(--light-025)] p-5 shadow-none transition-[background-color,border-color,transform] duration-300 hover:-translate-y-0.5 hover:border-[color:var(--light-150)] hover:bg-[color:var(--light-050)]"
    >
      <div className="flex items-center gap-3.5">
        <EntryLogo entry={entry} />
        <div className="min-w-0">
          <h3 className="m-0 truncate text-[16px]/[1.3] font-medium text-fg">
            <button
              type="button"
              onClick={() => onOpen(entry.id)}
              className="cursor-pointer truncate bg-transparent p-0 text-left font-medium text-fg outline-none after:absolute after:inset-0 after:rounded-[16px] focus-visible:after:ring-2 focus-visible:after:ring-brand"
            >
              {entry.name}
            </button>
          </h3>
          {byline ? <p className="m-0 truncate text-[13px]/[1.4] text-subtle">{byline}</p> : null}
        </div>
        <ArrowUpRight
          aria-hidden
          className="absolute top-3.5 right-3.5 size-4 text-fg/0 transition-colors duration-300 group-hover/card:text-fg/60"
        />
      </div>
      <p className="m-0 line-clamp-2 text-[14px]/[1.5] text-muted-foreground">
        {firstSentence(entry.description)}
      </p>
    </CatalogCard>
  );
}
