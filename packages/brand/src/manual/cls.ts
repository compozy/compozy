// Shared class strings for the manual. Bands set --b-* variables (see styles.css), so the
// same card, label and caption classes work on ink, paper and coloured bands.

/** small uppercase mono label */
export const lab = "mono-label font-medium text-(--b-muted)";

/** default card: sits on any band */
export const card = "min-w-0 rounded-card border border-(--b-line) bg-(--b-card) p-[22px] sm:p-7";

/** h3 in display face */
export const h3 =
  "mb-2 font-display text-[22px] font-medium leading-[1.2] tracking-[-0.02em] text-(--b-fg)";

/** bulleted list */
export const rules =
  "m-0 list-disc pl-[18px] marker:text-(--b-accent) [&_li]:mb-2 [&_strong]:font-semibold";

/** buttons */
export const btn =
  "inline-flex cursor-pointer items-center justify-center gap-2 rounded-[10px] border border-transparent font-medium no-underline transition-colors duration-150";
export const btnSm = "min-h-9 px-3.5 text-[13px]";
export const btnSecondary =
  "border-(--b-line) bg-transparent text-(--b-fg) hover:border-(--b-accent)";
/** the volt action fill (brand palette token from @compozy/ui) */
export const voltFill = "bg-(--color-brand-volt) text-(--color-brand-ink)";

/** full-width section band */
export const band = "py-[clamp(64px,8vw,120px)] scroll-mt-28";
