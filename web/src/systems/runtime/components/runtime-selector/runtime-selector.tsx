import { useRef, type ReactNode } from "react";

import { type RuntimeSpeed } from "@/lib/api-contract";
import { Popover, PopoverContent } from "@compozy/ui";

import { ModelList } from "./model-list";
import { ProviderChips } from "./provider-chips";
import { RuntimeAdvancedOptions } from "./runtime-advanced-options";
import { SelectorFooter } from "./selector-footer";
import { SelectorSearch } from "./selector-search";
import { RuntimeSelectorTrigger } from "./trigger";
import { useRuntimeSelector } from "./use-runtime-selector";
import { useRuntimeSelectorPopup } from "./use-runtime-selector-popup";
import type {
  RuntimeModelOption,
  RuntimeACPOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
  RuntimeSelectorVariant,
} from "./types";

export interface RuntimeSelectorProps {
  value: RuntimeSelectorValue;
  onChange: (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) => void;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  /** Live ACP descriptors for advanced select/boolean controls. */
  acpOptions?: RuntimeACPOption[];
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
  /** Allows an exact `provider/model` value when the provider catalog has no matching row. */
  allowCustomProvider?: boolean;
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
  acpOptions,
  variant = "default",
  disabled = false,
  readOnly = false,
  loading = false,
  modelPlaceholder = "Select model",
  onRefreshCatalog,
  refreshing = false,
  catalogStatus,
  allowCustomProvider = false,
  onOpenProviderSettings,
  speed,
  onSpeedChange,
  triggerId,
  triggerTestId,
  ariaLabelledby,
  className,
}: RuntimeSelectorProps) {
  const controller = useRuntimeSelector({
    value,
    onChange,
    providers,
    models,
    acpOptions,
    allowCustomProvider,
    speed,
  });
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
  const handleCancelExactEntry = () => {
    controller.cancelExactEntry();
    searchRef.current?.focus();
  };
  const handleCommitCustom = (modelId: string) => {
    const restoreSearchFocus = controller.entryMode === "exact";
    if (controller.commitCustom(modelId) && restoreSearchFocus) searchRef.current?.focus();
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
        value={controller.value}
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
        aria-label="Runtime selector"
        className="max-h-[min(440px,var(--available-height))] w-[min(320px,94vw)] overflow-hidden bg-canvas p-0 shadow-overlay"
      >
        <div
          id={popup.popupId}
          className="flex max-h-[inherit] flex-col"
          data-testid="runtime-selector-popup"
        >
          <SelectorSearch
            exactEntry={exactEntry}
            allowCustomProvider={allowCustomProvider}
            searchRef={searchRef}
            exactInputId={exactInputId}
            listId={popup.listId}
            activeDescendant={popup.activeDescendant}
            query={controller.query}
            onQueryChange={controller.changeQuery}
            onKeyDown={popup.handleSearchKeyDown}
            onCancelExactEntry={handleCancelExactEntry}
            onRefreshCatalog={onRefreshCatalog}
            refreshing={refreshing}
          />
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
            onCustomCommit={handleCommitCustom}
            onStartExactEntry={handleStartExactEntry}
          />
          <RuntimeAdvancedOptions
            disabled={inert}
            expanded={controller.advancedExpanded}
            onChange={controller.setACPOption}
            onExpandedChange={controller.setAdvancedExpanded}
            options={controller.advancedOptions}
            providerManaged={controller.providerManaged}
            selections={controller.value.acp_options ?? []}
          />
          <SelectorFooter
            providerManaged={controller.providerManaged}
            reasoning={controller.reasoningState}
            reasoningDisabled={inert || controller.providerManaged}
            value={controller.value.reasoning_effort}
            modelName={modelName || controller.value.provider}
            onSelect={controller.setReasoning}
            speed={speed}
            onSpeedChange={onSpeedChange}
            speedDisabled={inert || controller.providerManaged || !controller.speedSupported}
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
