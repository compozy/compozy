/**
 * Dialog open should land on a real field, not a HelpTip. HelpTips stay in the
 * tab order (keyboard users must be able to open the prose), but auto-focusing
 * one opens the tip and the first Escape dismisses the tip instead of the dialog.
 */
const HELP_TIP_SLOT = '[data-slot="help-tip"]';

const TABBABLE_SELECTOR = [
  "button:not([disabled])",
  "[href]",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(", ");

export function firstDialogTabbable(root: HTMLElement): HTMLElement | null {
  const nodes = root.querySelectorAll<HTMLElement>(TABBABLE_SELECTOR);
  for (const node of nodes) {
    if (node.closest(HELP_TIP_SLOT)) continue;
    if (node.getAttribute("aria-hidden") === "true") continue;
    return node;
  }
  return null;
}
