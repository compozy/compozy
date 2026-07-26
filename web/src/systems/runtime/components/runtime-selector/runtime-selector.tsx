import { RefreshCw, Search } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { cn, Popover, PopoverContent } from "@agh/ui";

import { ModelList } from "./model-list";
import { ProviderRail } from "./provider-rail";
import { ReasoningBar } from "./reasoning-bar";
import { RuntimeSelectorTrigger } from "./trigger";
import { useRuntimeSelector } from "./use-runtime-selector";
import { useRuntimeSelectorPopup } from "./use-runtime-selector-popup";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
  RuntimeSelectorVariant,
} from "./types";

export interface RuntimeSelectorProps {
  value: RuntimeSelectorValue;
  onChange: (next: RuntimeSelectorValue) => void;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  variant?: RuntimeSelectorVariant;
  disabled?: boolean;
  /** Keeps the trigger focusable and visible while preventing edits. */
  readOnly?: boolean;
  /** Catalog is loading (drives the list loading state). */
  loading?: boolean;
  /**
   * The catalog query has RESOLVED at least once (not merely "not loading"):
   * drives the strict favorites/recents purge so a loaded-empty catalog wipes
   * stale persisted entries while a still-loading/disabled query never does.
   */
  catalogLoaded?: boolean;
  modelPlaceholder?: string;
  /** Catalog refresh affordance absorbed from the legacy CatalogStatusLine block. */
  onRefreshCatalog?: () => void;
  refreshing?: boolean;
  /** Rendered under the search header (stale/error/count status). */
  catalogStatus?: ReactNode;
  onOpenProviderSettings?: () => void;
  triggerId?: string;
  triggerTestId?: string;
  /** id of the surface's visible caption; names the trigger group via aria-labelledby. */
  ariaLabelledby?: string;
  className?: string;
}

export function RuntimeSelector({
  value,
  onChange,
  providers,
  models,
  variant = "default",
  disabled = false,
  readOnly = false,
  loading = false,
  catalogLoaded = false,
  modelPlaceholder = "Select model",
  onRefreshCatalog,
  refreshing = false,
  catalogStatus,
  onOpenProviderSettings,
  triggerId,
  triggerTestId,
  ariaLabelledby,
  className,
}: RuntimeSelectorProps) {
  const controller = useRuntimeSelector({ value, onChange, providers, models, catalogLoaded });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const popup = useRuntimeSelectorPopup({
    controller,
    providers,
    disabled: disabled || readOnly,
    triggerRef,
    searchRef,
  });
  const modelName = controller.selectedModel?.name ?? value.model;

  // Provider Settings closes the popup FIRST, then hands off to the surface (which
  // closes its own dialog/flow and navigates to /settings/providers) — never an
  // inert gear left open over the create surface.
  const handleOpenSettings = onOpenProviderSettings
    ? () => {
        controller.close();
        onOpenProviderSettings();
      }
    : undefined;

  return (
    <Popover open={controller.open} onOpenChange={popup.handleOpenChange}>
      <RuntimeSelectorTrigger
        ref={triggerRef}
        id={triggerId}
        data-testid={triggerTestId}
        className={className}
        value={value}
        provider={controller.activeProvider}
        model={controller.selectedModel}
        variant={variant}
        open={controller.open}
        disabled={disabled}
        readOnly={readOnly}
        needsAuth={controller.activeProvider?.needs_auth}
        modelPlaceholder={modelPlaceholder}
        popupId={popup.popupId}
        ariaLabelledby={ariaLabelledby}
        onPress={popup.handleTriggerPress}
        onKeyDown={popup.handleTriggerKeyDown}
      />
      <PopoverContent
        align="start"
        sideOffset={6}
        anchor={popup.anchor}
        initialFocus={popup.resolveInitialFocus}
        finalFocus={popup.finalFocus}
        aria-label="Runtime selector"
        className="max-h-[min(520px,var(--available-height))] w-[min(528px,94vw)] overflow-hidden p-0 shadow-overlay"
      >
        <div
          id={popup.popupId}
          className="flex max-h-[inherit] flex-col"
          data-testid="runtime-selector-popup"
        >
          <div className="flex h-11 shrink-0 items-center gap-2.5 border-b border-line-soft px-3">
            <Search aria-hidden="true" className="size-4 shrink-0 text-subtle" />
            <input
              ref={searchRef}
              type="text"
              role="combobox"
              aria-label="Search models and providers"
              aria-expanded
              aria-controls={popup.listId}
              aria-autocomplete="list"
              aria-activedescendant={popup.activeDescendant}
              value={controller.query}
              onChange={event => controller.changeQuery(event.target.value)}
              onKeyDown={popup.handleSearchKeyDown}
              placeholder="Search models, providers…"
              autoComplete="off"
              spellCheck={false}
              data-testid="runtime-selector-search"
              className="min-w-0 flex-1 bg-transparent text-small-body text-fg-strong outline-none placeholder:text-subtle"
            />
            {onRefreshCatalog ? (
              <button
                type="button"
                aria-label="Refresh model catalog"
                title="Refresh catalog"
                data-testid="runtime-selector-refresh"
                disabled={refreshing}
                onClick={() => onRefreshCatalog()}
                className="grid size-button-icon-default shrink-0 place-items-center rounded text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={cn("size-3.5", refreshing && "animate-spin")}
                />
              </button>
            ) : null}
          </div>
          {catalogStatus ? (
            <div
              className="shrink-0 border-b border-line-soft px-3 py-1.5 text-badge text-subtle"
              data-testid="runtime-selector-status"
            >
              {catalogStatus}
            </div>
          ) : null}
          <div className="flex min-h-0 flex-1">
            <ProviderRail
              providers={providers}
              railFilter={controller.railFilter}
              searching={controller.listModel.searching}
              onRail={controller.changeRail}
              onOpenSettings={handleOpenSettings}
            />
            <ModelList
              listId={popup.listId}
              optionId={popup.optionId}
              listModel={controller.listModel}
              highlightIndex={controller.highlightIndex}
              loading={loading}
              providerName={popup.providerName}
              providerKind={popup.providerKind}
              onSelect={controller.pickModel}
              onHover={controller.highlightRow}
              highlightedFavorite={
                controller.highlightedRow
                  ? {
                      name: controller.highlightedRow.model.name,
                      providerName: popup.providerName(controller.highlightedRow.model.provider),
                      favorite: controller.highlightedRow.favorite,
                    }
                  : undefined
              }
              onToggleHighlightedFavorite={controller.toggleHighlightedFavorite}
              onCustomCommit={controller.pickCustom}
              onFocusSearch={() => searchRef.current?.focus()}
            />
          </div>
          <ReasoningBar
            reasoning={controller.reasoningState}
            value={value.reasoning_effort}
            providerName={popup.providerName(value.provider)}
            modelName={modelName || value.provider}
            onSelect={controller.setReasoning}
          />
          {/* Polite status: the favorite button's aria-pressed flip is not
              announced while focus stays in search (Alt+F), so speak the result. */}
          <span
            role="status"
            aria-live="polite"
            data-testid="runtime-selector-announcer"
            className="sr-only"
          >
            {controller.favoriteAnnouncement}
          </span>
        </div>
      </PopoverContent>
    </Popover>
  );
}
