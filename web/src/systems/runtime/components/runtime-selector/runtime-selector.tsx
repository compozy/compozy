import { Plus, RefreshCw, Search, X } from "lucide-react";
import { useRef, type ReactNode } from "react";

import { type RuntimeSpeed } from "@/lib/api-contract";
import { cn, Popover, PopoverContent } from "@compozy/ui";

import { ModelList } from "./model-list";
import { ProviderChips } from "./provider-chips";
import { SelectorFooter } from "./selector-footer";
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
  modelPlaceholder?: string;
  /** Catalog refresh affordance absorbed from the legacy CatalogStatusLine block. */
  onRefreshCatalog?: () => void;
  refreshing?: boolean;
  /** Rendered under the search header (stale/error/count status). */
  catalogStatus?: ReactNode;
  onOpenProviderSettings?: () => void;
  /**
   * Session-level ACP speed request (PR #267). Both props together render the
   * footer switch and the trigger bolt; leave them unwired on surfaces whose
   * create contract has no `speed` field. The daemon resolves the request at
   * prompt dispatch — this is intent, never a per-model capability claim.
   */
  speed?: RuntimeSpeed;
  onSpeedChange?: (next: RuntimeSpeed) => void;
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
  modelPlaceholder = "Select model",
  onRefreshCatalog,
  refreshing = false,
  catalogStatus,
  onOpenProviderSettings,
  speed,
  onSpeedChange,
  triggerId,
  triggerTestId,
  ariaLabelledby,
  className,
}: RuntimeSelectorProps) {
  const controller = useRuntimeSelector({ value, onChange, providers, models });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const inert = disabled || readOnly;
  const popup = useRuntimeSelectorPopup({
    controller,
    providers,
    disabled: inert,
    triggerRef,
    searchRef,
  });
  const modelName = controller.selectedModel?.name ?? value.model;
  const exactEntry = controller.entryMode === "exact";
  const exactInputId = `${popup.popupId}-exact-model-id`;
  const handleStartExactEntry = () => {
    controller.startExactEntry();
    searchRef.current?.focus();
  };

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
        speed={onSpeedChange ? speed : undefined}
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
        className="max-h-[min(440px,var(--available-height))] w-[min(320px,94vw)] overflow-hidden bg-canvas p-0 shadow-overlay"
      >
        <div
          id={popup.popupId}
          className="flex max-h-[inherit] flex-col"
          data-testid="runtime-selector-popup"
        >
          <div className="flex h-9 shrink-0 items-center gap-2 border-b border-line-soft px-3">
            {exactEntry ? (
              <>
                <Plus aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
                <label htmlFor={exactInputId} className="shrink-0 text-badge font-medium text-fg">
                  Exact model ID
                </label>
              </>
            ) : (
              <Search aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
            )}
            <input
              ref={searchRef}
              id={exactEntry ? exactInputId : undefined}
              type="text"
              role={exactEntry ? undefined : "combobox"}
              aria-label={exactEntry ? undefined : "Search models and providers"}
              aria-expanded={exactEntry ? undefined : true}
              aria-controls={exactEntry ? undefined : popup.listId}
              aria-autocomplete={exactEntry ? undefined : "list"}
              aria-activedescendant={exactEntry ? undefined : popup.activeDescendant}
              aria-keyshortcuts={exactEntry ? undefined : "Alt+F"}
              value={controller.query}
              onChange={event => controller.changeQuery(event.target.value)}
              onKeyDown={popup.handleSearchKeyDown}
              placeholder={exactEntry ? "composer-2.5" : "Search models, providers…"}
              autoComplete="off"
              spellCheck={false}
              data-testid="runtime-selector-search"
              className="min-w-0 flex-1 bg-transparent text-small-body text-fg-strong outline-none placeholder:text-subtle"
            />
            {exactEntry ? (
              <button
                type="button"
                aria-label="Return to model search"
                onClick={controller.cancelExactEntry}
                className="grid size-6 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:ring-2 focus-visible:ring-accent"
              >
                <X aria-hidden="true" className="size-3.5" />
              </button>
            ) : onRefreshCatalog ? (
              <button
                type="button"
                aria-label="Refresh model catalog"
                title="Refresh catalog"
                data-testid="runtime-selector-refresh"
                disabled={refreshing}
                onClick={() => onRefreshCatalog()}
                className="grid size-6 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={cn("size-3.5", refreshing && "animate-spin")}
                />
              </button>
            ) : null}
          </div>
          <ProviderChips
            providers={providers}
            railFilter={controller.railFilter}
            searching={controller.listModel.searching}
            onRail={controller.changeRail}
            onOpenSettings={handleOpenSettings}
          />
          {catalogStatus ? (
            <div
              className="shrink-0 border-b border-line-soft px-3 py-1.5 text-badge text-subtle"
              data-testid="runtime-selector-status"
            >
              {catalogStatus}
            </div>
          ) : null}
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
            onToggleFavorite={row => controller.toggleFavoriteFor(row.model)}
            onCustomCommit={controller.pickCustom}
            onStartExactEntry={handleStartExactEntry}
          />
          <SelectorFooter
            reasoning={controller.reasoningState}
            value={value.reasoning_effort}
            modelName={modelName || value.provider}
            onSelect={controller.setReasoning}
            speed={speed}
            onSpeedChange={onSpeedChange}
            speedDisabled={inert}
          />
          {/* Polite status: favoriting never moves focus (pointer star or Alt+F
              in search), so speak the result. */}
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
