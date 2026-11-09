// Shared class strings for the manual. Bands set --b-* variables (see styles.css), so the
// same card, label and caption classes work on ink, paper and coloured bands.

/** small uppercase mono label */
export const lab = "mono-label font-medium text-(--b-muted)";

/** default card: sits on any band */
export const card = "min-w-0 rounded-card border border-(--b-line) bg-(--b-card) p-[22px] sm:p-7";

/** flat card without border, for dense grids */
export const cardFlat = "min-w-0 rounded-card bg-(--b-card) p-[22px] sm:p-7";

/** h3 in display face */
export const h3 =
  "mb-2 font-display text-[22px] font-medium leading-[1.2] tracking-[-0.02em] text-(--b-fg)";

/** small caption under a figure */
export const cap = "mt-2.5 text-sm text-(--b-muted)";

/** bulleted list */
export const rules =
  "m-0 list-disc pl-[18px] marker:text-(--b-accent) [&_li]:mb-2 [&_strong]:font-semibold";

/** definition rows: <dl className={specs}><div><dt/><dd/></div></dl> */
export const specs =
  "m-0 grid gap-0 [&>div]:grid [&>div]:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)] [&>div]:gap-4 [&>div]:border-b [&>div]:border-(--b-line) [&>div]:py-2.5 [&_dt]:font-mono [&_dt]:text-[13px] [&_dt]:text-(--b-muted) [&_dd]:m-0 [&_dd]:text-[15px]";

/** buttons */
export const btn =
  "inline-flex cursor-pointer items-center justify-center gap-2 rounded-[10px] border border-transparent font-medium no-underline transition-colors duration-150";
export const btnSm = "min-h-9 px-3.5 text-[13px]";
export const btnSecondary =
  "border-(--b-line) bg-transparent text-(--b-fg) hover:border-(--b-accent)";
/** the volt action fill (brand palette token from @compozy/ui) */
export const voltFill = "bg-(--color-brand-volt) text-(--color-fg-strong)";

/** inline code chip */
export const chip =
  "inline-flex items-center rounded-md border border-(--b-line) px-2 py-0.5 font-mono text-[12px] text-(--b-accent)";

/** full-width section band */
export const band = "py-[clamp(64px,8vw,120px)] scroll-mt-28";

// Ported with the chapter bodies (S4); names match the Astro source cls.ts.

/** body text that is secondary */
export const muted = "text-(--b-muted)";

/** grids */
export const grid2 = "grid gap-4 md:grid-cols-2";
export const grid3 = "grid gap-4 sm:grid-cols-2 lg:grid-cols-3";
export const grid4 = "grid gap-4 sm:grid-cols-2 lg:grid-cols-4";

export const btnMd = "min-h-11 px-5 text-[15px]";
/** Volt fills keep white text, so they hover darker (deep, 7.5:1), never to the theme brand hover. */
export const voltHover = "hover:bg-(--color-brand-deep)";
export const btnPrimary = `${voltFill} ${voltHover}`;
