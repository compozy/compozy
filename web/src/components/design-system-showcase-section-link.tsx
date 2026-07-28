import { DESIGN_MD_BASE } from "./design-system-showcase-sections";
import type { ShowcaseSection } from "./design-system-showcase-sections";

export function SectionLink({
  section,
  children,
}: {
  section: ShowcaseSection;
  children?: string;
}) {
  return (
    <a
      data-testid={`section-link-${section.id}`}
      data-section-id={section.id}
      data-section-anchor={section.anchor}
      href={`${DESIGN_MD_BASE}${section.anchor}`}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1.5 text-muted transition-colors hover:text-accent"
    >
      <span>{children ?? section.label}</span>
      <span aria-hidden="true" className="font-mono text-badge tracking-mono">
        {"↗"}
      </span>
    </a>
  );
}
