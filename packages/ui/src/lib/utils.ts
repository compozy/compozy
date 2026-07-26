import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const customTwMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        "text-eyebrow",
        "text-badge",
        "text-micro",
        "text-small-body",
        "text-display-2xl",
        "text-site-lead",
        "text-item-title",
        "text-inline-code",
        "text-accent-glyph",
        "text-ui-title-lg",
        "text-pill-group-badge",
        "text-detail-h1",
        "text-empty-h1",
        "text-modal-title",
        "text-section-head",
        "text-form-input",
        "text-form-label",
        "text-form-hint",
        "text-form-required",
        "text-metric-value",
        "text-kpi-value",
        "text-kpi-compact",
        "text-card-title",
        "text-agent-metric",
        "text-rail-avatar",
        "text-ws-name",
        "text-mono-id",
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return customTwMerge(clsx(inputs));
}
