import { useEffect, useRef } from "react";
import { Settings2 } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  KindIcon,
  Pill,
  Spinner,
  cn,
  providerKindIconRegistry,
} from "@agh/ui";

import {
  AGENT_SETTINGS_SECTIONS,
  AgentSettingsPanels,
  resolveAgentSettingsSearch,
  useAgentSettingsPage,
  type AgentSettingsSearch,
  type AgentSettingsSection,
} from "@/systems/agent";

const SECTION_LABELS: Record<AgentSettingsSection, string> = {
  basics: "Basics",
  runtime: "Runtime",
  instructions: "Instructions",
  access: "Access",
  mcp: "MCP servers",
  danger: "Danger zone",
};

const SETTINGS_MODAL_CLASS =
  "text-fg flex w-(--width-modal-lg) max-w-[calc(100vw-2rem)] flex-col overflow-hidden sm:max-w-(--width-modal-lg) h-[min(720px,92vh)] max-h-[calc(100vh-2rem)]";

export function AgentSettingsLocation({
  name,
  rawSearch,
}: {
  name: string;
  rawSearch: AgentSettingsSearch;
}) {
  const search = resolveAgentSettingsSearch(rawSearch);
  const page = useAgentSettingsPage({ name, section: search.section });
  const returnFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const previous = document.activeElement;
    if (previous instanceof HTMLElement) {
      returnFocusRef.current = previous;
    }
    return () => {
      returnFocusRef.current?.focus?.();
    };
  }, []);

  // Missing agent: keep detail's not-found; do not paint an empty modal over it.
  if (!page.agentLoading && (page.agentError || !page.agent || !page.draft)) {
    return null;
  }

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      page.onBackToDetail();
    }
  };

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        className={SETTINGS_MODAL_CLASS}
        data-testid="agent-settings-dialog"
        aria-describedby="agent-settings-description"
      >
        {page.unsavedGuardDialog}
        {page.deleteFlow.confirmDialog}

        <DialogHeader variant="ruled" className="shrink-0 gap-3 sm:flex-row sm:items-start">
          <div className="flex min-w-0 flex-1 items-start gap-3">
            <span
              aria-hidden="true"
              className="grid size-[34px] shrink-0 place-items-center rounded-lg bg-elevated text-muted shadow-highlight"
            >
              {page.agent ? (
                <KindIcon
                  className="size-[18px]"
                  kind={page.agent.provider}
                  registry={providerKindIconRegistry}
                  size="sm"
                  tone="default"
                />
              ) : (
                <Settings2 className="size-4" />
              )}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <Eyebrow className="text-muted">Agents · Settings</Eyebrow>
                {page.dirty ? (
                  <Pill tone="warning" size="sm" data-testid="agent-settings-unsaved">
                    Unsaved
                  </Pill>
                ) : null}
              </div>
              <DialogTitle className="truncate text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">
                Edit {name}
              </DialogTitle>
              <DialogDescription
                id="agent-settings-description"
                className="mt-1 text-small-body text-muted"
              >
                Update the agent definition used for new sessions. Runtime, instructions, and access
                apply on save — existing sessions keep their current config.
              </DialogDescription>
            </div>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Close settings"
            onClick={() => page.onBackToDetail()}
            data-testid="agent-settings-close"
          >
            <span aria-hidden="true" className="text-body leading-none">
              ×
            </span>
          </Button>
        </DialogHeader>

        {page.agentLoading || !page.agent || !page.draft ? (
          <div
            className="flex flex-1 items-center justify-center gap-2 px-6 py-10 text-muted"
            data-testid="agent-settings-loading"
          >
            <Spinner aria-hidden="true" />
            Loading settings…
          </div>
        ) : (
          <div
            className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden md:flex-row"
            data-testid="agent-settings-page"
          >
            <nav
              aria-label="Agent settings sections"
              className="flex w-full shrink-0 flex-wrap gap-1 border-b border-line px-3 py-2 md:w-settings-nav md:flex-col md:flex-nowrap md:border-r md:border-b-0 md:py-3"
              data-testid="agent-settings-section-nav"
            >
              {AGENT_SETTINGS_SECTIONS.map(section => {
                const isActive = search.section === section;
                const isDanger = section === "danger";
                return (
                  <button
                    key={section}
                    type="button"
                    data-testid={`agent-settings-nav-${section}`}
                    data-active={isActive ? "true" : "false"}
                    aria-current={isActive ? "page" : undefined}
                    onClick={() => page.setSection(section)}
                    className={cn(
                      "rounded-md px-3 py-2 text-left text-small-body font-medium tracking-tight transition-colors duration-base ease-out",
                      "w-auto shrink-0 md:w-full",
                      isActive
                        ? "bg-row-selected text-fg-strong"
                        : "text-muted hover:bg-hover hover:text-fg",
                      isDanger && !isActive && "text-danger hover:text-danger",
                      isDanger && isActive && "bg-danger-tint text-danger"
                    )}
                  >
                    <span className="whitespace-nowrap md:truncate">{SECTION_LABELS[section]}</span>
                  </button>
                );
              })}
            </nav>
            <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
              <AgentSettingsPanels
                section={search.section}
                agent={page.agent}
                draft={page.draft}
                validation={page.validation}
                disabled={page.fieldsDisabled}
                readOnly={page.fieldsReadOnly}
                mutationDenied={page.mutationDenied}
                saveError={page.saveError}
                conflictBanner={page.conflictBanner}
                onReloadAndRetry={() => {
                  void page.onReloadAndRetry();
                }}
                onPatch={page.patchDraft}
                onDelete={page.deleteFlow.openDialog}
                isDeleting={page.deleteFlow.isDeleting}
                providerOptions={page.providerOptions}
                providersLoading={page.providersLoading}
                runtimeModels={page.runtimeModels}
                modelCatalogLoading={page.modelCatalogLoading}
                modelCatalogLoaded={page.modelCatalogLoaded}
                modelCatalogRefreshing={page.modelCatalogRefreshing}
                modelCatalogError={page.modelCatalogError}
                onRefreshCatalog={page.onRefreshCatalog}
                onOpenProviderSettings={page.onOpenProviderSettings}
              />
            </div>
          </div>
        )}

        <DialogFooter variant="ruled" className="shrink-0 sm:justify-between">
          <p className="text-small-body text-muted" data-testid="agent-settings-footer-note">
            Changes apply to new sessions only.
          </p>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => page.onBackToDetail()}
              data-testid="agent-settings-cancel"
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="default"
              size="sm"
              disabled={!page.dirty || page.saveBlocked || page.isSaving || page.agentLoading}
              aria-busy={page.isSaving}
              title={page.saveBlockedCaption}
              onClick={page.onSave}
              data-testid="agent-settings-save"
            >
              {page.isSaving ? <Spinner aria-hidden="true" /> : null}
              Save changes
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
