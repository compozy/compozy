import { useState } from "react";
import { Kbd, KbdGroup, PillGroup, SearchInput, Toaster, TooltipProvider } from "@agh/ui";
import { SparklesIcon } from "lucide-react";
import { DesignSystemShowcaseLink } from "./design-system-showcase-link";

import {
  ButtonsAndPillsSection,
  InputsAndSearchSection,
  StatusAndMetricSection,
} from "./design-system-showcase-controls";
import {
  CodeAndChatSection,
  LayoutSection,
  ListingRowSection,
} from "./design-system-showcase-layouts";
import { FeedbackSection, OverlaysSection } from "./design-system-showcase-feedback";
import { FoundationsTokenSection, TypographySection } from "./design-system-showcase-foundations";

const FILTERS = [
  { label: "All", value: "all" },
  { label: "Primitives", value: "primitives" },
  { label: "Surfaces", value: "surfaces" },
  { label: "Motion", value: "motion" },
] as const;

type FilterValue = (typeof FILTERS)[number]["value"];

export function DesignSystemShowcase() {
  const [filter, setFilter] = useState<FilterValue>("all");
  const [searchValue, setSearchValue] = useState("");

  return (
    <TooltipProvider>
      <main
        data-testid="design-system-showcase"
        className="flex min-h-dvh flex-col bg-canvas text-fg"
      >
        <header
          data-slot="page-header"
          className="flex min-h-11 flex-col gap-2 border-b border-line px-4 py-2.5"
        >
          <div
            data-slot="page-header-main"
            className="flex min-w-0 flex-wrap items-center gap-2 sm:gap-3"
          >
            <div data-slot="page-header-title" className="flex min-w-0 items-center gap-2">
              <span
                aria-hidden="true"
                data-slot="page-header-icon"
                className="inline-flex size-6 shrink-0 items-center justify-center rounded-sm bg-elevated text-accent"
              >
                <SparklesIcon className="size-3" />
              </span>
              <h1 className="truncate text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">
                AGH design system
              </h1>
              <span
                data-slot="page-header-count"
                className="inline-flex h-[19px] min-w-[19px] items-center justify-center rounded-mono-badge bg-canvas-soft px-1.5 font-mono text-[10.5px] font-medium tabular-nums text-muted"
              >
                v1
              </span>
            </div>
            <div
              data-slot="page-header-meta"
              className="ml-auto flex shrink-0 items-center gap-2 text-[13px] text-muted"
            >
              <DesignSystemShowcaseLink />
            </div>
          </div>
        </header>

        <div
          role="toolbar"
          aria-label="Showcase filters"
          className="flex min-h-11 flex-wrap items-center gap-3 border-b border-line bg-canvas-soft px-4 py-2"
        >
          <PillGroup
            value={filter}
            onChange={(next: FilterValue) => setFilter(next)}
            items={FILTERS.map(item => ({ label: item.label, value: item.value }))}
          />
          <SearchInput
            value={searchValue}
            onChange={setSearchValue}
            placeholder="Search primitives…"
            containerClassName="ml-auto w-72"
            aria-label="Search primitives"
            kbd={
              <KbdGroup>
                <Kbd>⌘</Kbd>
                <Kbd>K</Kbd>
              </KbdGroup>
            }
          />
        </div>

        <div className="flex flex-col gap-10 px-6 py-8">
          <FoundationsTokenSection />
          <TypographySection />
          <ButtonsAndPillsSection />
          <InputsAndSearchSection />
          <StatusAndMetricSection />
          <FeedbackSection />
          <OverlaysSection />
          <CodeAndChatSection />
          <LayoutSection />
          <ListingRowSection />
        </div>

        <Toaster />
      </main>
    </TooltipProvider>
  );
}
